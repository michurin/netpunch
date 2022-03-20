package app

import (
	"bytes"
	"fmt"
	"net"
	"time"
)

type task struct {
	message  []byte
	addr     *net.UDPAddr
	tries    int
	interval time.Duration
	fin      bool
}

type result struct {
	addr *net.UDPAddr
	err  error
}

func taskPing(addr *net.UDPAddr) task {
	return task{
		message:  []byte{labelPing},
		addr:     addr,
		tries:    20,
		interval: 100 * time.Millisecond,
		fin:      false,
	}
}

func taskPong(addr *net.UDPAddr) task {
	return task{
		message:  []byte{labelPong},
		addr:     addr,
		tries:    20,
		interval: 100 * time.Millisecond,
		fin:      false,
	}
}

func taskClose(addr *net.UDPAddr) task {
	return task{
		message:  []byte{labelClose},
		addr:     addr,
		tries:    5,
		interval: 50 * time.Millisecond,
		fin:      true, // it is final task
	}
}

func taskRequestToServer(addr *net.UDPAddr, message []byte) task {
	return task{
		message:  message,
		addr:     addr,
		tries:    -1, // infinite
		interval: 20 * time.Second,
		fin:      false,
	}
}

func taskEexecutor(
	conn Connenction,
	serverAddr *net.UDPAddr,
	serverMessage []byte,
	tq chan task,
	res chan result,
) {
	defaultTask := taskRequestToServer(serverAddr, serverMessage)
	tsk := defaultTask
	ok := true
	for ok {
		_, err := conn.WriteToUDP(tsk.message, tsk.addr)
		if err != nil {
			res <- result{addr: nil, err: err}
			return
		}
		if tsk.tries > 0 {
			tsk.tries--
		}
		if tsk.tries == 0 {
			if tsk.fin {
				res <- result{addr: tsk.addr, err: nil}
				return
			}
			tsk = defaultTask // back to server polling
		}
		select {
		case <-time.After(tsk.interval):
		case tsk, ok = <-tq: // stop looping if channel is closed
		}
	}
}

func serveForever(conn Connenction, tq chan task, res chan result) {
	buff := make([]byte, 1024)
	for {
		n, addr, err := conn.ReadFromUDP(buff)
		if err != nil { // TODO consider is fatal error? maybe skip?
			res <- result{addr: nil, err: err}
			close(tq) // avoid goroutine leaking
			break
		}
		if n == 0 {
			continue
		}
		switch buff[0] {
		case labelPeerInfo:
			flds := bytes.Split(buff[:n], []byte{labelsSeporator})
			if len(flds) != 3 {
				break
			}
			peerAddr, err := net.ResolveUDPAddr("udp", string(flds[2]))
			if err != nil {
				// TODO log? stop?
				break
			}
			tq <- taskPing(peerAddr)
		case labelPing:
			tq <- taskPong(addr)
		case labelPong:
			tq <- taskClose(addr) // task "close" will stop executor after all tries
			return                // stop listening on first pong
		case labelClose:
			close(tq) // stop execution immediately
			res <- result{addr: addr, err: nil}
			return // stop listening on first close
		default:
			// TODO Unexpected data. Log? stop? sleep?
		}
	}
}

func Client(slot, address, remoteAddress string, opt ...Option) (*net.UDPAddr, *net.UDPAddr, error) {
	if slot != "a" && slot != "b" {
		return nil, nil, fmt.Errorf("invalid slot (role): %s", slot)
	}
	message := []byte(slot)

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
	defer conn.Close()

	taskQueue := make(chan task, 8)
	resultChan := make(chan result, 1)

	go taskEexecutor(conn, addr, message, taskQueue, resultChan)
	go serveForever(conn, taskQueue, resultChan)

	res := <-resultChan
	return laddr, res.addr, res.err
}
