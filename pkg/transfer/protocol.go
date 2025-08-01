package transfer

type MessageType string

const (
	ChunkData MessageType = "chunk_data"
	FileComplete MessageType = "file_complete"
	TransferCancel   MessageType = "transfer_cancel"
	TransferComplete MessageType = "transfer_complete"
	ProgressUpdate MessageType = "progress_update"
)


type ChunkMessage struct {
    Type         MessageType
    Session      TransferSession
    FileID       string
    FileName     string
    SequenceNo   uint32
    Data         []byte
    ChunkHash    string
    TotalSize    int64
    ExpectedHash string
    ErrorMessage string
}


type MessageSerializer interface {
    Marshal(message *ChunkMessage) ([]byte, error)
    Unmarshal(data []byte) (*ChunkMessage, error)
	Name() string
	IsBinary() bool
}
