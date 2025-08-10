package transfer

import (
	"context"
	"fmt"
	"sync"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
	"golang.org/x/sync/semaphore"
)

const (
    // MaxSupportedFiles defines the maximum number of files that can be managed
    // This prevents potential integer overflow and memory issues
    MaxSupportedFiles = 1000000
)

type FileTransferManager struct {
	chunkers  map[string]*Chunker
	structure *FileStructureManager
	mu   sync.RWMutex
}

func NewFileTransferManager() *FileTransferManager {
	return &FileTransferManager{
		chunkers: make(map[string]*Chunker),
		structure: NewFileStructureManager(),
	}
}

func (ftm *FileTransferManager) AddFileNode(node *fileInfo.FileNode) error {
	if node == nil {
		return fmt.Errorf("node cannot be nil")
	}
	if len(ftm.chunkers) >= MaxSupportedFiles {
        return fmt.Errorf("file limit exceeded: maximum %d files supported", MaxSupportedFiles)
    }
	if node.IsDir {
		return ftm.processDirConcurrent(node)
	}
	return ftm.addSingleFile(node)
}

func (ftm *FileTransferManager) addSingleFile(node *fileInfo.FileNode) error {
	chunker, err := NewChunkerFromFileNode(node, int32(DefaultChunkSize))
	if err != nil {
		return err
	}
	ftm.mu.Lock()
	if oldChunker, exists := ftm.chunkers[node.Path]; exists {
		oldChunker.Close()
	}
	ftm.chunkers[node.Path] = chunker
	ftm.mu.Unlock()
	return nil
}

func (ftm *FileTransferManager) processDirConcurrent(node *fileInfo.FileNode) error {
	const maxConcurrent = 5
	semaphore := semaphore.NewWeighted(maxConcurrent)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	errChan := make(chan error, len(node.Children))

	for i := range node.Children {
		wg.Add(1)

		go func(child *fileInfo.FileNode) {
			defer wg.Done()
			if err := semaphore.Acquire(ctx, 1); err != nil {
				if err != context.Canceled {
					errChan <- err
				}
				return
			}
			defer semaphore.Release(1)

			select {
				case <- ctx.Done():
					return
				default:
			}

			if err := ftm.AddFileNode(child); err != nil {
				errChan <- err
				cancel()
				return
			}
		}(&node.Children[i])

	}
	wg.Wait()
	close(errChan)
	
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

func (ftm *FileTransferManager) GetChunker(filePath string) (*Chunker, bool) {
    ftm.mu.RLock()
    chunker, exists := ftm.chunkers[filePath]
    ftm.mu.RUnlock()
    return chunker, exists
}

func (ftm *FileTransferManager) Close() error {
    ftm.mu.Lock()
    defer ftm.mu.Unlock()
    
    for _, chunker := range ftm.chunkers {
        chunker.Close()
    }
    ftm.chunkers = make(map[string]*Chunker)
    return nil
}
