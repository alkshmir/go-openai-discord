package main

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Discord clears the typing indicator after ~10s, so refresh it slightly earlier.
const defaultTypingInterval = 8 * time.Second

// Responder handles generating replies and managing conversation state per channel.
type Responder interface {
	Reply(ctx context.Context, channelID, prompt string) (string, error)
	Reset(ctx context.Context, channelID string) error
}

// Sender abstracts Discord message sending for testability.
type Sender interface {
	ChannelSend(s *discordgo.Session, channelID string, content string) (*discordgo.Message, error)
	ReplySend(s *discordgo.Session, channelID string, content string, reference *discordgo.MessageReference) (*discordgo.Message, error)
	Typing(ctx context.Context, s *discordgo.Session, channelID string) error
}

type DefaultSender struct {
	logger *slog.Logger
}

func (ds *DefaultSender) ChannelSend(s *discordgo.Session, channelID string, content string) (*discordgo.Message, error) {
	ds.logger.Info("sending message", "channel_id", channelID, "content", content)
	return s.ChannelMessageSend(channelID, content)
}

func (ds *DefaultSender) ReplySend(s *discordgo.Session, channelID string, content string, reference *discordgo.MessageReference) (*discordgo.Message, error) {
	ds.logger.Info("sending reply", "channel_id", channelID, "content", content)
	return s.ChannelMessageSendReply(channelID, content, reference)
}

func (ds *DefaultSender) Typing(ctx context.Context, s *discordgo.Session, channelID string) error {
	return s.ChannelTyping(channelID, discordgo.WithContext(ctx))
}

type BotOption func(*Bot)

func WithLogger(l *slog.Logger) BotOption {
	return func(b *Bot) {
		b.logger = l
	}
}

func WithSender(s Sender) BotOption {
	return func(b *Bot) {
		b.sender = s
	}
}

type Bot struct {
	responder Responder
	logger    *slog.Logger
	sender    Sender
	// Interval to re-send Discord's typing indicator while waiting for a reply
	// (defaults to defaultTypingInterval; overridable in tests).
	typingInterval time.Duration
}

func NewBot(responder Responder, opts ...BotOption) *Bot {
	b := &Bot{
		responder:      responder,
		logger:         slog.Default(),
		typingInterval: defaultTypingInterval,
	}
	for _, opt := range opts {
		opt(b)
	}
	if b.sender == nil {
		b.sender = &DefaultSender{logger: b.logger}
	}
	return b
}

// HandleMessage is called every time a new message is created on any channel
// that the authenticated bot has access to.
func (bot *Bot) HandleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself.
	if m.Author.ID == s.State.User.ID {
		return
	}

	talking, err := isTalkingToBot(s, m)
	if err != nil {
		bot.logger.Error("failed to check if talking to bot", "channel_id", m.ChannelID, "error", err)
		return
	}
	if !talking {
		return
	}

	content := removeMention(m.Content)

	ctx := context.Background()
	stopTyping := bot.startTyping(ctx, s, m.ChannelID)
	defer stopTyping()

	reply, err := bot.responder.Reply(ctx, m.ChannelID, content)
	// Stop the typing loop before sending any reply or error message;
	// relying only on defer could let a late typing request fire after
	// ChannelSend and leave the typing indicator stuck on Discord.
	stopTyping()
	if err != nil {
		bot.handleReplyError(s, m.ChannelID, err)
		return
	}

	// Split the content so it's less than 2000 characters.
	for _, chunk := range splitMessage(reply, 2000) {
		if _, err := bot.sender.ChannelSend(s, m.ChannelID, chunk); err != nil {
			bot.logger.Error("failed to send message", "channel_id", m.ChannelID, "error", err)
		}
	}
}

func (bot *Bot) startTyping(ctx context.Context, s *discordgo.Session, channelID string) func() {
	interval := bot.typingInterval
	if interval <= 0 {
		interval = defaultTypingInterval
	}

	typingCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	go func() {
		defer close(done)
		if err := bot.sender.Typing(typingCtx, s, channelID); err != nil {
			if !errors.Is(err, context.Canceled) {
				bot.logger.Warn("failed to send typing indicator", "channel_id", channelID, "error", err)
			}
			return
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-typingCtx.Done():
				return
			case <-ticker.C:
				if err := bot.sender.Typing(typingCtx, s, channelID); err != nil {
					if !errors.Is(err, context.Canceled) {
						bot.logger.Warn("failed to refresh typing indicator", "channel_id", channelID, "error", err)
					}
					return
				}
			}
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			cancel()
			<-done
		})
	}
}

func (bot *Bot) handleReplyError(s *discordgo.Session, channelID string, err error) {
	var chatErr *ChatError
	if errors.As(err, &chatErr) {
		switch {
		case errors.Is(err, ErrContextTooLong):
			bot.logger.Warn("cleared message history due to max token length", "channel_id", channelID, "error", err)
		case errors.Is(err, ErrUnauthorized):
			bot.logger.Error("invalid AI provider credentials", "error", err)
		case errors.Is(err, ErrRateLimited):
			bot.logger.Warn("AI provider rate limited or overloaded", "channel_id", channelID, "error", err)
		case errors.Is(err, ErrProviderError):
			bot.logger.Error("AI provider server error", "channel_id", channelID, "error", err)
		default:
			bot.logger.Error("AI provider error", "channel_id", channelID, "error", err)
		}

		if chatErr.UserMessage != "" {
			if _, sendErr := bot.sender.ChannelSend(s, channelID, chatErr.UserMessage); sendErr != nil {
				bot.logger.Error("failed to send error message", "channel_id", channelID, "error", sendErr)
			}
		}
		return
	}

	bot.logger.Error("failed to generate reply", "channel_id", channelID, "error", err)
}

func (bot *Bot) HandleForget(s *discordgo.Session, i *discordgo.InteractionCreate) {
	content := "Deleted chat context of this channel."
	if err := bot.responder.Reset(context.Background(), i.ChannelID); err != nil {
		bot.logger.Error("failed to reset chat context", "channel_id", i.ChannelID, "error", err)
		content = "Failed to delete chat context of this channel."
	}
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
		},
	})
	if err != nil {
		bot.logger.Error("failed to respond to forget command", "channel_id", i.ChannelID, "error", err)
	}
}

func isTalkingToBot(s *discordgo.Session, m *discordgo.MessageCreate) (bool, error) {
	// Check if the message includes a mention to the bot.
	// Note that when the message is a reply, m.Mentions contains the user of the reference message.
	for _, mention := range m.Mentions {
		if mention.ID == s.State.User.ID {
			return true, nil
		}
	}

	// Check if this is a DM channel.
	channel, err := s.Channel(m.ChannelID)
	if err != nil {
		return false, err
	}
	if channel.Type == discordgo.ChannelTypeDM {
		return true, nil
	}

	return false, nil
}

func removeMention(m string) string {
	rep := regexp.MustCompile(`<@\d+>`)
	return rep.ReplaceAllString(m, "")
}
