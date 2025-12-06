package app

import (
	"context"
	"io"
	"net/http"

	"github.com/kirrryu2k/nemu-manaka/config"
	"github.com/mymmrac/telego"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

type app struct {
	bot *telego.Bot
	cli *http.Client

	logger         *zap.SugaredLogger
	cfg            *config.Config
	availableChats map[int64]config.Chat
}

func New(cfg *config.Config, logger *zap.SugaredLogger) (app, error) {
	bot, err := telego.NewBot(cfg.TelegramBot.Token, telego.WithDefaultDebugLogger())
	if err != nil {
		return app{}, errors.WithMessage(err, "new tg bot api")
	}

	availableChats := make(map[int64]config.Chat, len(cfg.TelegramBot.Chats))
	for _, chat := range cfg.TelegramBot.Chats {
		availableChats[chat.Id] = chat
	}

	return app{
		bot:            bot,
		cli:            http.DefaultClient,
		logger:         logger,
		cfg:            cfg,
		availableChats: availableChats,
	}, nil
}

func (a app) Run(ctx context.Context) error {
	updChan, err := a.bot.UpdatesViaLongPolling(ctx, &telego.GetUpdatesParams{
		Offset:         0,
		Limit:          0,
		Timeout:        int(a.cfg.TelegramBot.UpdateTimeout.Seconds()),
		AllowedUpdates: []string{"message"},
	})
	if err != nil {
		return errors.WithMessage(err, "get updates channel")
	}

	for {
		select {
		case <-ctx.Done():
			return errors.WithMessage(ctx.Err(), "ctx done")
		case upd := <-updChan:
			a.handleUpdate(ctx, upd)
		}
	}
}

func (a app) handleUpdate(ctx context.Context, upd telego.Update) {
	if upd.Message == nil {
		return
	}
	if err := a.handleMessage(ctx, upd.Message); err != nil {
		a.logger.Warn(errors.WithMessage(err, "handle message"))
	}
}

func (a app) handleMessage(ctx context.Context, msg *telego.Message) error {
	if a.isAvailableChat(msg) {
		return a.replyToMessage(ctx, msg.Chat.ID, msg.MessageID)
	}
	a.logger.Debug("chat '%d' is not available; skipping... ", msg.Chat.ID)
	return nil
}

func (a app) isAvailableChat(msg *telego.Message) bool {
	v, ok := a.availableChats[msg.Chat.ID]
	if !ok {
		return false
	}

	if v.OriginId == 0 {
		return true
	}

	origin, ok := msg.ForwardOrigin.(*telego.MessageOriginChannel)
	return ok && origin.Chat.ID == v.OriginId
}

func (a app) replyToMessage(ctx context.Context, chatId int64, msgId int) error {
	if err := a.setReacts(ctx, chatId, msgId); err != nil {
		a.logger.Warn(errors.WithMessage(err, "set reacts"))
	}

	comment, err := a.generateComment(ctx)
	if err != nil {
		return errors.WithMessage(err, "generate comment")
	}

	_, err = a.bot.SendMessage(ctx, &telego.SendMessageParams{
		ChatID: telego.ChatID{ID: chatId},
		Text:   comment,
		ReplyParameters: &telego.ReplyParameters{
			MessageID: msgId,
		},
	})
	if err != nil {
		return errors.WithMessage(err, "send reply message")
	}

	return nil
}

func (a app) setReacts(ctx context.Context, chatId int64, msgId int) error {
	err := a.bot.SetMessageReaction(ctx, new(telego.SetMessageReactionParams).
		WithChatID(telego.ChatID{ID: chatId}).
		WithMessageID(msgId).
		WithReaction(&telego.ReactionTypeCustomEmoji{
			Type:          telego.ReactionCustomEmoji,
			CustomEmojiID: "5326012643952059434",
		}).
		WithReaction(&telego.ReactionTypeEmoji{
			Type:  telego.ReactionEmoji,
			Emoji: "🤪",
		}),
	)
	if err != nil {
		return errors.WithMessage(err, "set message reaction")
	}
	return nil
}

func (a app) generateComment(ctx context.Context) (comment string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.CommentGenerator.Url, nil)
	if err != nil {
		return "", errors.WithMessage(err, "new request with ctx")
	}
	req.SetBasicAuth(a.cfg.CommentGenerator.Username, a.cfg.CommentGenerator.Password)

	resp, err := a.cli.Do(req)
	if err != nil {
		return "", errors.WithMessage(err, "do request")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.WithMessage(err, "read all")
	}
	defer func() {
		if cErr := resp.Body.Close(); cErr != nil && err == nil {
			err = errors.WithMessage(cErr, "close resp body")
		}
	}()

	comment = gjson.GetBytes(body, "comment").String()
	if comment == "" {
		return "", errors.New("unexpected empty comment response")
	}

	return comment, nil
}
