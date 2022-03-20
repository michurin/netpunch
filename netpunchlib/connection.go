package app

import "net"

//go:generate mockgen -source=$GOFILE -destination=./mock/$GOFILE -package=mock
type Connenction interface {
	Close() error
	ReadFromUDP([]byte) (int, *net.UDPAddr, error)
	WriteToUDP([]byte, *net.UDPAddr) (int, error)
}

type MW func(Connenction) Connenction
