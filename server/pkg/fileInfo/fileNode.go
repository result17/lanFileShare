package fileInfo

import (
	"log"
	"os"
	"path/filepath"

	"github.com/gabriel-vasile/mimetype"
)

type FileNode struct {
	Name        string      `json:"name"`
	IsDir       bool        `json:"is_dir"`
	SizeInBytes int64       `json:"size_in_bytes"`
	MimeType    string      `json:"mite_type,omitempty"`
	Checksum    string      `json:"checksum,omitempty"`
	Children    []*FileNode `json:"children,omitempty"`
	Path        string      `json:"-"`
}

func CreateNode(path string) (*FileNode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	node := &FileNode{
		Name:        info.Name(),
		IsDir:       info.IsDir(),
		SizeInBytes: info.Size(),
		Path: 		 path,
	}
	if node.IsDir {
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}

		node.Children = make([]*FileNode, 0)

		for _, entry := range entries {
			childPath := filepath.Join(path, entry.Name())
			childNode, err := CreateNode(childPath)
			if err != nil {
				log.Printf("Skipping %s: %v", childPath, err)
				continue
			}
			node.Children = append(node.Children, childNode)
			node.SizeInBytes += childNode.SizeInBytes
		}
	} else {
		mime, err := mimetype.DetectFile(path)
		if err != nil {
			node.MimeType = "application/octet-stream"
		}
		node.MimeType = mime.String()
	}
	checksum, err := node.CalcChecksum()
	if err != nil {
		return nil, err
	}
	node.Checksum = checksum
	return node, nil
}
