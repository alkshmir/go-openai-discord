package main

import (
	"context"
	"fmt"
	"log"
	"os"

	openai "github.com/sashabaranov/go-openai"
)

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
		Model: openai.GPT4Turbo,
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
