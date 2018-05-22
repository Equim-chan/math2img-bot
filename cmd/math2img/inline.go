package main

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"path/filepath"

	"ekyu.moe/tgbot/math2img"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func sendInline(rep interface{}, queryID string) {
	answer := tgbotapi.InlineConfig{
		InlineQueryID: queryID,
		Results:       []interface{}{rep},
	}
	if _, err := bot.AnswerInlineQuery(answer); err != nil {
		stderr.Println("answer inline:", err)
	}
}

func exposeErrorInline(err error, queryID, content string) {
	rep := tgbotapi.NewInlineQueryResultArticle(queryID, "Error", content)
	rep.Description = err.Error()
	sendInline(rep, queryID)
}

func inline(w http.ResponseWriter, r *http.Request, req *tgbotapi.InlineQuery) {
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	// render
	img, err := math2img.Render(ctx, req.Query)
	if err != nil {
		cerr := wrapError(ctx, err)
		http.Error(w, http.StatusText(500), 500)
		if cerr != err {
			stderr.Println("render:", err)
		}
		exposeErrorInline(cerr, req.ID, req.Query)
		return
	}
	width, height := img.Bounds().Dx(), img.Bounds().Dy()

	// prepare temp file
	tmp := filepath.Join(tmpDir, req.ID+".jpg")
	if err := dumpJpeg(ctx, img, tmp); err != nil {
		cerr := wrapError(ctx, errors.New("Internal server error"))
		http.Error(w, http.StatusText(500), 500)
		if cerr != err {
			stderr.Println("dump jpeg:", err)
		}
		exposeErrorInline(cerr, req.ID, req.Query)
		return
	}

	// end with an empty OK response
	w.WriteHeader(200)

	targetURL := baseURL + "/pop/" + url.PathEscape(req.ID+".jpg")
	rep := tgbotapi.NewInlineQueryResultPhotoWithThumb(req.ID, targetURL, targetURL+"?t=1")
	rep.Width = width
	rep.Height = height
	sendInline(rep, req.ID)
}
