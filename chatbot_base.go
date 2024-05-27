package main

import (
	"errors"
	"log"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
	openai "github.com/sashabaranov/go-openai"
)

type IchatBot interface {
	Init() error
	Reply(prompt string) (string, error)
	HandleReply(s *discordgo.Session, m *discordgo.MessageCreate)
}

// contract for logging
type Logger interface {
	Println(v ...any)
	Fatal(v ...any)
}

type DefaultLogger struct{}

func (l *DefaultLogger) Println(v ...any) {
	log.Println(v...)
}

func (l *DefaultLogger) Fatal(v ...any) {
	log.Fatal(v...)
}

// Sender interface to sniff messages at testing
type Sender interface {
	ChannelSend(s *discordgo.Session, channelID string, content string) (*discordgo.Message, error)
	ReplySend(s *discordgo.Session, channelID string, content string, reference *discordgo.MessageReference) (*discordgo.Message, error)
}

type DefaultSender struct {
	logger Logger
}

func (ds *DefaultSender) ChannelSend(s *discordgo.Session, channelID string, content string) (*discordgo.Message, error) {
	ds.logger.Println("Sending message:", content)
	return s.ChannelMessageSend(channelID, content)
}

func (ds *DefaultSender) ReplySend(s *discordgo.Session, channelID string, content string, reference *discordgo.MessageReference) (*discordgo.Message, error) {
	ds.logger.Println("Sending reply to:", content)
	return s.ChannelMessageSendReply(channelID, content, reference)
}

// Base implementation of HandleReply
type BaseChatBot struct {
	ReplyFunc func(string) (string, error)
	InitFunc  func() error
	logger    Logger
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

	if !isTalkingToBot(s, m) {
		// ignoring message
		return
	}

	content := removeMention(m.Content)

	if bot.ReplyFunc == nil {
		panic("ReplyFunc is not initialized. To generate a reply, specify ReplyFunc and InitFunc in Init().")
	}

	reply, err := bot.ReplyFunc(content)
	if err != nil {
		// opnai API error handling
		e := &openai.APIError{}
		if errors.As(err, &e) {
			switch e.HTTPStatusCode {
			case 400:
				if strings.Contains(e.Message, "Please reduce the length of the messages.") {
					// Initialize the client and clear the message history
					bot.InitFunc()
					bot.sender.ChannelSend(s, m.ChannelID, "Cleared the message history as reached maximum token length. Please retry.")
				}
			case 401:
				// invalid auth or key (do not retry)
				bot.logger.Fatal("Invalid auth or key")
			case 429:
				// rate limiting or engine overload (wait and retry)
				bot.logger.Println(err)
				bot.sender.ChannelSend(s, m.ChannelID, e.Message)
			case 500:
				// openai server error (retry)
				bot.logger.Println(err)
				bot.sender.ChannelSend(s, m.ChannelID, e.Message)
			default:
				// unhandled
				bot.sender.ChannelSend(s, m.ChannelID, e.Message)
				bot.logger.Fatal(err)
			}
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

func isTalkingToBot(s *discordgo.Session, m *discordgo.MessageCreate) bool {
	// Check if the message includes a mention to the bot
	// Note that when the message is a reply, m.Mentions contains the user of the reference message.
	for _, mention := range m.Mentions {
		if mention.ID == s.State.User.ID {
			return true
		}
	}

	// Check if this is a DM channel
	if channel, _ := s.Channel(m.ChannelID); channel.Type == discordgo.ChannelTypeDM {
		return true
	}

	return false
}

func removeMention(m string) string {
	rep := regexp.MustCompile(`<@\d+>`)
	return rep.ReplaceAllString(m, "")
}
