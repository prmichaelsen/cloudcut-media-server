package ws

import "encoding/json"

// Message types for the WebSocket protocol.
const (
	TypePing         = "ping"
	TypePong         = "pong"
	TypeEDLSubmit    = "edl.submit"
	TypeEDLAck       = "edl.ack"
	TypeJobProgress  = "job.progress"
	TypeJobComplete  = "job.complete"
	TypeJobError     = "job.error"
	TypeMediaStatus  = "media.status"
)

// Message is the base WebSocket message envelope.
type Message struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// ProgressPayload is sent with job.progress messages.
type ProgressPayload struct {
	JobID   string  `json:"jobId"`
	Percent float64 `json:"percent"`
	FPS     int     `json:"fps,omitempty"`
	Speed   string  `json:"speed,omitempty"`
	ETA     int     `json:"eta,omitempty"`
	Stage   string  `json:"stage"`
}

// CompletePayload is sent with job.complete messages.
type CompletePayload struct {
	JobID string `json:"jobId"`
	URL   string `json:"url"`
}

// ErrorPayload is sent with job.error messages.
type ErrorPayload struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	JobID     string `json:"jobId,omitempty"`
	Retryable bool   `json:"retryable"`
}

// MediaStatusPayload is sent with media.status messages.
type MediaStatusPayload struct {
	MediaID string `json:"mediaId"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
}

// NewMessage creates a Message with a typed payload.
func NewMessage(msgType, id string, payload interface{}) (*Message, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &Message{
		Type:    msgType,
		ID:      id,
		Payload: data,
	}, nil
}
