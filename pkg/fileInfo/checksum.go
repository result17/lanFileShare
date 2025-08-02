package fileInfo

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"
)

func calculateSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func () {
		if err := file.Close(); err != nil {
			slog.Error("fail to close file", "error", err.Error())
		}
	}()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	sum := hasher.Sum(nil)
	return hex.EncodeToString(sum), nil
}

func (n *FileNode) CalcChecksum() (string, error) {
	if !n.IsDir {
		sum, err := calculateSHA256(n.Path)
		if err != nil {
			return "", err
		}
		n.Checksum = sum
		return sum, nil
	}

	var childSums []string
	sort.Slice(n.Children, func(i, j int) bool {
		return n.Children[i].Name < n.Children[j].Name
	})
	for _, child := range n.Children {
		sum, err := child.CalcChecksum()
		if err != nil {
			return "", err
		}
		childSums = append(childSums, child.Name+":"+sum)
	}
	joined := strings.Join(childSums, "|")
	hash := sha256.Sum256([]byte(joined))
	n.Checksum = hex.EncodeToString(hash[:])
	return n.Checksum, nil
}

func (n *FileNode) VerifySHA256(expectedChecksum string) (bool, error) {
	actual, err := n.CalcChecksum()
	if err != nil {
		return false, err
	}
	return actual == expectedChecksum, nil
}
