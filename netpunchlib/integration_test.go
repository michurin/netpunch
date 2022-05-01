package netpunchlib_test

import (
	"context"
	"errors"
	"log"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/michurin/netpunch/netpunchlib"
)

func TestRegularInteraction(t *testing.T) {
	// Extremely naive integration test,
	// it does something, however it has a lot of misfeatures
	// - it doesn't mock network interaction
	// - it doesn't check port availability!
	// - it doesn't check retries carefully
	// - it is race conditions prone (however it doesn't ruin test)

	ctlPeer := "127.0.0.1:10000"
	peerA := "127.0.0.1:10001"
	peerB := "127.0.0.1:10002"

	type result struct {
		a   *net.UDPAddr
		b   *net.UDPAddr
		err error
	}

	aDone := make(chan result, 1)
	bDone := make(chan result, 1)
	cDone := make(chan error, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // 5s is global test timeout; out of the blue
	defer cancel()                                                          // force all agents to die

	opt := netpunchlib.ConnOption(netpunchlib.LoggingMiddleware(log.Default())) // it would be great to write custom logger with t.Log

	go func() {
		cDone <- netpunchlib.Server(ctx, ctlPeer, opt)
	}()
	go func() {
		a, b, err := netpunchlib.Client(ctx, "a", peerA, ctlPeer, opt)
		aDone <- result{a: a, b: b, err: err}
	}()
	go func() {
		a, b, err := netpunchlib.Client(ctx, "b", peerB, ctlPeer, opt)
		bDone <- result{a: a, b: b, err: err}
	}()

	aCount := 0
	bCount := 0
LOOP:
	for {
		select {
		case err := <-cDone:
			if errors.Is(err, context.Canceled) && aCount == 1 && bCount == 1 { // everything ok
				break LOOP
			}
			t.Fatal(err) // anyway it is error
		case res := <-aDone:
			require.NoError(t, res.err)
			assert.Equal(t, 10001, res.a.Port)
			assert.Equal(t, 10002, res.b.Port)
			aCount++
		case res := <-bDone:
			require.NoError(t, res.err)
			assert.Equal(t, 10002, res.a.Port)
			assert.Equal(t, 10001, res.b.Port)
			bCount++
		}
		if aCount == 1 && bCount == 1 {
			cancel() // stop server
		}
	}
}
