package netpunchlib

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
	defer func() {
		cancel()     // we must to cancel first
		conn.Close() // will be closed synchronously
	}()

	serverDataChan := make(chan receivedMessage)
	serverErrChan := make(chan error)

	go serve(ctx, conn, serverDataChan, serverErrChan)

	addresses := [][]byte{nil, nil}

	for {
		select {
		case data := <-serverDataChan:
			if len(data.message) != 1 {
				continue
			}
			switch data.message[0] {
			case 'a', 'b':
			default:
				continue
			}
			idx := int(data.message[0]) & 1
			addresses[idx] = bytes.Join([][]byte{
				{labelPeerInfo},
				data.message[:1],
				[]byte(data.addr.String()),
			}, []byte{labelsSeporator})
			payload := addresses[idx^1]
			if payload == nil {
				continue
			}
			_, err = conn.WriteToUDP(payload, data.addr)
			if err != nil {
				continue
			}
		case err := <-serverErrChan:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
