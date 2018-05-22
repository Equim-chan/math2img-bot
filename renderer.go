package math2img

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"log"
	"os/exec"
	"strings"
)

var (
	tex2svg, rsvg string
)

func init() {
	var err error

	tex2svg, err = exec.LookPath("tex2svg")
	if err != nil {
		log.Fatal("tex2svg not found")
	}
	log.Println("tex2svg found at", tex2svg)

	rsvg, err = exec.LookPath("rsvg-convert")
	if err != nil {
		rsvg, err = exec.LookPath("rsvg")
		if err != nil {
			log.Fatal("rsvg (rsvg-convert) not found")
		}
	}
	log.Println("rsvg found at", rsvg)
}

func Render(ctx context.Context, formula string) (image.Image, error) {
	// prepare tex2svg
	t := exec.CommandContext(ctx, tex2svg, formula)
	tStderr := new(bytes.Buffer)
	t.Stderr = tStderr
	tOut, err := t.StdoutPipe()
	if err != nil {
		return nil, errors.New("create stdout pipe on tex2svg: " + err.Error())
	}

	// prepare rsvg
	r := exec.CommandContext(ctx, rsvg,
		"--format", "png",
		"--zoom", "3.0",
		"--background-color", "white",
		"/dev/stdin",
	)
	r.Stdin = tOut
	rStderr := new(bytes.Buffer)
	r.Stderr = rStderr
	rOut, err := r.StdoutPipe()
	if err != nil {
		return nil, errors.New("create stdout pipe on rsvg: " + err.Error())
	}

	// run
	if err := t.Start(); err != nil {
		return nil, errors.New("start tex2svg: " + err.Error())
	}
	defer t.Process.Kill()
	if err := r.Start(); err != nil {
		return nil, errors.New("start rsvg: " + err.Error())
	}
	defer r.Process.Kill()

	// decode png
	img, pngErr := png.Decode(rOut)

	// wait
	rErr := r.Wait()
	tErr := t.Wait()
	ctxErr := ctx.Err()

	switch {
	// possible timeout
	case ctxErr != nil:
		return nil, ctxErr

	// because tex2svg doesn't really exit with non-zero value when an error occurs
	case tStderr.Len() > 0:
		return nil, errors.New(strings.TrimSpace(tStderr.String()))
	case tErr != nil:
		return nil, errors.New("wait tex2svg: " + tErr.Error())

	// just in case
	case rStderr.Len() > 0:
		return nil, errors.New(strings.TrimSpace(rStderr.String()))
	case rErr != nil:
		return nil, errors.New("wait rsvg: " + rErr.Error())

	case pngErr != nil:
		return nil, errors.New("decode png: " + pngErr.Error())
	}

	return img, nil
}
