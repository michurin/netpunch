package app

import (
	"bytes"
	"net"
)

func Server(address string, options ...Option) error {
	config := newConfig(options...)
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return err
	}
	udpConn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	conn := config.wrapConnection(udpConn)
	defer conn.Close()

	addresses := [][]byte{nil, nil}

	for {
		data := make([]byte, 1024) // we create new slice every time to prevent sharing memory between server and handler
		n, addr, err := conn.ReadFromUDP(data)
		if err != nil {
			continue
		}
		if n < 1 {
			continue
		}
		idx := int(data[0]) & 1
		addresses[idx] = bytes.Join([][]byte{
			{labelPeerInfo},
			data[:1],
			[]byte(addr.String()),
		}, []byte{labelsSeporator})
		payload := addresses[idx^1]
		if payload == nil {
			continue
		}
		_, err = conn.WriteToUDP(payload, addr)
		if err != nil {
			continue
		}
	}
}
