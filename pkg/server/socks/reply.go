package socks

import (
	"fmt"
	"io"
)

// SendReply is used to send a reply message to a client
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
		return fmt.Errorf("failed to format address: %v", addr)
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
		return fmt.Errorf("failed to send reply: %w", err)
	}
	return nil
}
