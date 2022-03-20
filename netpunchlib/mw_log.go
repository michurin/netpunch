package app

import (
	"fmt"
	"log"
	"net"
)

type logWrapper struct {
	next Connenction
	log  *log.Logger
}

func LogMW(log *log.Logger) MW {
	return func(conn Connenction) Connenction {
		return &logWrapper{
			next: conn,
			log:  log,
		}
	}
}

func (w *logWrapper) err(area string, err error) {
	w.log.Print(fmt.Sprintf("[error] %s: %s", area, err.Error()))
}

func (w *logWrapper) info(area, msg string) {
	w.log.Print(fmt.Sprintf("[info] %s: %s", area, msg))
}

func (w *logWrapper) Close() error {
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
