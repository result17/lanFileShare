package components

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/gabriel-vasile/mimetype"
	"github.com/rescp17/lanFileSharer/internal/style"
	"github.com/rescp17/lanFileSharer/internal/util"
)

const (
	DATE_FORMAT_STR = "2006-01-02 15:04:05"
	DIR_SIZE_STR = "<DIR>"
)

type NodeDisplayItem struct {
	Name       string
	Path       string
	IsDir      bool
	ModTime    string
	Size       string
	Type       string
	RenderType string
	Extension  string
	fs.DirEntry
}

func GetNodeDisplayItemFromDirEntity(entry os.DirEntry, absPath string) NodeDisplayItem {
	info, err := entry.Info()
	modTime := ""
	size := ""
	typeStr := ""

	if err == nil {
		modTime = info.ModTime().Format(DATE_FORMAT_STR)
		if info.IsDir() {
			size = DATE_FORMAT_STR
		} else {
			size = util.FormatSize(info.Size())
		}
	}

	// Get MIME type for files
	if !entry.IsDir() {
		entryPath := filepath.Join(absPath, entry.Name())
		mime, err := mimetype.DetectFile(entryPath)
		if err == nil {
			typeStr = mime.String()
		}
	}

	item := NodeDisplayItem{
		Name:     entry.Name(),
		Path:     filepath.Join(absPath, entry.Name()),
		IsDir:    entry.IsDir(),
		ModTime:  modTime,
		Size:     size,
		Type:     typeStr,
		DirEntry: entry,
	}

	var renderSize string
	if item.IsDir {
		renderStyle := GetColorForFileType(item)
		renderSize = fmt.Sprintf("%s %s", GetIconForItem(item), style.RenderWithSafeReset(renderStyle, SimplifyMIME(item.Type, filepath.Ext(item.Name))))
	} else {
		renderSize = GetIconForItem(item)
	}

	item.RenderType = renderSize

	return item
}

// GetIconForItem returns the appropriate emoji icon based on MIME type or fallback to extension
func GetIconForItem(item NodeDisplayItem) string {
	if item.IsDir {
		return "📁"
	}

	mime := strings.ToLower(strings.TrimSpace(item.Type))

	// MIME type-based icons
	if mime != "" {
		switch {
		case strings.HasPrefix(mime, "text/"):
			return "📄"
		case strings.HasPrefix(mime, "image/"):
			return "🖼️"
		case strings.HasPrefix(mime, "audio/"):
			return "🎵"
		case strings.HasPrefix(mime, "video/"):
			return "🎥"
		case mime == "application/pdf":
			return "📕"
		case strings.HasPrefix(mime, "application/zip") ||
			strings.Contains(mime, "tar") || strings.Contains(mime, "gzip"):
			return "📦"
		case strings.Contains(mime, "word"):
			return "📄"
		case strings.Contains(mime, "excel"), mime == "text/csv":
			return "📊"
		case strings.Contains(mime, "powerpoint"):
			return "📈"
		case strings.Contains(mime, "json"):
			return "🔧"
		case strings.Contains(mime, "yaml"), strings.Contains(mime, "yml"):
			return "⚙️"
		case strings.Contains(mime, "xml"):
			return "🏗️"
		case strings.Contains(mime, "shellscript"):
			return "🐚"
		case strings.Contains(mime, "x-executable"), strings.Contains(mime, "x-msdownload"):
			return "⚙️"
		}
	}

	// fallback: guess by extension if MIME type is missing
	fileType := strings.ToLower(filepath.Ext(item.Name))
	switch fileType {
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
	default:
		return "📄"
	}
}

