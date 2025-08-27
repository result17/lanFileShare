package fileInfo

import (
	"log"
	"os"
	"path/filepath"

	"github.com/gabriel-vasile/mimetype"
)

type FileNode struct {
	Name     string     `json:"name"`
	IsDir    bool       `json:"is_dir"`
	Size     int64      `json:"size"`
	MimeType string     `json:"mime_type,omitempty"`
	Checksum string     `json:"checksum,omitempty"`
	Children []FileNode `json:"children,omitempty"`
	Path     string     `json:"-"`
}

func CreateNode(path string) (FileNode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FileNode{}, err
	}
	node := FileNode{
		Name:  info.Name(),
		IsDir: info.IsDir(),
		Size:  info.Size(),
		Path:  path,
	}
	if node.IsDir {
		entries, err := os.ReadDir(path)
		if err != nil {
			return FileNode{}, err
		}

		node.Children = make([]FileNode, 0)

		for _, entry := range entries {
			childPath := filepath.Join(path, entry.Name())
			childNode, err := CreateNode(childPath)
			if err != nil {
				log.Printf("Skipping %s: %v", childPath, err)
				continue
			}
			node.Children = append(node.Children, childNode)
			node.Size += childNode.Size
		}
	} else {
		mime, err := mimetype.DetectFile(path)
		if err != nil {
			node.MimeType = "application/octet-stream"
		} else {
			node.MimeType = mime.String()
		}
	}
	checksum, err := node.CalcChecksum()
	if err != nil {
		return FileNode{}, err
	}
	node.Checksum = checksum
	return node, nil
}
