package ws

import (
	"sync"
	"time"
)

// Session tracks a client's WebSocket connection and state.
type Session struct {
	ID        string
	conn      *Connection
	CreatedAt time.Time
	LastSeen  time.Time
	mu        sync.Mutex

	// Buffered messages for replay on reconnect
	buffer    []*Message
	bufferMax int
}

func newSession(id string, conn *Connection) *Session {
	return &Session{
		ID:        id,
		conn:      conn,
		CreatedAt: time.Now(),
		LastSeen:  time.Now(),
		bufferMax: 100,
	}
}

// Send sends a message to the session's connection.
// If the connection is nil (disconnected), the message is buffered.
func (s *Session) Send(msg *Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.LastSeen = time.Now()

	if s.conn == nil {
		// Buffer for replay on reconnect
		if len(s.buffer) < s.bufferMax {
			s.buffer = append(s.buffer, msg)
		}
		return nil
	}

	return s.conn.Send(msg)
}

// Reconnect attaches a new connection and replays buffered messages.
func (s *Session) Reconnect(conn *Connection) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.conn = conn
	s.LastSeen = time.Now()

	// Replay buffered messages
	for _, msg := range s.buffer {
		if err := conn.Send(msg); err != nil {
			return err
		}
	}
	s.buffer = nil

	return nil
}

// Disconnect sets the connection to nil, allowing buffering.
func (s *Session) Disconnect() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conn = nil
}

// IsConnected returns whether the session has an active connection.
func (s *Session) IsConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn != nil
}