// GetColorForFileType returns a lipgloss style with bright colors suitable for both dark and light TUI themes
func GetColorForFileType(item NodeDisplayItem) lipgloss.Style {
	mime := strings.ToLower(strings.TrimSpace(item.Type))

	if mime != "" {
		switch {
		case strings.HasPrefix(mime, "text/"):
			return lipgloss.NewStyle().Foreground(lipgloss.Color("33")) // Bright Blue
		case strings.HasPrefix(mime, "image/"):
			return lipgloss.NewStyle().Foreground(lipgloss.Color("213")) // Bright Pink/Purple
		case strings.HasPrefix(mime, "video/"):
			return lipgloss.NewStyle().Foreground(lipgloss.Color("203")) // Bright Red
		case strings.HasPrefix(mime, "audio/"):
			return lipgloss.NewStyle().Foreground(lipgloss.Color("208")) // Bright Orange
		case mime == "application/pdf":
			return lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Bright Red
		case strings.HasPrefix(mime, "application/json"),
			strings.HasSuffix(mime, "+json"),
			strings.HasPrefix(mime, "application/xml"),
			strings.HasPrefix(mime, "text/xml"),
			strings.HasSuffix(mime, "+xml"):
			return lipgloss.NewStyle().Foreground(lipgloss.Color("135")) // Bright Purple
		case strings.HasPrefix(mime, "application/zip"),
			strings.Contains(mime, "tar"),
			strings.Contains(mime, "gzip"):
			return lipgloss.NewStyle().Foreground(lipgloss.Color("220")) // Bright Yellow
		case strings.Contains(mime, "word"):
			return lipgloss.NewStyle().Foreground(lipgloss.Color("33")) // Bright Blue
		case strings.Contains(mime, "excel"), mime == "text/csv":
			return lipgloss.NewStyle().Foreground(lipgloss.Color("46")) // Bright Green
		case strings.Contains(mime, "powerpoint"):
			return lipgloss.NewStyle().Foreground(lipgloss.Color("219")) // Bright Pink
		case strings.Contains(mime, "shellscript"):
			return lipgloss.NewStyle().Foreground(lipgloss.Color("82")) // Bright Green
		case strings.Contains(mime, "x-executable"), strings.Contains(mime, "x-msdownload"):
			return lipgloss.NewStyle().Foreground(lipgloss.Color("82")) // Bright Green
		default:
			return lipgloss.NewStyle().Foreground(lipgloss.Color("250")) // Neutral Gray
		}
	}

	// fallback for unknown types
	return lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
}

// SimplifyMIME converts a full base media type (like "image/png") to a short user-friendly label.
// If input is empty, it tries to infer from filename extension via extFallback.
func SimplifyMIME(base string, extFallback string) string {
	if base == "" {
		// fallback by extension
		ext := strings.ToLower(extFallback)
		switch ext {
		case ".png":
			return "PNG"
		case ".jpg", ".jpeg":
			return "JPEG"
		case ".mp4":
			return "MP4"
		case ".pdf":
			return "PDF"
		case ".json":
			return "JSON"
		case ".html", ".htm":
			return "HTML"
		case ".css":
			return "CSS"
		case ".go":
			return "Go"
		case ".py":
			return "Python"
		default:
			return "File"
		}
	}

	// common explicit mapping (exact matches)
	var common = map[string]string{
		"text/plain":                                                                 "Text",
		"text/html":                                                                  "HTML",
		"application/json":                                                           "JSON",
		"application/pdf":                                                            "PDF",
		"application/zip":                                                            "ZIP",
		"application/x-7z-compressed":                                                "7z",
		"application/x-rar-compressed":                                               "RAR",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document":    "Word",
		"application/msword":                                                          "Word",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":          "Excel",
		"application/vnd.ms-excel":                                                   "Excel",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation":  "PowerPoint",
		"application/vnd.ms-powerpoint":                                              "PowerPoint",
		"application/octet-stream":                                                   "Binary",
		"image/png":                                                                   "PNG",
		"image/jpeg":                                                                  "JPEG",
		"image/gif":                                                                   "GIF",
		"image/webp":                                                                  "WEBP",
		"audio/mpeg":                                                                  "MP3",
		"video/mp4":                                                                   "MP4",
	}

	if label, ok := common[strings.ToLower(base)]; ok {
		return label
	}

	// fallback by major type
	if strings.HasPrefix(base, "image/") {
		return "Image"
	}
	if strings.HasPrefix(base, "video/") {
		return "Video"
	}
	if strings.HasPrefix(base, "audio/") {
		return "Audio"
	}
	if strings.HasPrefix(base, "text/") {
		// show subtype if meaningful (e.g. text/css)
		parts := strings.SplitN(base, "/", 2)
		if len(parts) == 2 && parts[1] != "" {
			return strings.ToUpper(parts[1]) // e.g. "CSS", "CSV"
		}
		return "Text"
	}
	if strings.HasPrefix(base, "application/") {
		// try to extract a shorter suffix after common prefixes
		s := strings.TrimPrefix(base, "application/")
		if strings.HasPrefix(s, "vnd.") {
			// vendor-specific: try to look for "word", "excel", etc.
			if strings.Contains(s, "word") {
				return "Word"
			}
			if strings.Contains(s, "excel") {
				return "Excel"
			}
			if strings.Contains(s, "powerpoint") {
				return "PowerPoint"
			}
		}
		// general fallback
		return "App"
	}
	return "File"
}
