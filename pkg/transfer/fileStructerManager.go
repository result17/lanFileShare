package transfer

import (
    "sync"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

type FileStructureManager struct {
    RootNodes []*fileInfo.FileNode
    fileMap   map[string]*fileInfo.FileNode
    dirMap    map[string]*fileInfo.FileNode
    mu sync.RWMutex
}

func NewFileStructureManager () *FileStructureManager  {
    return &FileStructureManager {
        RootNodes: make([]*fileInfo.FileNode, 0),
        fileMap:   make(map[string]*fileInfo.FileNode),
        dirMap:    make(map[string]*fileInfo.FileNode),
    }
}

func (fsm *FileStructureManager) AddFileNode(node *fileInfo.FileNode) {
    fsm.mu.Lock()
    defer fsm.mu.Unlock()

    fsm.addFileNodeUnsafe(node)
}

func (fsm *FileStructureManager) addFileNodeUnsafe(node *fileInfo.FileNode) {
    if node.IsDir {
        fsm.dirMap[node.Path] = node
		for _, child := range node.Children {
            fsm.addFileNodeUnsafe(&child)
        }
    } else {
		fsm.fileMap[node.Path] = node
	}
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
