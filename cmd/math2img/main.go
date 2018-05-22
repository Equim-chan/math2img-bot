package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	// standalone mode
	msg := update.Message
	if msg != nil && msg.IsCommand() {
		// an empty OK response
		w.WriteHeader(200)
		standalone(msg)
		return
	}

	// inline mode
	req := update.InlineQuery
	if req == nil || req.Query == "" {
		http.Error(w, http.StatusText(400), 400)
		return
	}

	inline(w, r, req)
}
