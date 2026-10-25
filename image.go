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
	"golang.org/x/image/draw"
)

const maxImageWidth = 1920

// SaveCompressedImage takes any io.Reader, resizes if too large, compresses, and saves to disk
func SaveCompressedImage(r io.Reader, outputPath string, quality int) error {
	img, _, err := image.Decode(r)
	if err != nil {
		return err
	}

	// Resize to prevent massive CPU bottlenecks on large smartphone photos
	img = resizeIfNeeded(img, maxImageWidth)

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

// CompressImage takes any io.Reader, resizes if too large, and returns WebP bytes
func CompressImage(r io.Reader, quality int) ([]byte, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}

	// Resize to prevent massive CPU bottlenecks on large smartphone photos
	img = resizeIfNeeded(img, maxImageWidth)

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

// resizeIfNeeded downscales the image proportionally if it exceeds maxWidth
func resizeIfNeeded(src image.Image, maxWidth int) image.Image {
	bounds := src.Bounds()
	origWidth := bounds.Dx()

	// If the image is already smaller than maxWidth, leave it alone
	if origWidth <= maxWidth {
		return src
	}

	origHeight := bounds.Dy()
	targetWidth := maxWidth
	targetHeight := (origHeight * targetWidth) / origWidth

	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	draw.BiLinear.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)

	return dst
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