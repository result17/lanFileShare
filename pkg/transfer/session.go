package transfer

type TransferSession struct {
	// ServiceID identifies the sender application instance
	ServiceID string `json:"service_id"`
	// SessionID identifies this specific transfer session
	SessionID string `json:"session_id"`
	// CreatedAt indicates when the session was created (consider using int64 for Unix timestamp)
	SessionCreateAt int64 `json:"session_create_at"`
}
