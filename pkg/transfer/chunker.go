package transfer

type Chunk struct {
	SequenceNo uint32
	Data       []byte
	Hash       string
	IsLast     bool
	ByteSize   uint32
}
