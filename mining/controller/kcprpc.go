package controller

// This is the RPC for the controller. Creating and destroying the listener, encoding/decoding messages from messagepack binary format, and registering/unregistering listener clients.
// On a LAN, other than perhaps for WLANs, one does not need the overhead of encrypting the messages, and for connections over insecure paths, the use of a PSK acts to exclude non-authorised clients from subscription access and returning solved blocks
