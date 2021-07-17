// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package haproxy

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/log"
)

var (
	// v1Prefix is the string we look for at the start of a connection
	// to check if this connection is using the proxy protocol
	v1Prefix    = []byte("PROXY ")
	v1PrefixLen = len(v1Prefix)
	v2Prefix    = []byte("\x0D\x0A\x0D\x0A\x00\x0D\x0A\x51\x55\x49\x54\x0A")
	v2PrefixLen = len(v2Prefix)
)

// Conn is used to wrap and underlying connection which is speaking the
// Proxy Protocol. RemoteAddr() will return the address of the client
// instead of the proxy address.
type Conn struct {
	bufReader          *bufio.Reader
	conn               net.Conn
	localAddr          net.Addr
	remoteAddr         net.Addr
	once               sync.Once
	proxyHeaderTimeout time.Duration
	acceptUnknown      bool
}

// NewConn is used to wrap a net.Conn speaking the proxy protocol into
// a haproxy.Conn
func NewConn(conn net.Conn, timeout time.Duration) *Conn {
	pConn := &Conn{
		bufReader:          bufio.NewReader(conn),
		conn:               conn,
		proxyHeaderTimeout: timeout,
	}
	return pConn
}

// Read reads data from the connection.
// It will initially read the proxy protocol header.
// If there is an error parsing the header, it is returned and the socket is closed.
func (p *Conn) Read(b []byte) (int, error) {
	if err := p.readProxyHeaderOnce(); err != nil {
		return 0, err
	}
	return p.bufReader.Read(b)
}

// ReadFrom reads data from a provided reader and copies it to the connection.
func (p *Conn) ReadFrom(r io.Reader) (int64, error) {
	if err := p.readProxyHeaderOnce(); err != nil {
		return 0, err
	}
	if rf, ok := p.conn.(io.ReaderFrom); ok {
		return rf.ReadFrom(r)
	}
	return io.Copy(p.conn, r)
}

// WriteTo reads data from the connection and writes it to the writer.
// It will initially read the proxy protocol header.
// If there is an error parsing the header, it is returned and the socket is closed.
func (p *Conn) WriteTo(w io.Writer) (int64, error) {
	if err := p.readProxyHeaderOnce(); err != nil {
		return 0, err
	}
	return p.bufReader.WriteTo(w)
}

// Write writes data to the connection.
// Write can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetWriteDeadline.
func (p *Conn) Write(b []byte) (int, error) {
	if err := p.readProxyHeaderOnce(); err != nil {
		return 0, err
	}
	return p.conn.Write(b)
}

// Close closes the connection.
// Any blocked Read or Write operations will be unblocked and return errors.
func (p *Conn) Close() error {
	return p.conn.Close()
}

// LocalAddr returns the local network address.
func (p *Conn) LocalAddr() net.Addr {
	_ = p.readProxyHeaderOnce()
	if p.localAddr != nil {
		return p.localAddr
	}
	return p.conn.LocalAddr()
}

// RemoteAddr returns the address of the client if the proxy
// protocol is being used, otherwise just returns the address of
// the socket peer. If there is an error parsing the header, the
// address of the client is not returned, and the socket is closed.
// One implication of this is that the call could block if the
// client is slow. Using a Deadline is recommended if this is called
// before Read()
func (p *Conn) RemoteAddr() net.Addr {
	_ = p.readProxyHeaderOnce()
	if p.remoteAddr != nil {
		return p.remoteAddr
	}
	return p.conn.RemoteAddr()
}

// SetDeadline sets the read and write deadlines associated
// with the connection. It is equivalent to calling both
// SetReadDeadline and SetWriteDeadline.
//
// A deadline is an absolute time after which I/O operations
// fail instead of blocking. The deadline applies to all future
// and pending I/O, not just the immediately following call to
// Read or Write. After a deadline has been exceeded, the
// connection can be refreshed by setting a deadline in the future.
//
// If the deadline is exceeded a call to Read or Write or to other
// I/O methods will return an error that wraps os.ErrDeadlineExceeded.
// This can be tested using errors.Is(err, os.ErrDeadlineExceeded).
// The error's Timeout method will return true, but note that there
// are other possible errors for which the Timeout method will
// return true even if the deadline has not been exceeded.
//
// An idle timeout can be implemented by repeatedly extending
// the deadline after successful Read or Write calls.
//
// A zero value for t means I/O operations will not time out.
func (p *Conn) SetDeadline(t time.Time) error {
	return p.conn.SetDeadline(t)
}

