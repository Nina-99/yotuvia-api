package utils

import (
	"io"
	"mime/multipart"
	"os"
)

func SaveUploadedFile(file *multipart.FileHeader, dst string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

// RemoveFile deletes a stored file best-effort (used to clean orphaned
// uploads when a subsequent DB write fails).
func RemoveFile(path string) error {
	return os.Remove(path)
}
