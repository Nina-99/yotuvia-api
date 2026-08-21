package controller

import (
	"errors"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UploadController struct {
	uploadDir string
}

func NewUploadController() *UploadController {
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads/receipts"
	}

	// Ensure directory exists
	os.MkdirAll(uploadDir, 0755)

	return &UploadController{
		uploadDir: uploadDir,
	}
}

func (c *UploadController) UploadReceipt(ctx *gin.Context) {
	file, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	url, err := c.saveReceipt(file)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"url": url})
}

func (c *UploadController) saveReceipt(file *multipart.FileHeader) (string, error) {
	// Validate file type
	contentType := file.Header.Get("Content-Type")
	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
	}
	if !allowedTypes[contentType] {
		return "", errors.New("invalid file type: only JPEG and PNG are allowed")
	}

	// Validate file size (5MB)
	const maxSize = 5 * 1024 * 1024
	if file.Size > maxSize {
		return "", errors.New("file too large: maximum 5MB")
	}

	// Generate unique filename
	ext := filepath.Ext(file.Filename)
	if ext == "" {
		switch contentType {
		case "image/jpeg":
			ext = ".jpg"
		case "image/png":
			ext = ".png"
		}
	}
	filename := uuid.New().String() + ext
	filepath := filepath.Join(c.uploadDir, filename)

	// Save file
	if err := c.saveFile(file, filepath); err != nil {
		return "", errors.New("failed to save file")
	}

	// Return URL path (served statically via /uploads)
	return "/uploads/receipts/" + filename, nil
}

func (c *UploadController) saveFile(file *multipart.FileHeader, dst string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = dstFile.ReadFrom(src)
	return err
}

func (c *UploadController) UploadReceiptFromFile(file *multipart.FileHeader) (string, error) {
	return c.saveReceipt(file)
}