// SetReadDeadline sets the deadline for future Read calls
// and any currently-blocked Read call.
// A zero value for t means Read will not time out.
func (p *Conn) SetReadDeadline(t time.Time) error {
	return p.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the deadline for future Write calls
// and any currently-blocked Write call.
// Even if write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means Write will not time out.
func (p *Conn) SetWriteDeadline(t time.Time) error {
	return p.conn.SetWriteDeadline(t)
}

// readProxyHeaderOnce will ensure that the proxy header has been read
func (p *Conn) readProxyHeaderOnce() (err error) {
	p.once.Do(func() {
		if err = p.readProxyHeader(); err != nil && err != io.EOF {
			log.Error("Failed to read proxy prefix: %v", err)
			p.Close()
			p.bufReader = bufio.NewReader(p.conn)
		}
	})
	return
}

func (p *Conn) readProxyHeader() error {
	if p.proxyHeaderTimeout != 0 {
		readDeadLine := time.Now().Add(p.proxyHeaderTimeout)
		_ = p.conn.SetReadDeadline(readDeadLine)
		defer func() {
			_ = p.conn.SetReadDeadline(time.Time{})
		}()
	}

	inp, err := p.bufReader.Peek(v1PrefixLen)
	if err != nil {
		return err
	}

	if bytes.Equal(inp, v1Prefix) {
		return p.readV1ProxyHeader()
	}

	inp, err = p.bufReader.Peek(v2PrefixLen)
	if err != nil {
		return err
	}
	if bytes.Equal(inp, v2Prefix) {
		return p.readV2ProxyHeader()
	}

	return &ErrBadHeader{inp}
}

func (p *Conn) readV2ProxyHeader() error {
	// The binary header format starts with a constant 12 bytes block containing the
	// protocol signature :
	//
	//    \x0D \x0A \x0D \x0A \x00 \x0D \x0A \x51 \x55 \x49 \x54 \x0A
	//
	// Note that this block contains a null byte at the 5th position, so it must not
	// be handled as a null-terminated string.

	if _, err := p.bufReader.Discard(v2PrefixLen); err != nil {
		// This shouldn't happen as we have already asserted that there should be enough in the buffer
		return err
	}

	// The next byte (the 13th one) is the protocol version and command.
	version, err := p.bufReader.ReadByte()
	if err != nil {
		return err
	}

	// The 14th byte contains the transport protocol and address family.otocol.
	familyByte, err := p.bufReader.ReadByte()
	if err != nil {
		return err
	}

	// The 15th and 16th bytes is the address length in bytes in network endian order.
	var addressLen uint16
	if err := binary.Read(p.bufReader, binary.BigEndian, &addressLen); err != nil {
		return err
	}

	// Now handle the version byte: (14th byte).
	// The highest four bits contains the version. As of this specification, it must
	// always be sent as \x2 and the receiver must only accept this value.
	if version>>4 != 0x2 {
		return &ErrBadHeader{append(v2Prefix, version, familyByte, uint8(addressLen>>8), uint8(addressLen&0xff))}
	}

	// The lowest four bits represents the command :
	switch version & 0xf {
	case 0x0:
		// - \x0 : LOCAL : the connection was established on purpose by the proxy
		//   without being relayed. The connection endpoints are the sender and the
		//   receiver. Such connections exist when the proxy sends health-checks to the
		//   server. The receiver must accept this connection as valid and must use the
		//   real connection endpoints and discard the protocol block including the
		//   family which is ignored.

		// We therefore ignore the 14th, 15th and 16th bytes
		p.remoteAddr = p.conn.LocalAddr()
		p.localAddr = p.conn.RemoteAddr()
		return nil
	case 0x1:
	// - \x1 : PROXY : the connection was established on behalf of another node,
	//   and reflects the original connection endpoints. The receiver must then use
	//   the information provided in the protocol block to get original the address.
	default:
		// - other values are unassigned and must not be emitted by senders. Receivers
		//   must drop connections presenting unexpected values here.
		return &ErrBadHeader{append(v2Prefix, version, familyByte, uint8(addressLen>>8), uint8(addressLen&0xff))}
	}

	// Now handle the familyByte byte: (15th byte).
	// The highest 4 bits contain the address family, the lowest 4 bits contain the protocol

	// 	The address family maps to the original socket family without necessarily
	// matching the values internally used by the system. It may be one of :
	//
	//   - 0x0 : AF_UNSPEC : the connection is forwarded for an unknown, unspecified
	//     or unsupported protocol. The sender should use this family when sending
	//     LOCAL commands or when dealing with unsupported protocol families. The
	//     receiver is free to accept the connection anyway and use the real endpoint
	//     addresses or to reject it. The receiver should ignore address information.
	//
	//   - 0x1 : AF_INET : the forwarded connection uses the AF_INET address family
	//     (IPv4). The addresses are exactly 4 bytes each in network byte order,
	//     followed by transport protocol information (typically ports).
	//
	//   - 0x2 : AF_INET6 : the forwarded connection uses the AF_INET6 address family
	//     (IPv6). The addresses are exactly 16 bytes each in network byte order,
	//     followed by transport protocol information (typically ports).
	//
	//   - 0x3 : AF_UNIX : the forwarded connection uses the AF_UNIX address family
	//     (UNIX). The addresses are exactly 108 bytes each.
	//
	//   - other values are unspecified and must not be emitted in version 2 of this
	//     protocol and must be rejected as invalid by receivers.

	// 	The transport protocol is specified in the lowest 4 bits of the 14th byte :
	//
	//   - 0x0 : UNSPEC : the connection is forwarded for an unknown, unspecified
	//     or unsupported protocol. The sender should use this family when sending
	//     LOCAL commands or when dealing with unsupported protocol families. The
	//     receiver is free to accept the connection anyway and use the real endpoint
	//     addresses or to reject it. The receiver should ignore address information.
	//
	//   - 0x1 : STREAM : the forwarded connection uses a SOCK_STREAM protocol (eg:
	//     TCP or UNIX_STREAM). When used with AF_INET/AF_INET6 (TCP), the addresses
	//     are followed by the source and destination ports represented on 2 bytes
	//     each in network byte order.
	//
	//   - 0x2 : DGRAM : the forwarded connection uses a SOCK_DGRAM protocol (eg:
	//     UDP or UNIX_DGRAM). When used with AF_INET/AF_INET6 (UDP), the addresses
	//     are followed by the source and destination ports represented on 2 bytes
	//     each in network byte order.
	//
	//   - other values are unspecified and must not be emitted in version 2 of this
	//     protocol and must be rejected as invalid by receivers.

	if familyByte>>4 == 0x0 || familyByte&0xf == 0x0 {
		//   - hi 0x0 : AF_UNSPEC : the connection is forwarded for an unknown address type
		// or
		//   - lo 0x0 : UNSPEC : the connection is forwarded for an unspecified protocol
		if !p.acceptUnknown {
			p.conn.Close()
			return &ErrBadHeader{append(v2Prefix, version, familyByte, uint8(addressLen>>8), uint8(addressLen&0xff))}
		}
		p.remoteAddr = p.conn.LocalAddr()
		p.localAddr = p.conn.RemoteAddr()
		_, err = p.bufReader.Discard(int(addressLen))
		return err
	}

	// other address or protocol
	if (familyByte>>4) > 0x3 || (familyByte&0xf) > 0x2 {
		return &ErrBadHeader{append(v2Prefix, version, familyByte, uint8(addressLen>>8), uint8(addressLen&0xff))}
	}

	// Handle AF_UNIX addresses
	if familyByte>>4 == 0x3 {
		//   - \x31 : UNIX stream : the forwarded connection uses SOCK_STREAM over the
		//     AF_UNIX protocol family. Address length is 2*108 = 216 bytes.
		//   - \x32 : UNIX datagram : the forwarded connection uses SOCK_DGRAM over the
		//     AF_UNIX protocol family. Address length is 2*108 = 216 bytes.
		if addressLen != 216 {
			return &ErrBadHeader{append(v2Prefix, version, familyByte, uint8(addressLen>>8), uint8(addressLen&0xff))}
		}
		remoteName := make([]byte, 108)
		localName := make([]byte, 108)
		if _, err := p.bufReader.Read(remoteName); err != nil {
			return err
		}
		if _, err := p.bufReader.Read(localName); err != nil {
			return err
		}
		protocol := "unix"
		if familyByte&0xf == 2 {
			protocol = "unixgram"
		}

		p.remoteAddr = &net.UnixAddr{
			Name: string(remoteName),
			Net:  protocol,
		}
		p.localAddr = &net.UnixAddr{
			Name: string(localName),
			Net:  protocol,
		}
		return nil
	}

	var remoteIP []byte
	var localIP []byte
	var remotePort uint16
	var localPort uint16

	if familyByte>>4 == 0x1 {
		// AF_INET
		// 	 - \x11 : TCP over IPv4 : the forwarded connection uses TCP over the AF_INET
		//     protocol family. Address length is 2*4 + 2*2 = 12 bytes.
		//   - \x12 : UDP over IPv4 : the forwarded connection uses UDP over the AF_INET
		//     protocol family. Address length is 2*4 + 2*2 = 12 bytes.
		if addressLen != 12 {
			return &ErrBadHeader{append(v2Prefix, version, familyByte, uint8(addressLen>>8), uint8(addressLen&0xff))}
		}

		remoteIP = make([]byte, 4)
		localIP = make([]byte, 4)
	} else {
		// AF_INET6
		// - \x21 : TCP over IPv6 : the forwarded connection uses TCP over the AF_INET6
		// 	 protocol family. Address length is 2*16 + 2*2 = 36 bytes.
		// - \x22 : UDP over IPv6 : the forwarded connection uses UDP over the AF_INET6
		// 	 protocol family. Address length is 2*16 + 2*2 = 36 bytes.
		if addressLen != 36 {
			return &ErrBadHeader{append(v2Prefix, version, familyByte, uint8(addressLen>>8), uint8(addressLen&0xff))}
		}

		remoteIP = make([]byte, 16)
		localIP = make([]byte, 16)
	}

	if _, err := p.bufReader.Read(remoteIP); err != nil {
		return err
	}
	if _, err := p.bufReader.Read(localIP); err != nil {
		return err
	}
	if err := binary.Read(p.bufReader, binary.BigEndian, &remotePort); err != nil {
		return err
	}
	if err := binary.Read(p.bufReader, binary.BigEndian, &localPort); err != nil {
		return err
	}

	if familyByte&0xf == 1 {
		p.remoteAddr = &net.TCPAddr{
			IP:   remoteIP,
			Port: int(remotePort),
		}
		p.localAddr = &net.TCPAddr{
			IP:   localIP,
			Port: int(localPort),
		}
	} else {
		p.remoteAddr = &net.UDPAddr{
			IP:   remoteIP,
			Port: int(remotePort),
		}
		p.localAddr = &net.UDPAddr{
			IP:   localIP,
			Port: int(localPort),
		}
	}
	return nil
}

func (p *Conn) readV1ProxyHeader() error {
	// Read until a newline
	header, err := p.bufReader.ReadString('\n')
	if err != nil {
		p.conn.Close()
		return err
	}

	if header[len(header)-2] != '\r' {
		return &ErrBadHeader{[]byte(header)}
	}

	// Strip the carriage return and new line
	header = header[:len(header)-2]

	// Split on spaces, should be (PROXY <type> <remote addr> <local addr> <remote port> <local port>)
	parts := strings.Split(header, " ")
	if len(parts) < 2 {
		p.conn.Close()
		return &ErrBadHeader{[]byte(header)}
	}

	// Verify the type is known
	switch parts[1] {
	case "UNKNOWN":
		if !p.acceptUnknown || len(parts) != 2 {
			p.conn.Close()
			return &ErrBadHeader{[]byte(header)}
		}
		p.remoteAddr = p.conn.LocalAddr()
		p.localAddr = p.conn.RemoteAddr()
		return nil
	case "TCP4":
	case "TCP6":
	default:
		p.conn.Close()
		return &ErrBadAddressType{parts[1]}
	}

	if len(parts) != 6 {
		p.conn.Close()
		return &ErrBadHeader{[]byte(header)}
	}

	// Parse out the remote address
	ip := net.ParseIP(parts[2])
	if ip == nil {
		p.conn.Close()
		return &ErrBadRemote{parts[2], parts[4]}
	}
	port, err := strconv.Atoi(parts[4])
	if err != nil {
		p.conn.Close()
		return &ErrBadRemote{parts[2], parts[4]}
	}
	p.remoteAddr = &net.TCPAddr{IP: ip, Port: port}

	// Parse out the destination address
	ip = net.ParseIP(parts[3])
	if ip == nil {
		p.conn.Close()
		return &ErrBadLocal{parts[3], parts[5]}
	}
	port, err = strconv.Atoi(parts[5])
	if err != nil {
		p.conn.Close()
		return &ErrBadLocal{parts[3], parts[5]}
	}
	p.localAddr = &net.TCPAddr{IP: ip, Port: port}

	return nil
}
