package imageutil

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"

	webp "github.com/mayahiro/go-webp"
)

// SaveCompressedImage takes any io.Reader, compresses it, and saves it to disk
func SaveCompressedImage(r io.Reader, outputPath string, quality int) error {
	img, _, err := image.Decode(r)
	if err != nil {
		return err
	}

	out, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer out.Close()

	options := &webp.Options{
		Compression: webp.CompressionLossy,
		Quality:     quality,
	}

	return webp.Encode(out, img, options)
}

// CompressImage takes any io.Reader and returns WebP bytes
func CompressImage(r io.Reader, quality int) ([]byte, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	options := &webp.Options{
		Compression: webp.CompressionLossy,
		Quality:     quality,
	}

	if err := webp.Encode(&buf, img, options); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// ValidateImage checks if the uploaded file is valid and within the size limit.
func ValidateImage(fileHeader *multipart.FileHeader, fieldName string, maxBytes int64) error {
	if fileHeader == nil {
		return fmt.Errorf("%s image is required", fieldName)
	}

	if fileHeader.Size > maxBytes {
		return fmt.Errorf("image size exceeds the maximum allowed limit")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return errors.New("unable to read uploaded file")
	}
	defer file.Close()

	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		return errors.New("unable to verify file type")
	}

	contentType := http.DetectContentType(buffer)
	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/webp" {
		return errors.New("invalid image format. Only JPEG, PNG, and WebP are allowed")
	}

	return nil
}