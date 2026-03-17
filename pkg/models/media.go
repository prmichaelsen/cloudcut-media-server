package models

import "time"

type MediaStatus string

const (
	MediaStatusUploading  MediaStatus = "uploading"
	MediaStatusProcessing MediaStatus = "processing"
	MediaStatusReady      MediaStatus = "ready"
	MediaStatusError      MediaStatus = "error"
)

type Media struct {
	ID               string      `json:"id"`
	OriginalFilename string      `json:"originalFilename"`
	ContentType      string      `json:"contentType"`
	Size             int64       `json:"size"`
	GCSSourcePath    string      `json:"gcsSourcePath"`
	GCSProxyPath     string      `json:"gcsProxyPath,omitempty"`
	Status           MediaStatus `json:"status"`
	Error            string      `json:"error,omitempty"`
	CreatedAt        time.Time   `json:"createdAt"`
}
