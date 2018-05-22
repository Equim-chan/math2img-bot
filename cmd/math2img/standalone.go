package main

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"

	"ekyu.moe/tgbot/math2img"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

const usage = `/render <formula> - Render a formula in TeX format, without $. Example: ` + "`/render E = mc^2`" + `
/help - Print this message

The bot works in inline mode too.

Author: @Equim
Source: https://github.com/Equim-chan/math2img-bot
`

func sendStandalone(rep tgbotapi.Chattable) {
	if _, err := bot.Send(rep); err != nil {
		stderr.Println("reply:", err)
	}
}

func exposeErrorStandalone(err error, msg *tgbotapi.Message) {
	content := usage
	if err != nil {
		content = "Error:\n```\n" + err.Error() + "\n```"
	}

	rep := tgbotapi.NewMessage(msg.Chat.ID, content)
	rep.ParseMode = tgbotapi.ModeMarkdown
	if msg.Chat.IsGroup() || msg.Chat.IsSuperGroup() {
		rep.ReplyToMessageID = msg.MessageID
	}
	sendStandalone(rep)
}

func standalone(msg *tgbotapi.Message) {
	cmd := msg.Command()
	if cmd != "render" {
		exposeErrorStandalone(nil, msg)
		return
	}

	formula := strings.TrimSpace(msg.CommandArguments())
	if formula == "" {
		exposeErrorStandalone(nil, msg)
		return
	}

	// this is different from the inline one
	// we don't rely on this incoming http request's lifetime
	// so we just use the background context
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	img, err := math2img.Render(ctx, formula)
	if err != nil {
		cerr := wrapError(ctx, err)
		if cerr != err {
			stderr.Println("render:", err)
		}
		exposeErrorStandalone(cerr, msg)
		return
	}

	tmp := filepath.Join(tmpDir, strconv.Itoa(msg.MessageID)+".jpg")
	if err := dumpJpeg(ctx, img, tmp); err != nil {
		cerr := wrapError(ctx, errors.New("Internal server error"))
		if cerr != err {
			stderr.Println("dump jpeg:", err)
		}
		exposeErrorStandalone(cerr, msg)
		return
	}

	photo := tgbotapi.NewPhotoUpload(msg.Chat.ID, tmp)
	if msg.Chat.IsGroup() || msg.Chat.IsSuperGroup() {
		photo.ReplyToMessageID = msg.MessageID
	}
	sendStandalone(photo)
}
