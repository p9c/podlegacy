package server

import "net"

// SimpleAddr implements the net.Addr interface with two struct fields
type SimpleAddr struct {
	net, addr string
}

// String returns the address.
//
// This is part of the net.Addr interface.
func (a SimpleAddr) String() string {
	return a.addr
}

// Network returns the network.
//
// This is part of the net.Addr interface.
func (a SimpleAddr) Network() string {
	return a.net
}

// Ensure SimpleAddr implements the net.Addr interface.
var _ net.Addr = SimpleAddr{}
