package requests

import (
	"bytes"
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
	proxyAddress net.Addr
	net.PacketConn
	prefix   []byte
	bufRead  [MaxUdpPacket]byte
	bufWrite [MaxUdpPacket]byte
}

func NewUDPConn(packConn net.PacketConn, proxyAddress net.Addr) *UDPConn {
	return &UDPConn{
		PacketConn:   packConn,
		proxyAddress: proxyAddress,
		prefix:       []byte{0, 0, 0},
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
	a, err := ReadUdpAddr(buf)
	if err != nil {
		return 0, nil, err
	}
	n = copy(p, buf.Bytes())
	return n, a, nil
}

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
	_, err = c.PacketConn.WriteTo(buf.Bytes(), c.proxyAddress)
	if err != nil {
		return 0, err
	}
	return n, nil
}
