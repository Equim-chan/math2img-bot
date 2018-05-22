package main

import (
	"context"
	"errors"
	"image"
	"image/jpeg"
	"os"
	"time"
)

func dumpJpeg(ctx context.Context, img image.Image, tmpFilename string) error {
	f, err := os.Create(tmpFilename)
	if err != nil {
		return errors.New("create temp file: " + err.Error())
	}

	// clean up setup
	go func() {
		<-ctx.Done()
		f.Close()
	}()
	time.AfterFunc(90*time.Second, func() {
		os.Remove(tmpFilename)
	})

	// save jpeg to the temp file
	if err := jpeg.Encode(f, img, nil); err != nil {
		return errors.New("encode jpeg: " + err.Error())
	}

	return nil
}

func wrapError(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		err = ctxErr
	}

	switch err {
	case context.DeadlineExceeded:
		err = errors.New("Render timeout exceeded (10s)")
	case context.Canceled:
		err = errors.New("Render job has been canceled")
	}

	return err
}
