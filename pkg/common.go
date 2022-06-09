package pkg

import (
	"io/fs"
	"os"
)

func createFile(filePath string, mode fs.FileMode) (*os.File, error) {
	file, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}
	if err := file.Chmod(mode); err != nil {
		return nil, err
	}
	return file, nil
}
