package multiFilePicker

import (
	"github.com/charmbracelet/lipgloss"
	"io/fs"
	"path/filepath"
	"strings"
)

type NodeDisplayItem struct {
	Name    string
	Path    string
	IsDir   bool
	ModTime string
	Size    string
	Type    string
	fs.DirEntry
}

// getIconForItem returns the appropriate emoji icon for different file types
func GetIconForItem(item NodeDisplayItem) string {
	if item.IsDir {
		return "📁"
	}

	// Get file extension (convert to lowercase for case-insensitive matching)
	fileType := strings.ToLower(filepath.Ext(item.Name))

	// Return appropriate icon based on file type
	switch fileType {
	case ".txt", ".md", ".log":
		return "📄"
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg", ".webp":
		return "🖼️"
	case ".mp4", ".avi", ".mkv", ".mov", ".wmv":
		return "🎥"
	case ".mp3", ".wav", ".flac", ".ogg", ".m4a":
		return "🎵"
	case ".zip", ".rar", ".7z", ".tar", ".gz", ".bz2":
		return "📦"
	case ".pdf":
		return "📕"
	case ".doc", ".docx":
		return "📄"
	case ".xls", ".xlsx", ".csv":
		return "📊"
	case ".ppt", ".pptx":
		return "📈"
	case ".py":
		return "🐍"
	case ".go":
		return "🐹"
	case ".js", ".ts":
		return "📜"
	case ".html", ".htm":
		return "🌐"
	case ".css":
		return "🎨"
	case ".json":
		return "🔧"
	case ".yml", ".yaml":
		return "⚙️"
	case ".xml":
		return "🏗️"
	case ".sh", ".bash", ".zsh":
		return "🐚"
	case ".exe", ".bat", ".cmd":
		return "⚙️"
	default:
		// For MIME type-based icons
		if item.Type != "" {
			if strings.HasPrefix(item.Type, "text/") {
				return "📄"
			}
			if strings.HasPrefix(item.Type, "image/") {
				return "🖼️"
			}
			if strings.HasPrefix(item.Type, "audio/") {
				return "🎵"
			}
			if strings.HasPrefix(item.Type, "video/") {
				return "🎥"
			}
			if strings.HasPrefix(item.Type, "application/pdf") {
				return "📕"
			}
			// Default file icon for other types
			return "📄"
		}
		return "📄"
	}
}

// getColorForFileType returns appropriate color for different file types
func GetColorForFileType(item NodeDisplayItem) lipgloss.Style {
	fileType := strings.ToLower(filepath.Ext(item.Name))

	switch fileType {
	case ".txt", ".md", ".log":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("39")) // Blue
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg", ".webp":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("133")) // Magenta
	case ".mp4", ".avi", ".mkv", ".mov", ".wmv":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("124")) // Red
	case ".mp3", ".wav", ".flac", ".ogg", ".m4a":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // Orange
	case ".zip", ".rar", ".7z", ".tar", ".gz", ".bz2":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("136")) // Yellow
	case ".pdf":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("160")) // Red (dark)
	case ".py", ".go", ".js", ".ts":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("26")) // Cyan
	case ".html", ".htm":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("34")) // Green
	case ".json", ".xml":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("60")) // Purple
	case ".css":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("199")) // Pink
	case ".exe", ".bat", ".cmd":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("22")) // Dark Green
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("244")) // Gray
	}
}
