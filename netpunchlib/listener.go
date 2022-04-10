package netpunchlib

import (
	"context"
	"net"
)

type receivedMessage struct {
	message []byte
	addr    *net.UDPAddr
}

func serve(ctx context.Context, conn ConnectionReader, serverDataChan chan<- receivedMessage, serverErrChan chan<- error) {
	// You are to manage this function gently
	// To avoid hanging and panics keep in mind:
	// - it is bad idea to close args channels
	// - you have to cancel context before closing connection
	// It's not unforgivable if we do it in private helper function, however
	// you might think twice before you borrow this code
	for {
		buff := make([]byte, 1024)
		n, addr, err := conn.ReadFromUDP(buff) // will be interrupted by closing connection
		if ctx.Err() != nil {                  // we must *not* use channels after canceling
			return
		}
		if err != nil {
			serverErrChan <- err
			return
		}
		serverDataChan <- receivedMessage{
			message: buff[:n],
			addr:    addr,
		}
	}
}
