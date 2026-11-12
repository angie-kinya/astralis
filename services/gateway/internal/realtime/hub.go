// Package realtime pushes small events to a user's open tabs over SSE.
//
// the server pushes, the client still sends with ordinary POSTs. that is all a
// chat and a status line need, it is plain HTTP through Caddy, and EventSource
// reconnects on its own. websockets only pay off for fast two-way traffic
package realtime

import "sync"

// Event types, so publishers and the frontend agree on one spelling
const (
	MessageNew     = "message.new"     // {peerId, message}
	MessageDeleted = "message.deleted" // {peerId, messageId}
	MessageRead    = "message.read"    // {peerId}
	TurnStatus     = "turn.status"     // {sessionId, status}
)

type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// Hub is an in-memory pub/sub keyed by user. one process, one droplet: enough.
// a second api container would only see its own subscribers; at that point back
// Publish with postgres LISTEN/NOTIFY and keep this type as the local fan-out
type Hub struct {
	mu   sync.RWMutex
	subs map[string]map[chan Event]struct{}
	done chan struct{}
	stop sync.Once
}

func NewHub() *Hub {
	return &Hub{subs: map[string]map[chan Event]struct{}{}, done: make(chan struct{})}
}

// Done closes when the server shuts down. streams select on it and return, so
// http.Server.Shutdown is not left waiting 15s on tabs that never hang up
func (h *Hub) Done() <-chan struct{} { return h.done }

// Close ends every stream; wired to http.Server.RegisterOnShutdown
func (h *Hub) Close() { h.stop.Do(func() { close(h.done) }) }

// Subscribe registers one tab. call the returned func when the stream ends (twice is safe);
// when the stream ends; it removes the channel before closing it, so Publish
// can never send on a closed channel
func (h *Hub) Subscribe(userID string) (<-chan Event, func()) {
	ch := make(chan Event, 16)
	h.mu.Lock()
	if h.subs[userID] == nil {
		h.subs[userID] = map[chan Event]struct{}{}
	}
	h.subs[userID][ch] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	return ch, func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.subs[userID], ch)
			if len(h.subs[userID]) == 0 {
				delete(h.subs, userID) // no empty maps left behind per signed-out user
			}
			h.mu.Unlock()
			close(ch)
		})
	}
}

// Publish never blocks: a slow tab drops an event and recovers on its next fetch.
// nil-safe so a server built without a hub (tests) simply publishes nothing
func (h *Hub) Publish(userID string, e Event) {
	if h == nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subs[userID] {
		select {
		case ch <- e:
		default:
		}
	}
}

// Subscribers reports open streams for a user; used by tests and nothing else yet
func (h *Hub) Subscribers(userID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs[userID])
}
