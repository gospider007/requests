package requests

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"net"
	"strconv"
	"time"
)

var (
	errBadHeader = errors.New("bad header")
	// errUnsupportedMethod = errors.New("unsupported method")
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
		if len(addr.Name) > 255 {
			return errors.New("errStringTooLong")
		}
		con := make([]byte, 2+len(addr.Name))
		con[0] = fqdnAddress
		con[1] = byte(len(addr.Name))
		copy(con[1:], []byte(addr.Name))
		_, err := w.Write(con)
		return err
	} else {
		_, err := w.Write([]byte{ipv4Address, 0, 0, 0, 0})
		return err
	}
}

type Address struct {
	User     string
	Password string
	Name     string
	Host     string
	IP       net.IP
	Port     int
	NetWork  string
	Scheme   string
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
func ReadUdpAddr(r io.Reader) (Address, error) {
	UdpAddress := Address{}
	var addrType [1]byte
	if _, err := r.Read(addrType[:]); err != nil {
		return UdpAddress, err
	}

	switch addrType[0] {
	case ipv4Address:
		addr := make(net.IP, net.IPv4len)
		if _, err := io.ReadFull(r, addr); err != nil {
			return UdpAddress, err
		}
		UdpAddress.IP = addr
	case ipv6Address:
		addr := make(net.IP, net.IPv6len)
		if _, err := io.ReadFull(r, addr); err != nil {
			return UdpAddress, err
		}
		UdpAddress.IP = addr
	case fqdnAddress:
		if _, err := r.Read(addrType[:]); err != nil {
			return UdpAddress, err
		}
		addrLen := int(addrType[0])
		fqdn := make([]byte, addrLen)
		if _, err := io.ReadFull(r, fqdn); err != nil {
			return UdpAddress, err
		}
		UdpAddress.Name = string(fqdn)
	default:
		return UdpAddress, errors.New("invalid atyp")
	}
	var port [2]byte
	if _, err := io.ReadFull(r, port[:]); err != nil {
		return UdpAddress, err
	}
	UdpAddress.Port = int(binary.BigEndian.Uint16(port[:]))
	return UdpAddress, nil
}

type UDPConn struct {
	bufRead      [MaxUdpPacket]byte
	bufWrite     [MaxUdpPacket]byte
	proxyAddress net.Addr
	// defaultTarget net.Addr
	prefix []byte
	net.PacketConn
	UDPConn *net.UDPConn
}

func NewUDPConn(packConn net.PacketConn, proxyAddress net.Addr) (*UDPConn, error) {
	conn := &UDPConn{
		PacketConn:   packConn,
		UDPConn:      packConn.(*net.UDPConn),
		proxyAddress: proxyAddress,
		prefix:       []byte{0, 0, 0},
	}
	return conn, nil
}

// func NewUDPConn(raw net.PacketConn, proxyAddress net.Addr, defaultTarget net.Addr) (*UDPConn, error) {
// 	conn := &UDPConn{
// 		PacketConn:    raw,
// 		proxyAddress:  proxyAddress,
// 		defaultTarget: defaultTarget,
// 		prefix:        []byte{0, 0, 0},
// 	}
// 	return conn, nil
// }

// ReadFrom implements the net.PacketConn ReadFrom method.
func (c *UDPConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	n, addr, err = c.PacketConn.ReadFrom(c.bufRead[:])
	if err != nil {
		return 0, nil, err
	}
	if n < len(c.prefix) || addr.String() != c.proxyAddress.String() {
		return 0, nil, errBadHeader
	}
	buf := bytes.NewBuffer(c.bufRead[len(c.prefix):n])
	a, err := ReadUdpAddr(buf)
	if err != nil {
		return 0, nil, err
	}
	n = copy(p, buf.Bytes())
	return n, a, nil
}

// WriteTo implements the net.PacketConn WriteTo method.
func (c *UDPConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	buf := bytes.NewBuffer(c.bufWrite[:0])
	buf.Write(c.prefix)
	udpAddr := addr.(*net.UDPAddr)
	err = WriteUdpAddr(buf, Address{IP: udpAddr.IP, Port: udpAddr.Port})
	if err != nil {
		return 0, err
	}
	n, err = buf.Write(p)
	if err != nil {
		return 0, err
	}

	data := buf.Bytes()
	_, err = c.PacketConn.WriteTo(data, c.proxyAddress)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// // Read implements the net.Conn Read method.
// func (c *UDPConn) Read(b []byte) (int, error) {
// 	n, addr, err := c.ReadFrom(b)
// 	if err != nil {
// 		return 0, err
// 	}
// 	if addr.String() != c.defaultTarget.String() {
// 		return c.Read(b)
// 	}
// 	return n, nil
// }

// // Write implements the net.Conn Write method.
// func (c *UDPConn) Write(b []byte) (int, error) {
// 	return c.WriteTo(b, c.defaultTarget)
// }

// // RemoteAddr implements the net.Conn RemoteAddr method.
// func (c *UDPConn) RemoteAddr() net.Addr {
// 	return c.defaultTarget
// }

func (c *UDPConn) SetReadBuffer(bytes int) error {
	return c.UDPConn.SetReadBuffer(bytes)
}
func (c *UDPConn) SetWriteBuffer(bytes int) error {
	return c.UDPConn.SetWriteBuffer(bytes)
}
func (c *UDPConn) SetDeadline(t time.Time) error {
	return c.UDPConn.SetDeadline(t)
}
func (c *UDPConn) SetReadDeadline(t time.Time) error {
	return c.UDPConn.SetReadDeadline(t)
}
func (c *UDPConn) SetWriteDeadline(t time.Time) error {
	return c.UDPConn.SetWriteDeadline(t)
}
func (c *UDPConn) ReadFromUDP(b []byte) (n int, addr *net.UDPAddr, err error) {
	return c.UDPConn.ReadFromUDP(b)
}
func (c *UDPConn) ReadMsgUDP(b, oob []byte) (n, oobn, flags int, addr *net.UDPAddr, err error) {
	return c.UDPConn.ReadMsgUDP(b, oob)
}
func (c *UDPConn) WriteToUDP(b []byte, addr *net.UDPAddr) (int, error) {
	return c.UDPConn.WriteToUDP(b, addr)
}
func (c *UDPConn) WriteMsgUDP(b, oob []byte, addr *net.UDPAddr) (n, oobn int, err error) {
	return c.UDPConn.WriteMsgUDP(b, oob, addr)
}
