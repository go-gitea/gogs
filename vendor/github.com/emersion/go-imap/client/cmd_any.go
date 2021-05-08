package client

import (
	"errors"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/commands"
)

// ErrAlreadyLoggedOut is returned if Logout is called when the client is
// already logged out.
var ErrAlreadyLoggedOut = errors.New("Already logged out")

// Capability requests a listing of capabilities that the server supports.
// Capabilities are often returned by the server with the greeting or with the
// STARTTLS and LOGIN responses, so usually explicitly requesting capabilities
// isn't needed.
//
// Most of the time, Support should be used instead.
func (c *Client) Capability() (map[string]bool, error) {
	cmd := &commands.Capability{}

	if status, err := c.execute(cmd, nil); err != nil {
		return nil, err
	} else if err := status.Err(); err != nil {
		return nil, err
	}

	c.locker.Lock()
	caps := c.caps
	c.locker.Unlock()
	return caps, nil
}

// Support checks if cap is a capability supported by the server. If the server
// hasn't sent its capabilities yet, Support requests them.
func (c *Client) Support(cap string) (bool, error) {
	c.locker.Lock()
	ok := c.caps != nil
	c.locker.Unlock()

	// If capabilities are not cached, request them
	if !ok {
		if _, err := c.Capability(); err != nil {
			return false, err
		}
	}

	c.locker.Lock()
	supported := c.caps[cap]
	c.locker.Unlock()
	return supported, nil
}

// Noop always succeeds and does nothing.
//
// It can be used as a periodic poll for new messages or message status updates
// during a period of inactivity. It can also be used to reset any inactivity
// autologout timer on the server.
func (c *Client) Noop() error {
	cmd := new(commands.Noop)

	status, err := c.execute(cmd, nil)
	if err != nil {
		return err
	}
	return status.Err()
}

// Logout gracefully closes the connection.
func (c *Client) Logout() error {
	if c.State() == imap.LogoutState {
		return ErrAlreadyLoggedOut
	}

	cmd := new(commands.Logout)

	if status, err := c.execute(cmd, nil); err == errClosed {
		// Server closed connection, that's what we want anyway
		return nil
	} else if err != nil {
		return err
	} else if status != nil {
		return status.Err()
	}
	return nil
}
