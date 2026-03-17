package ws

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"nhooyr.io/websocket"
)

const sessionGracePeriod = 5 * time.Minute

// MessageHandler is called for each incoming WebSocket message.
type MessageHandler func(session *Session, msg *Message)

// Server manages WebSocket connections and sessions.
type Server struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	handler  MessageHandler
}

// NewServer creates a new WebSocket server.
func NewServer(handler MessageHandler) *Server {
	s := &Server{
		sessions: make(map[string]*Session),
		handler:  handler,
	}

	// Start session cleanup goroutine
	go s.cleanupLoop()

	return s
}

// HandleWebSocket is the HTTP handler for WebSocket upgrade.
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		log.Printf("ws accept error: %v", err)
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	conn := newConnection(r.Context(), ws)

	session := s.getOrCreateSession(sessionID, conn)
	log.Printf("ws connected: session=%s (reconnect=%v)", session.ID, sessionID != "")

	// Start read/write pumps
	go conn.writePump()

	// Read pump blocks until connection closes
	conn.readPump(func(msg *Message) {
		// Handle ping/pong at protocol level
		if msg.Type == TypePing {
			pong := &Message{Type: TypePong, ID: msg.ID}
			session.Send(pong)
			return
		}

		if s.handler != nil {
			s.handler(session, msg)
		}
	})

	// Connection closed
	session.Disconnect()
	log.Printf("ws disconnected: session=%s", session.ID)
}

func (s *Server) getOrCreateSession(sessionID string, conn *Connection) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Try to reconnect to existing session
	if sessionID != "" {
		if session, ok := s.sessions[sessionID]; ok {
			session.Reconnect(conn)
			return session
		}
	}

	// Create new session
	if sessionID == "" {
		sessionID = generateSessionID()
	}

	session := newSession(sessionID, conn)
	s.sessions[sessionID] = session
	return session
}

// GetSession returns a session by ID.
func (s *Server) GetSession(id string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[id]
	return session, ok
}

// SessionCount returns the number of active sessions.
func (s *Server) SessionCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

func (s *Server) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		for id, session := range s.sessions {
			if !session.IsConnected() && time.Since(session.LastSeen) > sessionGracePeriod {
				delete(s.sessions, id)
				log.Printf("ws session expired: %s", id)
			}
		}
		s.mu.Unlock()
	}
}

func generateSessionID() string {
	return uuid.New().String()
}
