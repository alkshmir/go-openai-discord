package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	openai "github.com/sashabaranov/go-openai"
)

var gpt = chatBot{}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	gpt.Init()

	botToken := os.Getenv("DISCORD_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("DISCORD_BOT_TOKEN not found in .env file")
	}
	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + botToken)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(messageCreate)
	// In this example, we only care about receiving message events.
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}
	defer dg.Close()

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

}

type chatBot struct {
	client openai.Client
	req    openai.ChatCompletionRequest
}

func (bot *chatBot) Init() error {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY not found in .env file")
	}
	bot.client = *openai.NewClient(apiKey)
	bot.req = openai.ChatCompletionRequest{
		Model: openai.GPT4,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "you are a helpful chatbot",
			},
		},
	}
	return nil
}

func (bot *chatBot) Reply(message string) (string, error) {
	bot.req.Messages = append(bot.req.Messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: message,
	})
	resp, err := bot.client.CreateChatCompletion(context.Background(), bot.req)
	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return "", err
	}
	fmt.Printf("%s\n\n", resp.Choices[0].Message.Content)
	bot.req.Messages = append(bot.req.Messages, resp.Choices[0].Message)
	return resp.Choices[0].Message.Content, nil
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the authenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

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

	reply, err := gpt.Reply(content)
	if err != nil {
		log.Fatal(err)
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
