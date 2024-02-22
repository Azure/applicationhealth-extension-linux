package utils

import (
	"os"
	"path/filepath"
)

// Get Current Process ID
func GetCurrentProcessID() int {
	return os.Getpid()
}

func GetProcessDirectory() (string, error) {
	p, err := filepath.Abs(os.Args[0])
	if err != nil {
		return "", err
	}

	return filepath.Dir(p), nil
}
