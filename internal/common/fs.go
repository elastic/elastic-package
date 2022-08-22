package common

import (
	"os"
	"path"
	"path/filepath"

	"github.com/pkg/errors"
)

func FindRepositoryRootDirectory() (string, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", errors.Wrap(err, "locating working directory failed")
	}

	dir := workDir
	for dir != "." {
		path := filepath.Join(dir, ".git")
		fileInfo, err := os.Stat(path)
		if err == nil && fileInfo.IsDir() {
			return dir, nil
		}

		if dir == "/" {
			break
		}
		dir = filepath.Dir(dir)
	}

	return "", os.ErrNotExist
}

func FindFileRootDirectory(fileName string) (string, error) {
	dir, err := FindRepositoryRootDirectory()
	if err != nil {
		return "", err
	}

	sourceFileName := path.Join(dir, fileName)
	_, err = os.Stat(sourceFileName)
	if err != nil {
		return "", err
	}

	return sourceFileName, nil
}
