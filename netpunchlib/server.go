package app

import (
	"bytes"
	"context"
	"net"
)

func Server(ctx context.Context, address string, options ...Option) error {
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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	addresses := [][]byte{nil, nil}

	for {
		data := make([]byte, 1024) // we create new slice every time to prevent sharing memory between server and handler
		n, addr, err := conn.ReadFromUDP(data)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			continue
		}
		if n <= 0 {
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
