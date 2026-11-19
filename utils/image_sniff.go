package utils

import (
	"errors"
	"io"
	"net/http"
)

// MaxImageSniffBytes mirrors the window http.DetectContentType inspects. Reading
// exactly this many bytes classifies every format the platform accepts.
const MaxImageSniffBytes = 512

// ErrUnsupportedImage signals bytes that are not one of the image formats we
// are willing to store.
var ErrUnsupportedImage = errors.New("unsupported image type")

// ImageExtByMIME maps a SNIFFED MIME type to the extension the stored file MUST
// get. The extension is always derived from the bytes, never from the
// client-supplied filename or the Content-Type header: both are attacker
// controlled, and a stored `.html` or `.svg` served from the application origin
// is stored XSS (the browser renders it as a document and it can read the SPA's
// localStorage tokens).
var ImageExtByMIME = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// SniffImage reads up to MaxImageSniffBytes from r and reports the detected MIME
// type together with the extension the file must be persisted with. It returns
// ErrUnsupportedImage when the bytes are not an accepted image format.
//
// The reader is not consumed beyond the sniff window, but callers that need the
// full body must reopen their source: a multipart.FileHeader can be opened
// again, which is the pattern used by every upload path.
func SniffImage(r io.Reader) (mime string, ext string, err error) {
	head := make([]byte, MaxImageSniffBytes)
	n := 0
	for n < len(head) {
		read, readErr := r.Read(head[n:])
		n += read
		if readErr != nil {
			break
		}
	}

	mime = http.DetectContentType(head[:n])
	ext, ok := ImageExtByMIME[mime]
	if !ok {
		return mime, "", ErrUnsupportedImage
	}
	return mime, ext, nil
}
