package netpunchlib

import "net"

type ConnectionReader interface {
	ReadFromUDP(data []byte) (int, *net.UDPAddr, error)
}

type ConnectionWriter interface {
	WriteToUDP(data []byte, addr *net.UDPAddr) (int, error)
}

type ConnectionCloser interface {
	Close() error
}

//go:generate mockgen -source=$GOFILE -destination=./internal/mock/$GOFILE -package=mock
type Connection interface {
	ConnectionReader
	ConnectionWriter
	ConnectionCloser
}

type ConnectionMiddleware func(Connection) Connection
