package transfer

import (
	"fmt"
	"sync"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

type FileStructureManager struct {
	RootNodes []*fileInfo.FileNode
	fileMap   map[string]*fileInfo.FileNode
	dirMap    map[string]*fileInfo.FileNode
	mu        sync.RWMutex
}

func NewFileStructureManager() *FileStructureManager {
	return &FileStructureManager{
		RootNodes: make([]*fileInfo.FileNode, 0),
		fileMap:   make(map[string]*fileInfo.FileNode),
		dirMap:    make(map[string]*fileInfo.FileNode),
	}
}

func NewFileStructureManagerWithRootNodes(rootNodes []*fileInfo.FileNode) (*FileStructureManager, error) {
	fsm := NewFileStructureManager()
	fsm.RootNodes = rootNodes
	for _, node := range rootNodes {
		err := fsm.addFileNodeUnsafe(node)
		if err != nil {
			return nil, err
		}
	}
	return fsm, nil
}

// NewFileStructureManagerFromPath creates a FileStructureManager from a single path
func NewFileStructureManagerFromPath(path string) (*FileStructureManager, error) {
	fsm := NewFileStructureManager()

	node, err := fileInfo.CreateNode(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create node from path %s: %w", path, err)
	}

	// AddFileNode now handles both internal maps and RootNodes
	err = fsm.AddFileNode(&node)
	if err != nil {
		return nil, err
	}

	return fsm, nil
}

func (fsm *FileStructureManager) AddPath(path string) error {
	fsm.mu.Lock()
	defer fsm.mu.Unlock()

	node, err := fileInfo.CreateNode(path)
	if err != nil {
		return fmt.Errorf("failed to create node from path %s: %w", path, err)
	}

	// Add to internal maps
	err = fsm.addFileNodeUnsafe(&node)
	if err != nil {
		return err
	}

	// Add to RootNodes to maintain consistency
	fsm.RootNodes = append(fsm.RootNodes, &node)
	return nil
}

func (fsm *FileStructureManager) AddFileNode(node *fileInfo.FileNode) error {
	fsm.mu.Lock()
	defer fsm.mu.Unlock()

	// Add to internal maps
	err := fsm.addFileNodeUnsafe(node)
	if err != nil {
		return err
	}

	// Add to RootNodes to maintain consistency
	fsm.RootNodes = append(fsm.RootNodes, node)
	return nil
}

func (fsm *FileStructureManager) addFileNodeUnsafe(node *fileInfo.FileNode) error {
	if node == nil {
		return fmt.Errorf("node cannot be nil")
	}

	queue := []*fileInfo.FileNode{node}

	for len(queue) > 0 {
		currentNode := queue[0]
		queue = queue[1:]

		if currentNode.IsDir {
			if _, exists := fsm.dirMap[currentNode.Path]; exists {
				continue
			}
			fsm.dirMap[currentNode.Path] = currentNode
			for i := range currentNode.Children {
				queue = append(queue, &currentNode.Children[i])
			}
		} else {
			if _, exists := fsm.fileMap[currentNode.Path]; exists {
				continue
			}
			fsm.fileMap[currentNode.Path] = currentNode
		}
	}
	return nil
}

func (fsm *FileStructureManager) GetFileCount() int {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()

	return len(fsm.fileMap)
}

func (fsm *FileStructureManager) GetDirCount() int {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()

	return len(fsm.dirMap)
}

func (fsm *FileStructureManager) GetFile(path string) (*fileInfo.FileNode, bool) {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()

	node, exists := fsm.fileMap[path]
	return node, exists
}

func (fsm *FileStructureManager) GetDir(path string) (*fileInfo.FileNode, bool) {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()

	node, exists := fsm.dirMap[path]
	return node, exists
}

func (fsm *FileStructureManager) GetAllFiles() []*fileInfo.FileNode {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()

	files := make([]*fileInfo.FileNode, 0, len(fsm.fileMap))
	for _, node := range fsm.fileMap {
		files = append(files, node)
	}
	return files
}

func (fsm *FileStructureManager) GetAllFileEntities() []fileInfo.FileNode {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()

	// Convert []*fileInfo.FileNode to []fileInfo.FileNode
	files := make([]fileInfo.FileNode, 0, len(fsm.fileMap))
	for _, ptr := range fsm.fileMap {
		if ptr != nil {
			files = append(files, *ptr)
		}
	}
	return files
}

func (fsm *FileStructureManager) GetAllDirs() []*fileInfo.FileNode {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()

	dirs := make([]*fileInfo.FileNode, 0, len(fsm.dirMap))
	for _, node := range fsm.dirMap {
		dirs = append(dirs, node)
	}
	return dirs
}

func (fsm *FileStructureManager) GetTotalSize() int64 {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()

	var totalSize int64
	for _, node := range fsm.fileMap {
		totalSize += node.Size
	}
	return totalSize
}

func (fsm *FileStructureManager) Clear() {
	fsm.mu.Lock()
	defer fsm.mu.Unlock()

	fsm.RootNodes = fsm.RootNodes[:0]
	fsm.fileMap = make(map[string]*fileInfo.FileNode)
	fsm.dirMap = make(map[string]*fileInfo.FileNode)
}
