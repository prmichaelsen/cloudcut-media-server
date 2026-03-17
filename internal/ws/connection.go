package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"nhooyr.io/websocket"
)

const (
	pingInterval = 30 * time.Second
	pongTimeout  = 10 * time.Second
	writeTimeout = 10 * time.Second
)

// Connection wraps a WebSocket connection with read/write helpers.
type Connection struct {
	ws     *websocket.Conn
	sendCh chan *Message
	ctx    context.Context
	cancel context.CancelFunc
}

func newConnection(ctx context.Context, ws *websocket.Conn) *Connection {
	ctx, cancel := context.WithCancel(ctx)
	return &Connection{
		ws:     ws,
		sendCh: make(chan *Message, 64),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Send queues a message for writing.
func (c *Connection) Send(msg *Message) error {
	select {
	case c.sendCh <- msg:
		return nil
	case <-c.ctx.Done():
		return fmt.Errorf("connection closed")
	default:
		return fmt.Errorf("send buffer full")
	}
}

// readPump reads messages from the WebSocket and passes them to the handler.
func (c *Connection) readPump(handler func(*Message)) {
	defer c.cancel()

	for {
		_, data, err := c.ws.Read(c.ctx)
		if err != nil {
			if c.ctx.Err() == nil {
				log.Printf("ws read error: %v", err)
			}
			return
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("ws unmarshal error: %v", err)
			continue
		}

		handler(&msg)
	}
}

// writePump writes queued messages and sends pings.
func (c *Connection) writePump() {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.cancel()
	}()

	for {
		select {
		case msg := <-c.sendCh:
			data, err := json.Marshal(msg)
			if err != nil {
				log.Printf("ws marshal error: %v", err)
				continue
			}

			ctx, cancel := context.WithTimeout(c.ctx, writeTimeout)
			err = c.ws.Write(ctx, websocket.MessageText, data)
			cancel()
			if err != nil {
				log.Printf("ws write error: %v", err)
				return
			}

		case <-ticker.C:
			ctx, cancel := context.WithTimeout(c.ctx, pongTimeout)
			err := c.ws.Ping(ctx)
			cancel()
			if err != nil {
				log.Printf("ws ping failed: %v", err)
				return
			}

		case <-c.ctx.Done():
			return
		}
	}
}

// Close closes the underlying WebSocket connection.
func (c *Connection) Close() {
	c.cancel()
	c.ws.Close(websocket.StatusNormalClosure, "closing")
}

// Done returns a channel that's closed when the connection is done.
func (c *Connection) Done() <-chan struct{} {
	return c.ctx.Done()
}
