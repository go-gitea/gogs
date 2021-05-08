// Package client provides an IMAP client.
//
// It is not safe to use the same Client from multiple goroutines. In general,
// the IMAP protocol doesn't make it possible to send multiple independent
// IMAP commands on the same connection.
package client

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/commands"
	"github.com/emersion/go-imap/responses"
)

// errClosed is used when a connection is closed while waiting for a command
// response.
var errClosed = fmt.Errorf("imap: connection closed")

// errUnregisterHandler is returned by a response handler to unregister itself.
var errUnregisterHandler = fmt.Errorf("imap: unregister handler")

// Update is an unilateral server update.
type Update interface {
	update()
}

// StatusUpdate is delivered when a status update is received.
type StatusUpdate struct {
	Status *imap.StatusResp
}

func (u *StatusUpdate) update() {}

// MailboxUpdate is delivered when a mailbox status changes.
type MailboxUpdate struct {
	Mailbox *imap.MailboxStatus
}

func (u *MailboxUpdate) update() {}

// ExpungeUpdate is delivered when a message is deleted.
type ExpungeUpdate struct {
	SeqNum uint32
}

func (u *ExpungeUpdate) update() {}

// MessageUpdate is delivered when a message attribute changes.
type MessageUpdate struct {
	Message *imap.Message
}

func (u *MessageUpdate) update() {}

// Client is an IMAP client.
type Client struct {
	conn       *imap.Conn
	isTLS      bool
	serverName string

	loggedOut chan struct{}
	continues chan<- bool
	upgrading bool

	handlers       []responses.Handler
	handlersLocker sync.Mutex

	// The current connection state.
	state imap.ConnState
	// The selected mailbox, if there is one.
	mailbox *imap.MailboxStatus
	// The cached server capabilities.
	caps map[string]bool
	// state, mailbox and caps may be accessed in different goroutines. Protect
	// access.
	locker sync.Mutex

	// A channel to which unilateral updates from the server will be sent. An
	// update can be one of: *StatusUpdate, *MailboxUpdate, *MessageUpdate,
	// *ExpungeUpdate. Note that blocking this channel blocks the whole client,
	// so it's recommended to use a separate goroutine and a buffered channel to
	// prevent deadlocks.
	Updates chan<- Update

	// ErrorLog specifies an optional logger for errors accepting connections and
	// unexpected behavior from handlers. By default, logging goes to os.Stderr
	// via the log package's standard logger. The logger must be safe to use
	// simultaneously from multiple goroutines.
	ErrorLog imap.Logger

	// Timeout specifies a maximum amount of time to wait on a command.
	//
	// A Timeout of zero means no timeout. This is the default.
	Timeout time.Duration
}

func (c *Client) registerHandler(h responses.Handler) {
	if h == nil {
		return
	}

	c.handlersLocker.Lock()
	c.handlers = append(c.handlers, h)
	c.handlersLocker.Unlock()
}

func (c *Client) handle(resp imap.Resp) error {
	c.handlersLocker.Lock()
	for i := len(c.handlers) - 1; i >= 0; i-- {
		if err := c.handlers[i].Handle(resp); err != responses.ErrUnhandled {
			if err == errUnregisterHandler {
				c.handlers = append(c.handlers[:i], c.handlers[i+1:]...)
				err = nil
			}
			c.handlersLocker.Unlock()
			return err
		}
	}
	c.handlersLocker.Unlock()
	return responses.ErrUnhandled
}

func (c *Client) reader() {
	defer close(c.loggedOut)
	// Loop while connected.
	for {
		connected, err := c.readOnce()
		if err != nil {
			c.ErrorLog.Println("error reading response:", err)
		}
		if !connected {
			return
		}
	}
}

func (c *Client) readOnce() (bool, error) {
	if c.State() == imap.LogoutState {
		return false, nil
	}

	resp, err := imap.ReadResp(c.conn.Reader)
	if err == io.EOF || c.State() == imap.LogoutState {
		return false, nil
	} else if err != nil {
		if imap.IsParseError(err) {
			return true, err
		} else {
			return false, err
		}
	}

	if err := c.handle(resp); err != nil && err != responses.ErrUnhandled {
		c.ErrorLog.Println("cannot handle response ", resp, err)
	}
	return true, nil
}

func (c *Client) writeReply(reply []byte) error {
	if _, err := c.conn.Writer.Write(reply); err != nil {
		return err
	}
	// Flush reply
	return c.conn.Writer.Flush()
}

type handleResult struct {
	status *imap.StatusResp
	err    error
}

