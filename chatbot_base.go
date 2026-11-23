package main

import (
	"errors"
	"log/slog"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
	openai "github.com/sashabaranov/go-openai"
)

type IchatBot interface {
	Init() error
	Reply(prompt string, s *discordgo.Session, m *discordgo.MessageCreate) (string, error)
	HandleReply(s *discordgo.Session, m *discordgo.MessageCreate)
	RemoveContext(s *discordgo.Session, i *discordgo.InteractionCreate)
}

// Sender interface to sniff messages at testing
type Sender interface {
	ChannelSend(s *discordgo.Session, channelID string, content string) (*discordgo.Message, error)
	ReplySend(s *discordgo.Session, channelID string, content string, reference *discordgo.MessageReference) (*discordgo.Message, error)
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

// Base implementation of HandleReply
type BaseChatBot struct {
	ReplyFunc func(string, *discordgo.Session, *discordgo.MessageCreate) (string, error)
	InitFunc  func() error
	logger    *slog.Logger
	sender    Sender
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the authenticated bot has access to.
func (bot *BaseChatBot) HandleReply(s *discordgo.Session, m *discordgo.MessageCreate) {

	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}

	t, err := isTalkingToBot(s, m)
	if err != nil {
		bot.logger.Error("failed to check if talking to bot", "channel_id", m.ChannelID, "error", err)
		return
	}
	if !t {
		// ignoring message
		return
	}

	content := removeMention(m.Content)

	if bot.ReplyFunc == nil {
		panic("ReplyFunc is not initialized. To generate a reply, specify ReplyFunc and InitFunc in Init().")
	}

	reply, err := bot.ReplyFunc(content, s, m)
	if err != nil {
		// opnai API error handling
		e := &openai.APIError{}
		if errors.As(err, &e) {
			switch e.HTTPStatusCode {
			case 400:
				if strings.Contains(e.Message, "Please reduce the length of the messages.") {
					// Initialize the client and clear the message history
					bot.logger.Warn("cleared message history due to max token length", "channel_id", m.ChannelID, "error", err)
					bot.InitFunc()
					bot.sender.ChannelSend(s, m.ChannelID, "Cleared the message history as reached maximum token length. Please retry.")
				} else {
					bot.logger.Error("OpenAI bad request", "channel_id", m.ChannelID, "error", err)
				}
			case 401:
				// invalid auth or key (do not retry)
				bot.logger.Error("invalid OpenAI auth or API key", "error", err)
			case 429:
				// rate limiting or engine overload (wait and retry)
				bot.logger.Warn("OpenAI rate limited or overloaded", "channel_id", m.ChannelID, "error", err)
				bot.sender.ChannelSend(s, m.ChannelID, e.Message)
			case 500:
				// openai server error (retry)
				bot.logger.Error("OpenAI server error", "channel_id", m.ChannelID, "error", err)
				bot.sender.ChannelSend(s, m.ChannelID, e.Message)
			default:
				// unhandled
				bot.logger.Error("unhandled OpenAI API error", "status_code", e.HTTPStatusCode, "channel_id", m.ChannelID, "error", err)
				bot.sender.ChannelSend(s, m.ChannelID, e.Message)
			}
		} else {
			bot.logger.Error("failed to generate reply", "channel_id", m.ChannelID, "error", err)
		}
		// TODO: add discord API error handling
	} else {
		// split the content so it's less than 2000 characters
		replies := splitMessage(reply, 2000)
		for _, r := range replies {
			bot.sender.ChannelSend(s, m.ChannelID, r)
		}
	}

}

func isTalkingToBot(s *discordgo.Session, m *discordgo.MessageCreate) (bool, error) {
	// Check if the message includes a mention to the bot
	// Note that when the message is a reply, m.Mentions contains the user of the reference message.
	for _, mention := range m.Mentions {
		if mention.ID == s.State.User.ID {
			return true, nil
		}
	}

	// Check if this is a DM channel
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
