package netpunchlib

import (
	"bytes"
	"crypto/sha256"
	"encoding/ascii85"
	"net"
)

type signWrapper struct {
	next   Connection
	secret []byte
}

func SigningMiddleware(secret []byte) ConnectionMiddleware {
	return func(conn Connection) Connection {
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
		return copy(b, []byte("[message skipped, since it is too short]")), addr, nil // data too short, pretentd it is no data
	}
	sum := w.sum(buff[signLen+1 : n])
	if !bytes.Equal(sum, buff[:signLen]) {
		return copy(b, []byte("[message skipped due to invalid signature]")), addr, nil // invalid signature, pretend it is no data
	}
	return copy(b, buff[signLen+1:n]), addr, nil
}

func (w *signWrapper) WriteToUDP(b []byte, addr *net.UDPAddr) (int, error) {
	m := len(b)
	buff := make([]byte, m+signLen+1)
	copy(buff, w.sum(b))
	buff[signLen] = 32
	copy(buff[signLen+1:], b)
	n, err := w.next.WriteToUDP(buff, addr)
	if err != nil {
		return n, err
	}
	return m, nil
}

func (w *signWrapper) sum(data []byte) []byte {
	sum := sha256.Sum224(append(w.secret, data...))
	enc := make([]byte, 35)
	ascii85.Encode(enc, sum[:])
	return enc
}
