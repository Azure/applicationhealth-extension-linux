package manifest

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func copyFileNewDirectory(src, dst, fileName string) error {
	// This function is used to copy the manifest file to a new directory
	// The function is not implemented as it is not relevant to the test case
	if src == "" || dst == "" {
		return fmt.Errorf("invalid source or destination path")
	}
	src, err := filepath.Abs(src)
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(dst), 0755)
	if err != nil {
		return err
	}

	// Open the source file
	srcFile, err := os.Open(filepath.Join(src, fileName))
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Create the destination file
	dstFile, err := os.Create(filepath.Join(dst, fileName))
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// Copy the contents of the source file to the destination file
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	return nil
}
