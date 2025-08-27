package util

import (
    "os"
)

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