package main

import (
	"errors"
	"fmt"
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

// Base implementation of HandleReply
type BaseChatBot struct {
	ReplyFunc func(string) (string, error)
	InitFunc  func() error
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
		fmt.Println("Ignoring message")
		return
	}

	content := removeMention(m.Content)

	reply, err := bot.ReplyFunc(content)
	if err != nil {
		e := &openai.APIError{}
		if errors.As(err, &e) {
			switch e.HTTPStatusCode {
			case 400:
				if strings.Contains(e.Message, "Please reduce the length of the messages.") {
					// Initialize the client and clear the message history
					bot.InitFunc()
					s.ChannelMessageSend(m.ChannelID, "Cleared the message history as reached maximum token length. Please retry.")
				}
			case 401:
				// invalid auth or key (do not retry)
				log.Fatal("Invalid auth or key")
			case 429:
				// rate limiting or engine overload (wait and retry)
				log.Println(err)
				s.ChannelMessageSend(m.ChannelID, e.Message)
			case 500:
				// openai server error (retry)
				log.Println(err)
				s.ChannelMessageSend(m.ChannelID, e.Message)
			default:
				// unhandled
				log.Fatal(err)
			}
		}
	}
	s.ChannelMessageSend(m.ChannelID, reply)
}

func isTalkingToBot(s *discordgo.Session, m *discordgo.MessageCreate) bool {
	// Check if the message includes a mention to the bot
	for _, mention := range m.Mentions {
		if mention.ID == s.State.User.ID {
			return true
		}
	}

	// Check if the message is a reply to a bot
	if m.Message.Reference() != nil {
		referenceMessage, _ := s.ChannelMessage(m.Message.ChannelID, m.Message.Reference().MessageID)
		if referenceMessage.Author.ID == s.State.User.ID {
			return true
		}
	}

	if channel, _ := s.Channel(m.ChannelID); channel.Type == discordgo.ChannelTypeDM {
		return true
	}

	return false
}

func removeMention(m string) string {
	rep := regexp.MustCompile(`<@\d+>`)
	return rep.ReplaceAllString(m, "")
}
