package transfer

type MessageType string

const (
	TransferStructure MessageType = "transfer_structure"
	ChunkData         MessageType = "chunk_data"
	FileBegin         MessageType = "file_begin"
	FileComplete      MessageType = "file_complete"
	TransferBegin     MessageType = "transfer_begin"
	TransferCancel    MessageType = "transfer_cancel"
	TransferComplete  MessageType = "transfer_complete"
	ProgressUpdate    MessageType = "progress_update"
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
