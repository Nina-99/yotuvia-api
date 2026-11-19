package controller

import (
	"errors"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// maxReceiptBytes is the upload cap for a payment receipt or QR image.
const maxReceiptBytes = 5 * 1024 * 1024

var errReceiptTooLarge = errors.New("file too large: maximum 5MB")

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
		writeBadRequest(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"url": url})
}

// saveReceipt persists an uploaded image and returns its public path.
//
// The stored extension comes from the SNIFFED bytes, never from the client
// filename or the Content-Type header. Trusting either one lets a caller store
// `receipt.html` (declaring `Content-Type: image/jpeg`) and have the reverse
// proxy serve it as text/html from the application origin — stored XSS with
// direct access to the web app's localStorage tokens.
func (c *UploadController) saveReceipt(file *multipart.FileHeader) (string, error) {
	if file.Size > maxReceiptBytes {
		return "", errReceiptTooLarge
	}

	src, err := file.Open()
	if err != nil {
		return "", errors.New("failed to read file")
	}
	_, ext, sniffErr := utils.SniffImage(src)
	_ = src.Close()
	if sniffErr != nil {
		return "", errors.New("invalid file type: only real JPEG, PNG, WebP or GIF images are allowed")
	}

	filename := uuid.New().String() + ext
	dst := filepath.Join(c.uploadDir, filename)

	// Reopen: the sniff consumed the first bytes of the reader above.
	if err := c.saveFile(file, dst); err != nil {
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
