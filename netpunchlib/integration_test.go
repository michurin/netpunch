package netpunchlib_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
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

	// Settings

	host := "127.0.0.1"
	peers := 26
	ctrlPort := 10000
	peerBasePort := ctrlPort + 1

	// Prepare

	ctrlAddr := fmt.Sprintf("%s:%d", host, ctrlPort)

	type result struct {
		role string
		a    *net.UDPAddr
		b    *net.UDPAddr
		err  error
	}

	peerDone := make(chan result, peers)
	ctrlDone := make(chan error, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // 5s is global test timeout; out of the blue
	defer cancel()                                                          // force all agents to die

	// Run

	go func() {
		ctrlDone <- netpunchlib.Server(ctx, ctrlAddr, opt("server"))
	}()
	for i := 0; i < peers; i++ {
		go func(role, peerAddr string) {
			a, b, err := netpunchlib.Client(ctx, role, peerAddr, ctrlAddr, opt("peer "+role))
			peerDone <- result{role: role, a: a, b: b, err: err}
		}(string(byte(i)+'a'), fmt.Sprintf("%s:%d", host, peerBasePort+i))
	}

	// Results collecting

	results := map[string]result{} // we collect all results to be sure we don't have any duplicates or misses
LOOP:
	for {
		select {
		case err := <-ctrlDone:
			if errors.Is(err, context.Canceled) && len(results) == peers { // everything ok
				break LOOP
			}
			t.Fatal(err) // anyway it is error
		case res := <-peerDone:
			require.NoError(t, res.err)
			_, ok := results[res.role]
			require.False(t, ok)
			results[res.role] = res
		}
		if len(results) == peers {
			cancel() // break loop by stopping server
		}
	}

	// Asserts

	for role, res := range results {
		assert.Len(t, res.role, 1)
		assert.Equal(t, res.role, role)
		assert.NoError(t, res.err)
		slotA := int([]byte(role)[0] - 'a')
		slotB := slotA ^ 1
		assert.Equal(t, peerBasePort+slotA, res.a.Port)
		assert.Equal(t, peerBasePort+slotB, res.b.Port)
	}
}

func opt(p string) netpunchlib.Option {
	return netpunchlib.ConnOption(netpunchlib.LoggingMiddleware(log.New(os.Stderr, "["+p+"] ", 0)))
}
