package transfer

import (
	"encoding/json"
)

type JSONSerializer struct{}

func NewJSONSerializer() *JSONSerializer {
	return &JSONSerializer{}
}

type JSONChunkMessage struct {
	Type         MessageType     `json:"type"`
	Session      TransferSession `json:"session"`
	FileID       string          `json:"file_id"`
	FileName     string          `json:"file_name"`
	SequenceNo   uint32          `json:"sequence_no"`
	Offset       int64           `json:"offset"`
	Data         []byte          `json:"data,omitempty"`
	ChunkHash    string          `json:"chunk_hash,omitempty"`
	TotalSize    int64           `json:"total_size,omitempty"`
	ExpectedHash string          `json:"expected_hash,omitempty"`
	ErrorMessage string          `json:"error_message,omitempty"`
}

func (j *JSONSerializer) Marshal(msg *ChunkMessage) ([]byte, error) {
	return json.Marshal(JSONChunkMessage{
		Type:         msg.Type,
		Session:      msg.Session,
		FileID:       msg.FileID,
		FileName:     msg.FileName,
		SequenceNo:   msg.SequenceNo,
		Offset:       msg.Offset,
		Data:         msg.Data,
		ChunkHash:    msg.ChunkHash,
		TotalSize:    msg.TotalSize,
		ExpectedHash: msg.ExpectedHash,
		ErrorMessage: msg.ErrorMessage,
	})
}

func (j *JSONSerializer) Unmarshal(data []byte) (*ChunkMessage, error) {
	var jsonMsg JSONChunkMessage
	if err := json.Unmarshal(data, &jsonMsg); err != nil {
		return nil, err
	}
	return &ChunkMessage{
		Type:         jsonMsg.Type,
		Session:      jsonMsg.Session,
		FileID:       jsonMsg.FileID,
		FileName:     jsonMsg.FileName,
		SequenceNo:   jsonMsg.SequenceNo,
		Offset:       jsonMsg.Offset,
		Data:         jsonMsg.Data,
		ChunkHash:    jsonMsg.ChunkHash,
		TotalSize:    jsonMsg.TotalSize,
		ExpectedHash: jsonMsg.ExpectedHash,
		ErrorMessage: jsonMsg.ErrorMessage,
	}, nil
}

func (j *JSONSerializer) Name() string {
	return "json"
}

func (j *JSONSerializer) IsBinary() bool {
	return false
}
