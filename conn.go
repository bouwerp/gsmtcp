package gsmtcp

import (
	"errors"
	"github.com/rs/zerolog/log"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

type Conn struct {
	g             *DefaultGsmModule
	remoteAddress string
}

type Reader struct {
	c net.Conn
}

func (r Reader) Read(p []byte) (n int, err error) {
	return r.c.Read(p)
}

func NewReader(c net.Conn) io.Reader {
	return Reader{c: c}
}

func NewConnection(g *DefaultGsmModule, address string) (net.Conn, error) {
	// first make sure it's a new connection
	_ = g.CloseTcpConnection()

	log.Debug().Msg("connecting to server")
	err := g.OpenTcpConnection(address)
	if err != nil {
		return nil, err
	}

	log.Debug().Msg("check if we're connected")
	connected, err := g.IsConnected()
	if err != nil {
		return nil, err
	}
	if !connected {
		return nil, errors.New("not connected")
	}

	log.Debug().Msg("successfully connected")
	return &Conn{
		g:             g,
		remoteAddress: address,
	}, nil
}

func (c Conn) Read(b []byte) (n int, err error) {
	var data []byte
	for {
		if len(data) == len(b) {
			copy(b, data)
			return len(data), nil
		}
		d, err := c.g.ReadData()
		if err != nil {
			if err == io.EOF || err.Error() == "EOF" {
				copy(b, data)
				return len(data), nil
			}
			return 0, err
		}
		data = append(data, d)
	}
}

func (c Conn) Write(b []byte) (n int, err error) {
	n, err = c.g.SendRawTcpData(b)
	if err != nil {
		switch err.(type) {
		case MaxBytesErr:
			return c.Write(b[n:])
		}
		return 0, err
	}
	return n, nil
}

func (c Conn) Close() error {
	err := c.g.CloseTcpConnection()
	if err != nil {
		return err
	}
	return nil
}

func (c Conn) LocalAddr() net.Addr {
	ip, err := c.g.GetLocalIPAddress()
	if err != nil {
		return nil
	}
	s := strings.Split(ip, ".")
	ip1, err := strconv.Atoi(s[0])
	if err != nil {
		return nil
	}
	ip2, err := strconv.Atoi(s[1])
	if err != nil {
		return nil
	}
	ip3, err := strconv.Atoi(s[2])
	if err != nil {
		return nil
	}
	ip4, err := strconv.Atoi(s[3])
	if err != nil {
		return nil
	}
	return &net.IPAddr{IP: []byte{byte(ip1), byte(ip2), byte(ip3), byte(ip4)}}
}

func (c Conn) RemoteAddr() net.Addr {
	s := strings.Split(strings.Split(c.remoteAddress, ":")[0], ".")
	ip1, err := strconv.Atoi(s[0])
	if err != nil {
		return nil
	}
	ip2, err := strconv.Atoi(s[1])
	if err != nil {
		return nil
	}
	ip3, err := strconv.Atoi(s[2])
	if err != nil {
		return nil
	}
	ip4, err := strconv.Atoi(s[3])
	if err != nil {
		return nil
	}
	return &net.IPAddr{IP: []byte{byte(ip1), byte(ip2), byte(ip3), byte(ip4)}}
}

func (c Conn) SetDeadline(t time.Time) error {
	if t.Before(time.Now()) {
		return errors.New("dealine has already passed")
	}
	c.g.TotalDeadline = t
	return nil
}

func (c Conn) SetReadDeadline(t time.Time) error {
	if t.Before(time.Now()) {
		return errors.New("dealine has already passed")
	}
	c.g.ReadDeadline = t
	return nil
}

func (c Conn) SetWriteDeadline(t time.Time) error {
	if t.Before(time.Now()) {
		return errors.New("dealine has already passed")
	}
	c.g.WriteDeadline = t
	return nil
}
