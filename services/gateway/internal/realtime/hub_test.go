package realtime

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublishReachesOnlyThatUser(t *testing.T) {
	h := NewHub()
	a1, ua1 := h.Subscribe("a")
	a2, ua2 := h.Subscribe("a") // a second tab
	b, ub := h.Subscribe("b")
	defer ua1()
	defer ua2()
	defer ub()

	h.Publish("a", Event{Type: MessageNew})
	require.Equal(t, MessageNew, (<-a1).Type)
	require.Equal(t, MessageNew, (<-a2).Type)
	select {
	case e := <-b:
		t.Fatalf("b received %v", e)
	default:
	}
}

func TestPublishNeverBlocksOnAFullTab(t *testing.T) {
	h := NewHub()
	_, unsub := h.Subscribe("a")
	defer unsub()
	for range 100 { // buffer is 16; the rest must be dropped, not wait
		h.Publish("a", Event{Type: TurnStatus})
	}
}

func TestUnsubscribeCleansUpAndIsIdempotent(t *testing.T) {
	h := NewHub()
	ch, unsub := h.Subscribe("a")
	unsub()
	unsub()
	_, open := <-ch
	require.False(t, open)
	require.Equal(t, 0, h.Subscribers("a"))
	h.Publish("a", Event{Type: MessageNew}) // no subscriber, no panic
}

func TestNilHubPublishIsANoop(t *testing.T) {
	var h *Hub
	h.Publish("a", Event{Type: MessageNew})
}

// run with -race: publishers and subscribers churning at once
func TestConcurrentPublishAndUnsubscribe(t *testing.T) {
	h := NewHub()
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for range 200 {
				_, unsub := h.Subscribe("a")
				unsub()
			}
		}()
		go func() {
			defer wg.Done()
			for range 200 {
				h.Publish("a", Event{Type: MessageNew})
			}
		}()
	}
	wg.Wait()
	require.Equal(t, 0, h.Subscribers("a"))
}

func TestCloseSignalsDoneOnce(t *testing.T) {
	h := NewHub()
	h.Close()
	h.Close()
	<-h.Done()
}