func (c *Client) execute(cmdr imap.Commander, h responses.Handler) (*imap.StatusResp, error) {
	cmd := cmdr.Command()
	cmd.Tag = generateTag()

	var replies <-chan []byte
	if replier, ok := h.(responses.Replier); ok {
		replies = replier.Replies()
	}

	if c.Timeout > 0 {
		err := c.conn.SetDeadline(time.Now().Add(c.Timeout))
		if err != nil {
			return nil, err
		}
	} else {
		// It's possible the client had a timeout set from a previous command, but no
		// longer does. Ensure we respect that. The zero time means no deadline.
		if err := c.conn.SetDeadline(time.Time{}); err != nil {
			return nil, err
		}
	}

	// Check if we are upgrading.
	upgrading := c.upgrading

	// Add handler before sending command, to be sure to get the response in time
	// (in tests, the response is sent right after our command is received, so
	// sometimes the response was received before the setup of this handler)
	doneHandle := make(chan handleResult, 1)
	unregister := make(chan struct{})
	c.registerHandler(responses.HandlerFunc(func(resp imap.Resp) error {
		select {
		case <-unregister:
			// If an error occured while sending the command, abort
			return errUnregisterHandler
		default:
		}

		if s, ok := resp.(*imap.StatusResp); ok && s.Tag == cmd.Tag {
			// This is the command's status response, we're done
			doneHandle <- handleResult{s, nil}
			// Special handling of connection upgrading.
			if upgrading {
				c.upgrading = false
				// Wait for upgrade to finish.
				c.conn.Wait()
			}
			// Cancel any pending literal write
			select {
			case c.continues <- false:
			default:
			}
			return errUnregisterHandler
		}

		if h != nil {
			// Pass the response to the response handler
			if err := h.Handle(resp); err != nil && err != responses.ErrUnhandled {
				// If the response handler returns an error, abort
				doneHandle <- handleResult{nil, err}
				return errUnregisterHandler
			} else {
				return err
			}
		}
		return responses.ErrUnhandled
	}))

	// Send the command to the server
	if err := cmd.WriteTo(c.conn.Writer); err != nil {
		// Error while sending the command
		close(unregister)

		if err, ok := err.(imap.LiteralLengthErr); ok {
			// Expected > Actual
			//  The server is waiting for us to write
			//  more bytes, we don't have them. Run.
			// Expected < Actual
			//  We are about to send a potentially truncated message, we don't
			//  want this (ths terminating CRLF is not sent at this point).
			c.conn.Close()
			return nil, err
		}

		return nil, err
	}
	// Flush writer if we are upgrading
	if upgrading {
		if err := c.conn.Writer.Flush(); err != nil {
			// Error while sending the command
			close(unregister)
			return nil, err
		}
	}

	for {
		select {
		case reply := <-replies:
			// Response handler needs to send a reply (Used for AUTHENTICATE)
			if err := c.writeReply(reply); err != nil {
				close(unregister)
				return nil, err
			}
		case <-c.loggedOut:
			// If the connection is closed (such as from an I/O error), ensure we
			// realize this and don't block waiting on a response that will never
			// come. loggedOut is a channel that closes when the reader goroutine
			// ends.
			close(unregister)
			return nil, errClosed
		case result := <-doneHandle:
			return result.status, result.err
		}
	}
}

// State returns the current connection state.
func (c *Client) State() imap.ConnState {
	c.locker.Lock()
	state := c.state
	c.locker.Unlock()
	return state
}

// Mailbox returns the selected mailbox. It returns nil if there isn't one.
func (c *Client) Mailbox() *imap.MailboxStatus {
	// c.Mailbox fields are not supposed to change, so we can return the pointer.
	c.locker.Lock()
	mbox := c.mailbox
	c.locker.Unlock()
	return mbox
}

// SetState sets this connection's internal state.
//
// This function should not be called directly, it must only be used by
// libraries implementing extensions of the IMAP protocol.
func (c *Client) SetState(state imap.ConnState, mailbox *imap.MailboxStatus) {
	c.locker.Lock()
	c.state = state
	c.mailbox = mailbox
	c.locker.Unlock()
}

// Execute executes a generic command. cmdr is a value that can be converted to
// a raw command and h is a response handler. The function returns when the
// command has completed or failed, in this case err is nil. A non-nil err value
// indicates a network error.
//
// This function should not be called directly, it must only be used by
// libraries implementing extensions of the IMAP protocol.
func (c *Client) Execute(cmdr imap.Commander, h responses.Handler) (*imap.StatusResp, error) {
	return c.execute(cmdr, h)
}

func (c *Client) handleContinuationReqs() {
	c.registerHandler(responses.HandlerFunc(func(resp imap.Resp) error {
		if _, ok := resp.(*imap.ContinuationReq); ok {
			go func() {
				c.continues <- true
			}()
			return nil
		}
		return responses.ErrUnhandled
	}))
}

func (c *Client) gotStatusCaps(args []interface{}) {
	c.locker.Lock()

	c.caps = make(map[string]bool)
	for _, cap := range args {
		if cap, ok := cap.(string); ok {
			c.caps[cap] = true
		}
	}

	c.locker.Unlock()
}

