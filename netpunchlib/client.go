package netpunchlib

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"time"
)

type modeInfo struct {
	retrys  int
	delay   time.Duration
	message []byte
}

const (
	modeDiscovering = iota
	modePinging
	modePonging
	modeClosing
	modeSleeping
)

var modes = map[int]modeInfo{ //nolint:gochecknoglobals
	modeDiscovering: {
		retrys:  5,
		delay:   100 * time.Millisecond,
		message: nil,
	},
	modePinging: {
		retrys:  10,
		delay:   100 * time.Millisecond,
		message: []byte{labelPing},
	},
	modePonging: {
		retrys:  10,
		delay:   100 * time.Millisecond,
		message: []byte{labelPong},
	},
	modeClosing: {
		retrys:  5,
		delay:   20 * time.Millisecond,
		message: []byte{labelClose},
	},
	modeSleeping: {
		retrys:  1,
		delay:   30 * time.Second,
		message: nil, // not used
	},
}

func processor(
	conn ConnectionWriter,
	serverAddr *net.UDPAddr,
	serverMessage []byte,
	serverDataChan <-chan receivedMessage,
	serverErrChan <-chan error,
	addrChan chan<- *net.UDPAddr,
	errChan chan<- error,
) {
	var err error
	var peerAddr *net.UDPAddr
	mode := modeDiscovering
	tryCount := 0
	for {
		tryCount++
		minfo := modes[mode]
		if mode != modeSleeping {
			msg := minfo.message
			addr := peerAddr
			if msg == nil {
				msg = serverMessage
				addr = serverAddr
			}
			_, err = conn.WriteToUDP(msg, addr)
			if err != nil {
				errChan <- err
				return
			}
		}
		select {
		case <-time.After(minfo.delay):
			if tryCount >= minfo.retrys { // perform transition if count of tries exhausted
				switch mode { // sort of FSM transition table
				case modeClosing:
					addrChan <- peerAddr
					return
				case modeSleeping:
					mode = modeDiscovering
				default:
					mode = modeSleeping
				}
				tryCount = 0
			}
		case data := <-serverDataChan:
			if len(data.message) == 0 {
				break // ignore empty messages
			}
			switch data.message[0] {
			case labelPeerInfo:
				flds := bytes.Split(data.message, []byte{labelsSeporator})
				if len(flds) != 3 {
					break // ignore invalid messages
				}
				peerAddr, err = net.ResolveUDPAddr("udp", string(flds[2]))
				if err != nil {
					errChan <- err
					return
				}
				mode = modePinging // start pinging
			case labelPing:
				peerAddr = data.addr // ping can come before first peer info response
				mode = modePonging
			case labelPong:
				peerAddr = data.addr // and pong can too
				mode = modeClosing
			case labelClose:
				addrChan <- peerAddr
				return
			}
			tryCount = 0
		case err := <-serverErrChan:
			errChan <- err
			return
		}
	}
}

func buildMessage(s string) ([]byte, error) {
	m := []byte(s)
	if len(m) != 1 {
		return nil, fmt.Errorf("invalid slot (role): %q", s)
	}
	if m[0] < 'a' || m[0] > 'z' {
		return nil, fmt.Errorf("invalid slot (role): %q", s)
	}
	return m, nil
}

func Client(ctx context.Context, slot, address, remoteAddress string, opt ...Option) (*net.UDPAddr, *net.UDPAddr, error) {
	message, err := buildMessage(slot)
	if err != nil {
		return nil, nil, err
	}

	config := newConfig(opt...)

	laddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, nil, err
	}
	addr, err := net.ResolveUDPAddr("udp", remoteAddress)
	if err != nil {
		return nil, nil, err
	}

	udpConn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		return nil, nil, err
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

	addrChan := make(chan *net.UDPAddr)
	errChan := make(chan error)

	go processor(conn, addr, message, serverDataChan, serverErrChan, addrChan, errChan)

	select {
	case addr := <-addrChan:
		return laddr, addr, nil
	case err := <-errChan:
		return nil, nil, err
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
}
