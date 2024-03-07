package utils

import (
	"os"
)

// Get Current Process ID
func GetCurrentProcessID() int {
	return os.Getpid()
}
