package netpunchlib

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/ascii85"
	"fmt"
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

var signLen = ascii85.MaxEncodedLen(32) //nolint:gochecknoglobals

func (w *signWrapper) ReadFromUDP(b []byte) (int, *net.UDPAddr, error) {
	buff := make([]byte, len(b)+signLen+1)
	n, addr, err := w.next.ReadFromUDP(buff)
	if err != nil {
		return n, addr, err
	}
	if n < signLen+2 {
		return copy(b, []byte("[message skipped, since it is too short]")), addr, nil // data too short, pretentd it is no data
	}
	sum, err := w.sum(buff[signLen+1 : n])
	if err != nil {
		return n, addr, err // consider summing errors as fatal, they most likely refer to errors in code
	}
	if !hmac.Equal(sum, buff[:signLen]) { // do not use bytes.Equal, beware time leaking and timing attacks :)
		return copy(b, []byte("[message skipped due to invalid signature]")), addr, nil // invalid signature, pretend it is no data
	}
	return copy(b, buff[signLen+1:n]), addr, nil
}

func (w *signWrapper) WriteToUDP(b []byte, addr *net.UDPAddr) (int, error) {
	inputLen := len(b)
	buff := make([]byte, inputLen+signLen+1)
	sum, err := w.sum(b)
	if err != nil {
		return 0, err
	}
	if n := copy(buff, sum); n != signLen {
		return 0, fmt.Errorf("impossible sum coping error: len=%d", n)
	}
	buff[signLen] = 32
	if n := copy(buff[signLen+1:], b); n != inputLen {
		return 0, fmt.Errorf("impossible data coping error: len=%d", n)
	}
	_, err = w.next.WriteToUDP(buff, addr)
	if err != nil {
		return 0, err
	}
	return inputLen, nil // return m to pretend we wrote given data
}

func (w *signWrapper) sum(data []byte) ([]byte, error) {
	mac := hmac.New(sha256.New, w.secret)
	_, err := mac.Write(data)
	if err != nil {
		return nil, err
	}
	sum := mac.Sum(nil)
	if len(sum) != 32 {
		return nil, fmt.Errorf("impossible summing error: len=%d", len(sum))
	}
	enc := make([]byte, signLen)

	if n := ascii85.Encode(enc, sum); n != signLen {
		return nil, fmt.Errorf("impossible encoding error: len=%d", len(sum))
	}
	return enc, nil
}
