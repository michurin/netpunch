package netpunchlib

import (
	"errors"
	"fmt"
	"net"
	"sync/atomic"
)

type logInterface interface {
	Print(v ...interface{})
}

type logWrapper struct {
	next     Connection
	log      logInterface
	isClosed *int32
}

func LoggingMiddleware(log logInterface) ConnectionMiddleware {
	return func(conn Connection) Connection {
		return &logWrapper{
			next:     conn,
			log:      log,
			isClosed: new(int32),
		}
	}
}

func (w *logWrapper) err(area string, err error) {
	if atomic.AddInt32(w.isClosed, 0) != 0 {
		opErr := (*net.OpError)(nil)
		if errors.As(err, &opErr) {
			return // skip errors after closing
		}
	}
	w.log.Print(fmt.Sprintf("[error] %s: %s", area, err.Error()))
}

func (w *logWrapper) info(area, msg string) {
	w.log.Print(fmt.Sprintf("[info] %s: %s", area, msg))
}

func (w *logWrapper) Close() error {
	atomic.AddInt32(w.isClosed, 1) // oh. slightly too early
	err := w.next.Close()
	if err != nil {
		w.err("close", err)
		return err
	}
	w.info("close", "ok")
	return err
}

func (w *logWrapper) ReadFromUDP(b []byte) (int, *net.UDPAddr, error) {
	n, addr, err := w.next.ReadFromUDP(b)
	if err != nil {
		w.err("read", err)
		return n, addr, err
	}
	w.info("read", fmt.Sprintf("%q <- %s", b[:n], addr))
	return n, addr, err
}

func (w *logWrapper) WriteToUDP(b []byte, addr *net.UDPAddr) (int, error) {
	n, err := w.next.WriteToUDP(b, addr)
	if err != nil {
		w.err("write", err)
		return n, err
	}
	w.info("write", fmt.Sprintf("%q -> %s", b[:n], addr))
	return n, err
}
