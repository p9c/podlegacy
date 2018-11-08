package main

import "net"

// OnionAddr implements the net.Addr interface and represents a tor address.
type OnionAddr struct {
	addr string
}

// String returns the onion address.
//
// This is part of the net.Addr interface.
func (oa *OnionAddr) String() string {
	return oa.addr
}

// Network returns "onion".
//
// This is part of the net.Addr interface.
func (oa *OnionAddr) Network() string {
	return "onion"
}

// Ensure OnionAddr implements the net.Addr interface.
var _ net.Addr = (*OnionAddr)(nil)
