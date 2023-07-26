package socks

import (
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
)

// AddrSpec is used to return the target AddrSpec
// which may be specified as IPv4, IPv6, or a FQDN
type AddrSpec struct {
	FQDN string
	IP   net.IP
	Port int
}

func (a *AddrSpec) String() string {
	if a.FQDN != "" {
		return fmt.Sprintf("%s (%s):%d", a.FQDN, a.IP, a.Port)
	}
	return fmt.Sprintf("%s:%d", a.IP, a.Port)
}

// Address returns a string suitable to dial; prefer returning IP-based
// address, fallback to FQDN
func (a AddrSpec) Address() string {
	if len(a.IP) > 0 {
		return net.JoinHostPort(a.IP.String(), strconv.Itoa(a.Port))
	}
	return net.JoinHostPort(a.FQDN, strconv.Itoa(a.Port))
}

// A Request represents request received by a server
type Request struct {
	// Protocol version
	Version uint8
	// Requested command
	Command uint8
	// AddrSpec of the the network that sent the request
	RemoteAddr *AddrSpec
	// AddrSpec of the desired destination
	DestAddr *AddrSpec
}

func (r *Request) ParseRequest(conn net.Conn) error {
	header := []byte{0, 0, 0}
	if _, err := io.ReadAtLeast(conn, header, 3); err != nil {
		return fmt.Errorf("read 3 bytes header error:%s", err.Error())
	}

	if header[0] != 0x05 {
		return fmt.Errorf("unsupported proxy version")
	}

	// Read in the destination address
	dest, err := readAddrSpec(conn)
	if err != nil {
		return err
	}
	remote, _ := conn.RemoteAddr().(*net.TCPAddr)

	r.Version = 0x05
	r.Command = header[1]
	r.DestAddr = dest
	r.RemoteAddr = &AddrSpec{IP: remote.IP, Port: remote.Port}

	return nil
}

// readAddrSpec is used to read AddrSpec.
// Expects an address type byte, follwed by the address and port
func readAddrSpec(r io.Reader) (*AddrSpec, error) {
	d := &AddrSpec{}

	// Get the address type
	addrType := []byte{0}
	if _, err := r.Read(addrType); err != nil {
		return nil, fmt.Errorf("read addr type error:%s", err.Error())
	}

	// Handle on a per type basis
	switch addrType[0] {
	case ipv4Address:
		addr := make([]byte, 4)
		if _, err := io.ReadAtLeast(r, addr, 4); err != nil {
			return nil, fmt.Errorf("read ipv4 add error:%s", err.Error())
		}
		d.IP = net.IP(addr)

	case ipv6Address:
		addr := make([]byte, 16)
		if _, err := io.ReadAtLeast(r, addr, 16); err != nil {
			return nil, fmt.Errorf("read ipv6 add error:%s", err.Error())
		}
		d.IP = net.IP(addr)

	case fqdnAddress:
		buf := []byte{0}
		if _, err := r.Read(buf); err != nil {
			return nil, fmt.Errorf("read fqdn len error:%s", err.Error())
		}
		addrLen := int(buf[0])
		fqdn := make([]byte, addrLen)
		if _, err := io.ReadAtLeast(r, fqdn, addrLen); err != nil {
			return nil, fmt.Errorf("read fqdn %d bytes error:%s", addrLen, err.Error())
		}
		d.FQDN = string(fqdn)

	default:
		return nil, fmt.Errorf("unrecognized address type")
	}

	// Read the port
	port := []byte{0, 0}
	if _, err := io.ReadAtLeast(r, port, 2); err != nil {
		return nil, fmt.Errorf("read 2 bytes port error:%s", err.Error())
	}
	d.Port = (int(port[0]) << 8) | int(port[1])

	return d, nil
}

// SendReply is used to send a reply message
func SendReply(w io.Writer, rep uint8, addr *AddrSpec) error {
	// Format the address
	var addrType uint8
	var addrBody []byte
	var addrPort uint16
	switch {
	case addr == nil:
		addrType = ipv4Address
		addrBody = []byte{0, 0, 0, 0}
		addrPort = 0

	case addr.FQDN != "":
		addrType = fqdnAddress
		addrBody = append([]byte{byte(len(addr.FQDN))}, addr.FQDN...)
		addrPort = uint16(addr.Port)

	case addr.IP.To4() != nil:
		addrType = ipv4Address
		addrBody = []byte(addr.IP.To4())
		addrPort = uint16(addr.Port)

	case addr.IP.To16() != nil:
		addrType = ipv6Address
		addrBody = []byte(addr.IP.To16())
		addrPort = uint16(addr.Port)

	default:
		err := fmt.Errorf("failed to format address: %v", addr)
		log.Printf("failed to send reply, error: %s, client: , target: ", err.Error())
		return err
	}

	// Format the message
	reply := make([]byte, 6+len(addrBody))
	reply[0] = socks5Version
	reply[1] = rep
	reply[2] = 0 // Reserved
	reply[3] = addrType
	copy(reply[4:], addrBody)
	reply[4+len(addrBody)] = byte(addrPort >> 8)
	reply[4+len(addrBody)+1] = byte(addrPort & 0xff)

	// Send the message
	_, err := w.Write(reply)
	if err != nil {
		log.Printf("failed to send reply, error: %s, client: , target: ", err.Error())
	}
	return err
}