// The server can send unilateral data. This function handles it.
func (c *Client) handleUnilateral() {
	c.registerHandler(responses.HandlerFunc(func(resp imap.Resp) error {
		switch resp := resp.(type) {
		case *imap.StatusResp:
			if resp.Tag != "*" {
				return responses.ErrUnhandled
			}

			switch resp.Type {
			case imap.StatusRespOk, imap.StatusRespNo, imap.StatusRespBad:
				if c.Updates != nil {
					c.Updates <- &StatusUpdate{resp}
				}
			case imap.StatusRespBye:
				c.locker.Lock()
				c.state = imap.LogoutState
				c.mailbox = nil
				c.locker.Unlock()

				c.conn.Close()

				if c.Updates != nil {
					c.Updates <- &StatusUpdate{resp}
				}
			default:
				return responses.ErrUnhandled
			}
		case *imap.DataResp:
			name, fields, ok := imap.ParseNamedResp(resp)
			if !ok {
				return responses.ErrUnhandled
			}

			switch name {
			case "CAPABILITY":
				c.gotStatusCaps(fields)
			case "EXISTS":
				if c.Mailbox() == nil {
					break
				}

				if messages, err := imap.ParseNumber(fields[0]); err == nil {
					c.locker.Lock()
					c.mailbox.Messages = messages
					c.locker.Unlock()

					c.mailbox.ItemsLocker.Lock()
					c.mailbox.Items[imap.StatusMessages] = nil
					c.mailbox.ItemsLocker.Unlock()
				}

				if c.Updates != nil {
					c.Updates <- &MailboxUpdate{c.Mailbox()}
				}
			case "RECENT":
				if c.Mailbox() == nil {
					break
				}

				if recent, err := imap.ParseNumber(fields[0]); err == nil {
					c.locker.Lock()
					c.mailbox.Recent = recent
					c.locker.Unlock()

					c.mailbox.ItemsLocker.Lock()
					c.mailbox.Items[imap.StatusRecent] = nil
					c.mailbox.ItemsLocker.Unlock()
				}

				if c.Updates != nil {
					c.Updates <- &MailboxUpdate{c.Mailbox()}
				}
			case "EXPUNGE":
				seqNum, _ := imap.ParseNumber(fields[0])

				if c.Updates != nil {
					c.Updates <- &ExpungeUpdate{seqNum}
				}
			case "FETCH":
				seqNum, _ := imap.ParseNumber(fields[0])
				fields, _ := fields[1].([]interface{})

				msg := &imap.Message{SeqNum: seqNum}
				if err := msg.Parse(fields); err != nil {
					break
				}

				if c.Updates != nil {
					c.Updates <- &MessageUpdate{msg}
				}
			default:
				return responses.ErrUnhandled
			}
		default:
			return responses.ErrUnhandled
		}
		return nil
	}))
}

func (c *Client) handleGreetAndStartReading() error {
	var greetErr error
	gotGreet := false

	c.registerHandler(responses.HandlerFunc(func(resp imap.Resp) error {
		status, ok := resp.(*imap.StatusResp)
		if !ok {
			greetErr = fmt.Errorf("invalid greeting received from server: not a status response")
			return errUnregisterHandler
		}

		c.locker.Lock()
		switch status.Type {
		case imap.StatusRespPreauth:
			c.state = imap.AuthenticatedState
		case imap.StatusRespBye:
			c.state = imap.LogoutState
		case imap.StatusRespOk:
			c.state = imap.NotAuthenticatedState
		default:
			c.state = imap.LogoutState
			c.locker.Unlock()
			greetErr = fmt.Errorf("invalid greeting received from server: %v", status.Type)
			return errUnregisterHandler
		}
		c.locker.Unlock()

		if status.Code == imap.CodeCapability {
			c.gotStatusCaps(status.Arguments)
		}

		gotGreet = true
		return errUnregisterHandler
	}))

	// call `readOnce` until we get the greeting or an error
	for !gotGreet {
		connected, err := c.readOnce()
		// Check for read errors
		if err != nil {
			// return read errors
			return err
		}
		// Check for invalid greet
		if greetErr != nil {
			// return read errors
			return greetErr
		}
		// Check if connection was closed.
		if !connected {
			// connection closed.
			return io.EOF
		}
	}

	// We got the greeting, now start the reader goroutine.
	go c.reader()

	return nil
}

// Upgrade a connection, e.g. wrap an unencrypted connection with an encrypted
// tunnel.
//
// This function should not be called directly, it must only be used by
// libraries implementing extensions of the IMAP protocol.
func (c *Client) Upgrade(upgrader imap.ConnUpgrader) error {
	return c.conn.Upgrade(upgrader)
}

