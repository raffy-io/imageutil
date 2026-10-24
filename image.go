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

// ValidateImage checks file size, magic bytes, and structural integrity, 
// then resets the file read pointer for downstream use.
func ValidateImage(fileHeader *multipart.FileHeader, fieldName string, maxBytes int64) (multipart.File, error) {
	if fileHeader == nil {
		return nil, fmt.Errorf("%s image is required", fieldName)
	}

	if fileHeader.Size > maxBytes {
		return nil, fmt.Errorf("image size exceeds the maximum allowed limit")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, errors.New("unable to read uploaded file")
	}

	// Note: We don't defer file.Close() here because we want to return the 
	// open file back to the handler so it can be saved or compressed next!

	// 1. Check magic bytes (content type)
	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		file.Close()
		return nil, errors.New("unable to verify file type")
	}

	contentType := http.DetectContentType(buffer)
	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/webp" {
		file.Close()
		return nil, errors.New("invalid image format. Only JPEG, PNG, and WebP are allowed")
	}

	// 2. Reset pointer to start so DecodeConfig can read the whole header
	if seeker, ok := file.(io.Seeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			file.Close()
			return nil, errors.New("unable to process uploaded file")
		}
	}

	// 3. Structurally verify it's a valid image (catches corrupted files & spoofed extensions)
	_, _, err = image.DecodeConfig(file)
	if err != nil {
		file.Close()
		return nil, errors.New("invalid or corrupted image file structure")
	}

	// 4. Reset pointer AGAIN so downstream compression/saving can read from byte 0
	if seeker, ok := file.(io.Seeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			file.Close()
			return nil, errors.New("unable to process uploaded file")
		}
	}

	return file, nil
}