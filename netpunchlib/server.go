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
		cancel()         // we must to cancel first
		_ = conn.Close() // will be closed synchronously
	}()

	serverDataChan := make(chan receivedMessage)
	serverErrChan := make(chan error)

	go serve(ctx, conn, serverDataChan, serverErrChan)

	addresses := make([][]byte, 26)

	for {
		select {
		case data := <-serverDataChan:
			if len(data.message) != 1 {
				continue
			}
			slot := data.message[0]
			if slot < 'a' || slot > 'z' {
				continue
			}
			idx := int(slot - 'a')
			addresses[idx] = bytes.Join([][]byte{ //nolint:gosec // we checked range before
				{labelPeerInfo},
				data.message[:1],
				[]byte(data.addr.String()),
			}, []byte{labelsSeporator})
			payload := addresses[idx^1] //nolint:gosec // len is even
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
