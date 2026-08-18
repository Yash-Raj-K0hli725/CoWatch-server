package views

// UploadURLResponse is returned when a presigned upload URL is (re)issued
// for a room. ObjectKey is echoed back for observability only -- clients
// never choose or send it back to us; the server always looks up the
// room's expected key itself before trusting an upload.
type UploadURLResponse struct {
	UploadURL string `json:"upload_url"`
	ObjectKey string `json:"object_key"`
}

// UploadCompleteRequest is sent by the client after its presigned PUT to
// the bucket finishes, asking the server to verify + enqueue transcoding.
type UploadCompleteRequest struct {
	RoomID string `json:"room_id"`
}

// UploadCompleteResponse acknowledges a job was accepted for processing.
type UploadCompleteResponse struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
}

// StorageEventPayload is the normalized "object created" event contract our
// storage webhook expects. Native S3 (EventBridge/SNS) and Cloudflare R2
// (Event Notifications -> Queue) event shapes differ, so a thin relay is
// expected to translate the native event into this shape and HMAC-sign the
// body with the shared webhook secret before forwarding it to us.
type StorageEventPayload struct {
	EventType string `json:"event_type,omitempty"` // "ObjectCreated"; other types are ignored
	Bucket    string `json:"bucket"`
	ObjectKey string `json:"object_key"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
	ETag      string `json:"etag,omitempty"`
}

// VideoStatusMessage is pushed over the room's websocket whenever the
// video's processing status changes (PROCESSING/READY/FAILED).
type VideoStatusMessage struct {
	Type           string   `json:"type"` // "VIDEO_STATUS"
	Status         string   `json:"status"`
	MasterPlaylist string   `json:"master_playlist_url,omitempty"`
	Qualities      []string `json:"qualities,omitempty"`
	Error          string   `json:"error,omitempty"`
}
