package main

import (
	"context"
	"encoding/json"
	"errors"
	"image/jpeg"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	math2img "ekyu.moe/tgbot/math2img"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

var (
	token   = os.Getenv("TGBOT_TOKEN")
	baseURL = os.Getenv("TGBOT_BASE_URL")
	bind    = os.Getenv("TGBOT_BIND")
	timeout = 10 * time.Second

	bot    *tgbotapi.BotAPI
	tmpDir string
	stdout = log.New(os.Stdout, "", log.Lshortfile|log.Lmicroseconds)
	stderr = log.New(os.Stderr, "", log.Lshortfile|log.Lmicroseconds)
)

func main() {
	os.Exit(realMain())
}

func realMain() int {
	var err error
	bot, err = tgbotapi.NewBotAPI(token)
	if err != nil {
		stderr.Println("auth:", err)
		return 1
	}
	stdout.Println("Logged in as", bot.Self.UserName)

	webhook := tgbotapi.NewWebhook(baseURL + "/")
	if _, err := bot.SetWebhook(webhook); err != nil {
		stderr.Println("register webhook:", err)
		return 1
	}

	info, err := bot.GetWebhookInfo()
	if err != nil {
		stderr.Println("get webhook info:", err)
		return 1
	}
	if info.LastErrorDate != 0 {
		stderr.Println("last error:", info.LastErrorMessage)
	}

	tmpDir, err = ioutil.TempDir("", "math2img")
	if err != nil {
		stderr.Println("create temp dir:", err)
		return 1
	}
	defer os.RemoveAll(tmpDir)

	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
		<-c
		os.RemoveAll(tmpDir)
		os.Exit(0)
	}()

	fs := http.FileServer(http.Dir(tmpDir))
	http.Handle("/pop/", http.StripPrefix("/pop", fs))
	// we perfer not to use bot.ListenForWebhook here, since we need some primitive
	// features like context.
	// updates := bot.ListenForWebhook("/" + token)
	http.HandleFunc("/", index)

	stdout.Println("Listening on", bind)
	if err := http.ListenAndServe(bind, nil); err != nil {
		stderr.Println(err)
		return 1
	}

	return 0
}

func index(w http.ResponseWriter, r *http.Request) {
	// fetch update
	update := &tgbotapi.Update{}
	if err := json.NewDecoder(r.Body).Decode(update); err != nil {
		http.Error(w, http.StatusText(400), 400)
		return
	}

	req := update.InlineQuery
	if req == nil || req.Query == "" {
		http.Error(w, http.StatusText(404), 404)
		return
	}

	// begin here
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	// render
	img, err := math2img.Render(ctx, req.Query)
	if err != nil {
		stderr.Println("render:", err)
		exposeError(ctx, req, err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
	width, height := img.Bounds().Dx(), img.Bounds().Dy()

	// prepare temp file
	fn := filepath.Join(tmpDir, req.ID+".jpg")
	f, err := os.Create(fn)
	if err != nil {
		stderr.Println("create temp file:", err)
		exposeError(ctx, req, errors.New("Internal server error"))
		http.Error(w, http.StatusText(500), 500)
		return
	}

	// clean up setup
	go func() {
		<-ctx.Done()
		f.Close()
	}()
	time.AfterFunc(90*time.Second, func() {
		os.Remove(fn)
	})

	// save jpeg to the temp file
	if err := jpeg.Encode(f, img, nil); err != nil {
		stderr.Println("encode jpeg:", err)
		exposeError(ctx, req, errors.New("Internal server error"))
		http.Error(w, http.StatusText(500), 500)
		return
	}

	// clean up
	cancel()

	// end with an empty response
	w.WriteHeader(200)

	// send
	targetURL := baseURL + "/pop/" + url.PathEscape(req.ID+".jpg")
	res := tgbotapi.NewInlineQueryResultPhotoWithThumb(req.ID, targetURL, targetURL+"?t=1")
	res.Width = width
	res.Height = height
	ans := tgbotapi.InlineConfig{
		InlineQueryID: req.ID,
		Results:       []interface{}{res},
	}
	if _, err := bot.AnswerInlineQuery(ans); err != nil {
		stderr.Println(err)
	}
}

func exposeError(ctx context.Context, req *tgbotapi.InlineQuery, err error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		err = ctxErr
	}

	msg := err.Error()
	switch err {
	case context.DeadlineExceeded:
		msg = "Render timeout exceeded (10s)"
	case context.Canceled:
		msg = "Render job has been canceled"
	}

	res := tgbotapi.NewInlineQueryResultArticle(req.ID, "Error", req.Query)
	res.Description = msg
	ans := tgbotapi.InlineConfig{
		InlineQueryID: req.ID,
		Results:       []interface{}{res},
	}
	bot.AnswerInlineQuery(ans)
}