// Writer returns the imap.Writer for this client's connection.
//
// This function should not be called directly, it must only be used by
// libraries implementing extensions of the IMAP protocol.
func (c *Client) Writer() *imap.Writer {
	return c.conn.Writer
}

// IsTLS checks if this client's connection has TLS enabled.
func (c *Client) IsTLS() bool {
	return c.isTLS
}

// LoggedOut returns a channel which is closed when the connection to the server
// is closed.
func (c *Client) LoggedOut() <-chan struct{} {
	return c.loggedOut
}

// SetDebug defines an io.Writer to which all network activity will be logged.
// If nil is provided, network activity will not be logged.
func (c *Client) SetDebug(w io.Writer) {
	// Need to send a command to unblock the reader goroutine.
	cmd := new(commands.Noop)
	err := c.Upgrade(func(conn net.Conn) (net.Conn, error) {
		// Flag connection as in upgrading
		c.upgrading = true
		if status, err := c.execute(cmd, nil); err != nil {
			return nil, err
		} else if err := status.Err(); err != nil {
			return nil, err
		}

		// Wait for reader to block.
		c.conn.WaitReady()

		c.conn.SetDebug(w)
		return conn, nil
	})
	if err != nil {
		log.Println("SetDebug:", err)
	}

}

// New creates a new client from an existing connection.
func New(conn net.Conn) (*Client, error) {
	continues := make(chan bool)
	w := imap.NewClientWriter(nil, continues)
	r := imap.NewReader(nil)

	c := &Client{
		conn:      imap.NewConn(conn, r, w),
		loggedOut: make(chan struct{}),
		continues: continues,
		state:     imap.ConnectingState,
		ErrorLog:  log.New(os.Stderr, "imap/client: ", log.LstdFlags),
	}

	c.handleContinuationReqs()
	c.handleUnilateral()
	if err := c.handleGreetAndStartReading(); err != nil {
		return c, err
	}

	plusOk, _ := c.Support("LITERAL+")
	minusOk, _ := c.Support("LITERAL-")
	// We don't use non-sync literal if it is bigger than 4096 bytes, so
	// LITERAL- is fine too.
	c.conn.AllowAsyncLiterals = plusOk || minusOk

	return c, nil
}

// Dial connects to an IMAP server using an unencrypted connection.
func Dial(addr string) (*Client, error) {
	return DialWithDialer(new(net.Dialer), addr)
}

type Dialer interface {
	// Dial connects to the given address.
	Dial(network, addr string) (net.Conn, error)
}

// DialWithDialer connects to an IMAP server using an unencrypted connection
// using dialer.Dial.
//
// Among other uses, this allows to apply a dial timeout.
func DialWithDialer(dialer Dialer, addr string) (*Client, error) {
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	// We don't return to the caller until we try to receive a greeting. As such,
	// there is no way to set the client's Timeout for that action. As a
	// workaround, if the dialer has a timeout set, use that for the connection's
	// deadline.
	if netDialer, ok := dialer.(*net.Dialer); ok && netDialer.Timeout > 0 {
		err := conn.SetDeadline(time.Now().Add(netDialer.Timeout))
		if err != nil {
			return nil, err
		}
	}

	c, err := New(conn)
	if err != nil {
		return nil, err
	}

	c.serverName, _, _ = net.SplitHostPort(addr)
	return c, nil
}

// DialTLS connects to an IMAP server using an encrypted connection.
func DialTLS(addr string, tlsConfig *tls.Config) (*Client, error) {
	return DialWithDialerTLS(new(net.Dialer), addr, tlsConfig)
}

// DialWithDialerTLS connects to an IMAP server using an encrypted connection
// using dialer.Dial.
//
// Among other uses, this allows to apply a dial timeout.
func DialWithDialerTLS(dialer Dialer, addr string, tlsConfig *tls.Config) (*Client, error) {
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	serverName, _, _ := net.SplitHostPort(addr)
	if tlsConfig == nil {
		tlsConfig = &tls.Config{}
	}
	if tlsConfig.ServerName == "" {
		tlsConfig = tlsConfig.Clone()
		tlsConfig.ServerName = serverName
	}
	tlsConn := tls.Client(conn, tlsConfig)

	// We don't return to the caller until we try to receive a greeting. As such,
	// there is no way to set the client's Timeout for that action. As a
	// workaround, if the dialer has a timeout set, use that for the connection's
	// deadline.
	if netDialer, ok := dialer.(*net.Dialer); ok && netDialer.Timeout > 0 {
		err := tlsConn.SetDeadline(time.Now().Add(netDialer.Timeout))
		if err != nil {
			return nil, err
		}
	}

	c, err := New(tlsConn)
	if err != nil {
		return nil, err
	}

	c.isTLS = true
	c.serverName = serverName
	return c, nil
}
