package log

import (
	"encoding/binary"
	"net"
	"time"
)

//SocketHandler writes log to a connectionl.
//Network protocol is simple: log length + log | log length + log. log length is uint32, bigendian.
//you must implement your own log server, maybe you can use logd instead simply.
type SocketHandler struct {
	c        net.Conn
	protocol string
	addr     string
}

func NewSocketHandler(protocol string, addr string) (*SocketHandler, error) {
	s := new(SocketHandler)

	s.protocol = protocol
	s.addr = addr

	return s, nil
}

func (h *SocketHandler) Write(p []byte) (n int, err error) {
	if err = h.connect(); err != nil {
		return
	}

	buf := make([]byte, len(p)+4)

	binary.BigEndian.PutUint32(buf, uint32(len(p)))

	copy(buf[4:], p)

	n, err = h.c.Write(buf)
	if err != nil {
		h.c.Close()
		h.c = nil
	}
	return
}

func (h *SocketHandler) Close() error {
	if h.c != nil {
		h.c.Close()
	}
	return nil
}

func (h *SocketHandler) connect() error {
	if h.c != nil {
		return nil
	}

	var err error
	h.c, err = net.DialTimeout(h.protocol, h.addr, 20*time.Second)
	if err != nil {
		return err
	}

	return nil
}
