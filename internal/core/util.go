package core

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"

	"github.com/mobazha/mobazha3.0/pkg/models"
)

// NormalizeCurrencyCode standardizes the format for the given currency code.
func normalizeCurrencyCode(currencyCode string) string {
	var c, err = models.CurrencyDefinitions.Lookup(currencyCode)
	if err != nil {
		log.Errorf("invalid currency code (%s): %s", currencyCode, err.Error())
		return ""
	}
	return c.String()
}

// maybeCloseDone is a helper to close the done chan if it's not nil.
func maybeCloseDone(done chan<- struct{}) {
	if done != nil {
		close(done)
	}
}

// maxSize: max size in bytes. Dont uses compression if maxSize <= 0.
func imageToJpeg(buf *bytes.Buffer, in image.Image, maxSize int) error {
	quality := 100

	if err := jpeg.Encode(buf, in, &jpeg.Options{Quality: quality}); err != nil {
		return err
	}
	if maxSize < 1 {
		return nil
	}

	size := buf.Len()
	for size > maxSize && quality > 0 {

		quality -= 10
		if err := jpeg.Encode(buf, in, &jpeg.Options{Quality: quality}); err != nil {
			return err
		}

		size = buf.Len()
	}

	if quality <= 0 {
		return errors.New("can't resize image (image size is too large)")
	}

	return nil
}
