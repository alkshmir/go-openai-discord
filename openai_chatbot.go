package main

import (
	"context"
	"os"

	"github.com/bwmarrin/discordgo"
	openai "github.com/sashabaranov/go-openai"
)

type OpenAIChatBot struct {
	BaseChatBot
	client openai.Client
	req    openai.ChatCompletionRequest
}

// the functional options for OpenAIChatBot
type ChatBotOption func(*OpenAIChatBot)

// functional option to set the logger for OpenAIChatBot
func WithLogger(l Logger) ChatBotOption {
	return func(s *OpenAIChatBot) {
		s.logger = l
	}
}

func NewOpenAIChatBot(opts ...ChatBotOption) (IchatBot, error) {
	cb := &OpenAIChatBot{}
	for _, opt := range opts {
		opt(cb)
	}
	// Default logger configuration
	if cb.logger == nil {
		cb.logger = &DefaultLogger{}
	}
	cb.sender = &DefaultSender{
		logger: cb.logger,
	}
	cb.Init()
	return cb, nil
}

func (bot *OpenAIChatBot) Init() error {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		bot.logger.Fatal("OPENAI_API_KEY not found in .env file or environment variable")
	}
	bot.client = *openai.NewClient(apiKey)
	bot.req = openai.ChatCompletionRequest{
		Model: openai.GPT4o,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "you are a helpful chatbot",
			},
		},
	}
	bot.ReplyFunc = bot.Reply
	bot.InitFunc = bot.Init
	bot.logger.Println("Initialized OpenAI chatbot")
	return nil
}

func (bot *OpenAIChatBot) Reply(prompt string) (string, error) {
	bot.req.Messages = append(bot.req.Messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: prompt,
	})
	resp, err := bot.client.CreateChatCompletion(context.Background(), bot.req)
	if err != nil {
		bot.logger.Println("ChatCompletion error: %v\n", err)
		return "", err
	}
	//fmt.Printf("%s\n\n", resp.Choices[0].Message.Content)
	bot.req.Messages = append(bot.req.Messages, resp.Choices[0].Message)
	return resp.Choices[0].Message.Content, nil
}

func (bot *OpenAIChatBot) FakeReply(prompt string) (string, error) {
	f, _ := os.Open("fake.txt")
	data := make([]byte, 4096)
	count, _ := f.Read(data)
	return string(data[:count]), nil
}

func (bot *OpenAIChatBot) newContext() openai.ChatCompletionRequest {
	return openai.ChatCompletionRequest{
		Model: openai.GPT4o,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "you are a helpful chatbot",
			},
		},
	}
}

func (bot *OpenAIChatBot) RemoveContext(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if _, exists := bot.chatContext[i.ChannelID]; exists {
		delete(bot.chatContext, i.ChannelID)
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Deleted chat context of this channel.",
		},
	})
}
