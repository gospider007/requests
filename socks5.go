package requests

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"net"
	"strconv"
)

const MaxUdpPacket int = math.MaxUint16 - 28
const (
	ipv4Address = 0x01
	fqdnAddress = 0x03
	ipv6Address = 0x04
)

func WriteUdpAddr(w io.Writer, addr Address) error {
	if addr.IP != nil {
		if ip4 := addr.IP.To4(); ip4 != nil {
			con := make([]byte, 1+4+2)
			con[0] = ipv4Address
			copy(con[1:], ip4)
			binary.BigEndian.PutUint16(con[1+4:], uint16(addr.Port))
			_, err := w.Write(con)
			return err
		} else if ip6 := addr.IP.To16(); ip6 != nil {
			con := make([]byte, 1+16+2)
			con[0] = ipv6Address
			copy(con[1:], ip6)
			binary.BigEndian.PutUint16(con[1+16:], uint16(addr.Port))
			_, err := w.Write(con)
			return err
		} else {
			con := make([]byte, 1+4+2)
			con[0] = ipv4Address
			copy(con[1:], net.IPv4(0, 0, 0, 0))
			binary.BigEndian.PutUint16(con[1+4:], uint16(addr.Port))
			_, err := w.Write(con)
			return err
		}
	} else if addr.Name != "" {
		l := len(addr.Name)
		if l > 255 {
			return errors.New("errStringTooLong")
		}
		con := make([]byte, 2+l+2)
		con[0] = fqdnAddress
		con[1] = byte(l)
		copy(con[2:], []byte(addr.Name))
		binary.BigEndian.PutUint16(con[2+l:], uint16(addr.Port))
		_, err := w.Write(con)
		return err
	} else {
		con := make([]byte, 1+4+2)
		con[0] = ipv4Address
		copy(con[1:], net.IPv4(0, 0, 0, 0))
		binary.BigEndian.PutUint16(con[1+4:], uint16(addr.Port))
		_, err := w.Write(con)
		return err
	}
}

type Address struct {
	User     string
	Password string
	Name     string
	Host     string
	NetWork  string
	Scheme   string
	IP       net.IP
	Port     int
}

func (a Address) String() string {
	port := strconv.Itoa(a.Port)
	if len(a.IP) != 0 {
		return net.JoinHostPort(a.IP.String(), port)
	}
	return net.JoinHostPort(a.Name, port)
}
func (a Address) Network() string {
	return a.NetWork
}

type UDPConn struct {
	ctx context.Context
	net.PacketConn
	prefix        []byte
	bufRead       [MaxUdpPacket]byte
	bufWrite      [MaxUdpPacket]byte
	proxyAddress  net.Addr
	remoteAddress Address
}

func NewUDPConn(ctx context.Context, packConn net.PacketConn, proxyAddress net.Addr, remoteAddress Address) *UDPConn {
	return &UDPConn{
		ctx:           ctx,
		remoteAddress: remoteAddress,
		PacketConn:    packConn,
		proxyAddress:  proxyAddress,
		prefix:        []byte{0, 0, 0},
	}
}

func (c *UDPConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	n, addr, err = c.PacketConn.ReadFrom(c.bufRead[:])
	if err != nil {
		return 0, nil, err
	}
	if n < len(c.prefix) || addr.String() != c.proxyAddress.String() {
		return 0, nil, errors.New("bad header")
	}
	buf := bytes.NewBuffer(c.bufRead[len(c.prefix):n])
	a, err := readUdpAddr(buf)
	if err != nil {
		return 0, nil, err
	}
	n = copy(p, buf.Bytes())
	return n, a, nil
}

func (c *UDPConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	buf := bytes.NewBuffer(c.bufWrite[:0])
	buf.Write(c.prefix)
	err = WriteUdpAddr(buf, c.remoteAddress)
	if err != nil {
		return 0, err
	}
	n, err = buf.Write(p)
	if err != nil {
		return 0, err
	}
	_, err = c.PacketConn.WriteTo(buf.Bytes(), c.proxyAddress)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (c *UDPConn) SetReadBuffer(i int) error {
	return c.PacketConn.(*net.UDPConn).SetReadBuffer(i)
}
func (c *UDPConn) SetWriteBuffer(i int) error {
	return c.PacketConn.(*net.UDPConn).SetWriteBuffer(i)
}
func (c *UDPConn) Context() context.Context {
	return c.ctx
}
