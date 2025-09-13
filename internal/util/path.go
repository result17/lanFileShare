package util

import (
	"log/slog"
	"os"
)

// CheckDirectory checks if a directory exists at the given path.
// Returns (exists, isDir, error)
func CheckDirectory(path string) (exists bool, isDir bool, err error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, false, nil
		}
		return false, false, err
	}
	return true, info.IsDir(), nil
}

// GetOrCreateDirByPath returns a valid directory path.
// If the specified path exists and is a directory, it returns that path.
// If the path doesn't exist, it attempts to create it.
// If creation fails or the path exists but is not a directory, it falls back to the current working directory.
// Returns the final path to be used.
func GetOrCreateDirByPath(inputPath string) string {
	exists, isDir, err := CheckDirectory(inputPath)

	var resultPath string
	if err != nil {
		slog.Error("Failed to check output directory", "error", err)
		// Fallback to current working directory
		resultPath, err = os.Getwd()
		if err != nil {
			slog.Error("Failed to get current working directory", "error", err)
			return ""
		}
		slog.Info("Using current working directory as output path", "path", resultPath)
	} else if !exists {
		// Path doesn't exist, attempt to create it
		slog.Info("Output directory does not exist, attempting to create", "path", inputPath)
		err = os.MkdirAll(inputPath, 0755) // 0755 permissions: rwxr-xr-x
		if err != nil {
			slog.Error("Failed to create output directory", "path", inputPath, "error", err)
			// Fallback to current working directory
			resultPath, err = os.Getwd()
			if err != nil {
				slog.Error("Failed to get current working directory", "error", err)
				return ""
			}
			slog.Info("Using current working directory as output path", "path", resultPath)
		} else {
			slog.Info("Successfully created output directory", "path", inputPath)
			resultPath = inputPath
		}
	} else if !isDir {
		// Path exists but is not a directory
		slog.Error("Output path exists but is not a directory", "path", inputPath)
		// Fallback to current working directory
		resultPath, err = os.Getwd()
		if err != nil {
			slog.Error("Failed to get current working directory", "error", err)
			return ""
		}
		slog.Info("Using current working directory as output path", "path", resultPath)
	} else {
		// Path exists and is a directory
		resultPath = inputPath
		slog.Info("Using specified output directory", "path", resultPath)
	}
	return resultPath
}
