package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/ascii85"
	"errors"
	"net"
)

type signWrapper struct {
	next   Connenction
	secret []byte
}

func SignMW(secret []byte) MW {
	return func(conn Connenction) Connenction {
		return &signWrapper{
			next:   conn,
			secret: secret,
		}
	}
}

func (w *signWrapper) Close() error {
	return w.next.Close()
}

var signLen = ascii85.MaxEncodedLen(sha256.Size224) //nolint:gochecknoglobals

func (w *signWrapper) ReadFromUDP(b []byte) (int, *net.UDPAddr, error) {
	buff := make([]byte, len(b)+signLen+1)
	n, addr, err := w.next.ReadFromUDP(buff)
	if err != nil {
		return n, addr, err
	}
	if n < signLen+2 {
		return 0, addr, errors.New("message too short")
	}
	sum := w.sum(buff[signLen+1 : n])
	if !bytes.Equal(sum, buff[:signLen]) {
		return 0, addr, errors.New("invalid signature")
	}
	copy(b, buff[signLen+1:n])
	return n - signLen - 1, addr, nil
}

func (w *signWrapper) WriteToUDP(b []byte, addr *net.UDPAddr) (int, error) {
	buff := make([]byte, len(b)+signLen+1)
	copy(buff, w.sum(b))
	buff[signLen] = 32
	copy(buff[signLen+1:], b)
	n, err := w.next.WriteToUDP(buff, addr)
	if err != nil {
		return n, err
	}
	return n - signLen - 1, nil
}

func (w *signWrapper) sum(data []byte) []byte {
	sum := sha256.Sum224(append(w.secret, data...))
	enc := make([]byte, 35)
	ascii85.Encode(enc, sum[:])
	return enc
}
