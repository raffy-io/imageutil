package imageutil

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"path/filepath"
	"strings"

	webp "github.com/mayahiro/go-webp"
	_ "github.com/vegidio/avif-go"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	MaxImageWidth    = 1920
	MaxAllowedWidth  = 15000
	MaxAllowedHeight = 15000
)

// ValidateImage checks file size, extension, structural integrity, and dimensions
// using any io.ReadSeeker, making it reusable across HTTP, CLI, and S3.
func ValidateImage(r io.ReadSeeker, filename string, fileSize int64, maxBytes int64) error {
	if fileSize > maxBytes {
		return errors.New("image size exceeds the maximum allowed limit")
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" && ext != ".avif" {
		return errors.New("invalid image format. Only JPEG, PNG, WebP, and AVIF are allowed")
	}

	config, _, err := image.DecodeConfig(r)
	if err != nil {
		return errors.New("invalid or corrupted image file structure")
	}

	if config.Width > MaxAllowedWidth || config.Height > MaxAllowedHeight {
		return fmt.Errorf("image dimensions are too large (%dx%dpx). Maximum allowed is %dx%dpx",
			config.Width, config.Height, MaxAllowedWidth, MaxAllowedHeight)
	}

	// Reset read pointer so downstream processing can read from byte 0
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return errors.New("unable to process image stream")
	}

	return nil
}

// ProcessImage smartly bypasses re-encoding if the file is already a valid, small-enough WebP
func ProcessImage(r io.ReadSeeker, filename string, quality int) ([]byte, error) {
	ext := strings.ToLower(filepath.Ext(filename))

	if ext == ".webp" {
		config, _, err := image.DecodeConfig(r)
		r.Seek(0, io.SeekStart)

		if err == nil && config.Width <= MaxImageWidth {
			bytesData, readErr := io.ReadAll(r)
			if readErr == nil {
				return bytesData, nil
			}
		}
		r.Seek(0, io.SeekStart)
	}

	return CompressImage(r, quality)
}

// CompressImage takes any io.Reader, resizes if too large, and returns WebP bytes
func CompressImage(r io.Reader, quality int) ([]byte, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}

	img = resizeIfNeeded(img, MaxImageWidth)

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