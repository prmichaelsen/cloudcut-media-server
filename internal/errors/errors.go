package errors

import "fmt"

// AppError represents a structured application error.
type AppError struct {
	Code      string                 `json:"code"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Retryable bool                   `json:"retryable"`
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Common error codes
const (
	ErrUploadTooLarge        = "UPLOAD_TOO_LARGE"
	ErrInvalidContentType    = "INVALID_CONTENT_TYPE"
	ErrInvalidUpload         = "INVALID_UPLOAD"
	ErrUploadFailed          = "UPLOAD_FAILED"
	ErrNotFound              = "NOT_FOUND"
	ErrProxyNotReady         = "PROXY_NOT_READY"
	ErrSignFailed            = "SIGN_FAILED"
	ErrRenderFailed          = "RENDER_FAILED"
	ErrJobNotFound           = "JOB_NOT_FOUND"
	ErrGCSTransient          = "GCS_TRANSIENT"
	ErrFFmpegFailed          = "FFMPEG_FAILED"
	ErrEDLValidationFailed   = "EDL_VALIDATION_FAILED"
	ErrJobQueueFull          = "JOB_QUEUE_FULL"
	ErrRateLimitExceeded     = "RATE_LIMIT_EXCEEDED"
)

// New creates a new AppError.
func New(code, message string, retryable bool) *AppError {
	return &AppError{
		Code:      code,
		Message:   message,
		Retryable: retryable,
	}
}

// NewWithDetails creates a new AppError with additional details.
func NewWithDetails(code, message string, retryable bool, details map[string]interface{}) *AppError {
	return &AppError{
		Code:      code,
		Message:   message,
		Details:   details,
		Retryable: retryable,
	}
}

// Wrap wraps an existing error with a code.
func Wrap(code string, err error, retryable bool) *AppError {
	return &AppError{
		Code:      code,
		Message:   err.Error(),
		Retryable: retryable,
	}
}
