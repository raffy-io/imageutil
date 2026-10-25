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
	"os"
	"path/filepath"
	"strings"

	webp "github.com/mayahiro/go-webp"
	_ "github.com/vegidio/avif-go" // Registers AVIF decoder
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // Registers WebP decoder
)

const (
	maxImageWidth    = 1920
	maxAllowedWidth  = 4000
	maxAllowedHeight = 4000
)

// SaveCompressedImage takes any io.Reader, resizes if too large, compresses to WebP, and saves to disk
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

// ValidateImage checks file size, allowed extensions, structural integrity, and maximum dimensions, 
// then resets the file read pointer for downstream use.
func ValidateImage(fileHeader *multipart.FileHeader, fieldName string, maxBytes int64) (multipart.File, error) {
	if fileHeader == nil {
		return nil, fmt.Errorf("%s image is required", fieldName)
	}

	if fileHeader.Size > maxBytes {
		return nil, fmt.Errorf("image size exceeds the maximum allowed limit")
	}

	// Validate extension to block dangerous non-image uploads instantly
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" && ext != ".avif" {
		return nil, errors.New("invalid image format. Only JPEG, PNG, WebP, and AVIF are allowed")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, errors.New("unable to read uploaded file")
	}

	// Structurally verify it's a valid image and check dimensions via registered decoders
	config, _, err := image.DecodeConfig(file)
	if err != nil {
		file.Close()
		return nil, errors.New("invalid or corrupted image file structure")
	}

	if config.Width > maxAllowedWidth || config.Height > maxAllowedHeight {
		file.Close()
		return nil, fmt.Errorf("image dimensions are too large (%dx%dpx). Maximum allowed is %dx%dpx", 
			config.Width, config.Height, maxAllowedWidth, maxAllowedHeight)
	}

	// Reset pointer so downstream compression/saving can read from byte 0
	if seeker, ok := file.(io.Seeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			file.Close()
			return nil, errors.New("unable to process uploaded file")
		}
	}

	return file, nil
}