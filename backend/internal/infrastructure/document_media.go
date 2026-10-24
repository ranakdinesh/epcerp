package infrastructure

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// No SVG/HTML. Decode bounded raster dimensions before accepting the full image.
type DocumentInspector struct{ maximumBytes int }

func NewDocumentInspector(maximumBytes int) DocumentInspector { return DocumentInspector{maximumBytes} }

// ValidationMessage exposes reviewed input-validation text without module dependencies.
type DocumentValidationError struct{ message string }

func (e *DocumentValidationError) Error() string             { return e.message }
func (e *DocumentValidationError) ValidationMessage() string { return e.message }

func (i DocumentInspector) Inspect(data []byte) (string, error) {
	if len(data) == 0 || len(data) > i.maximumBytes {
		message := fmt.Sprintf("File exceeds %d bytes", i.maximumBytes)
		if i.maximumBytes > 0 && i.maximumBytes%(1024*1024) == 0 {
			message = fmt.Sprintf("File exceeds %d MiB", i.maximumBytes/(1024*1024))
		}
		return "", &DocumentValidationError{message}
	}
	if bytes.HasPrefix(data, []byte("%PDF-")) && bytes.Contains(data[max(0, len(data)-2048):], []byte("%%EOF")) {
		return "application/pdf", nil
	}
	config, format, e := image.DecodeConfig(bytes.NewReader(data))
	if e != nil || config.Width < 1 || config.Height < 1 || config.Width > 16000 || config.Height > 16000 || int64(config.Width)*int64(config.Height) > 40000000 {
		return "", &DocumentValidationError{"Only valid PDF, PNG, JPEG or GIF files are allowed"}
	}
	_, _, e = image.Decode(bytes.NewReader(data))
	if e != nil {
		return "", &DocumentValidationError{"Image is corrupt"}
	}
	switch format {
	case "jpeg":
		return "image/jpeg", nil
	case "png":
		return "image/png", nil
	case "gif":
		return "image/gif", nil
	}
	return "", &DocumentValidationError{"Unsupported image format"}
}
