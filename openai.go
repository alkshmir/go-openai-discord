package main

import (
	"context"
	"errors"
	"strings"
	"sync"

	openai "github.com/sashabaranov/go-openai"
)

var _ Responder = (*OpenAIClient)(nil)

type OpenAIClient struct {
	client      *openai.Client
	mu          sync.Mutex
	chatContext map[string]openai.ChatCompletionRequest
}

func NewOpenAIClient(apiKey string) (*OpenAIClient, error) {
	if apiKey == "" {
		return nil, errors.New("OPENAI_API_KEY not found in .env file or environment variable")
	}
	return &OpenAIClient{
		client:      openai.NewClient(apiKey),
		chatContext: make(map[string]openai.ChatCompletionRequest),
	}, nil
}

func (c *OpenAIClient) Reply(ctx context.Context, channelID, prompt string) (string, error) {
	c.mu.Lock()
	req, exists := c.chatContext[channelID]
	if !exists {
		req = c.newContext()
	}
	messages := make([]openai.ChatCompletionMessage, len(req.Messages)+1)
	copy(messages, req.Messages)
	messages[len(req.Messages)] = openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: prompt,
	}
	req.Messages = messages
	c.mu.Unlock()

	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", c.classifyError(ctx, channelID, err)
	}
	if len(resp.Choices) == 0 {
		return "", &ChatError{
			Kind: ErrProviderError,
			Err:  errors.New("openai returned empty choices"),
		}
	}

	assistantMsg := resp.Choices[0].Message

	c.mu.Lock()
	updated, exists := c.chatContext[channelID]
	if !exists {
		updated = c.newContext()
	}
	updated.Messages = append(updated.Messages,
		openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: prompt,
		},
		assistantMsg,
	)
	c.chatContext[channelID] = updated
	c.mu.Unlock()

	return assistantMsg.Content, nil
}

func (c *OpenAIClient) Reset(_ context.Context, channelID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.chatContext, channelID)
	return nil
}

func (c *OpenAIClient) classifyError(ctx context.Context, channelID string, err error) error {
	var apiErr *openai.APIError
	if !errors.As(err, &apiErr) {
		return err
	}

	switch apiErr.HTTPStatusCode {
	case 400:
		if strings.Contains(apiErr.Message, "Please reduce the length of the messages.") {
			if resetErr := c.Reset(ctx, channelID); resetErr != nil {
				return errors.Join(err, resetErr)
			}
			return &ChatError{
				Kind:        ErrContextTooLong,
				UserMessage: "Cleared the message history as reached maximum token length. Please retry.",
				Err:         err,
			}
		}
		return &ChatError{Err: err}
	case 401:
		return &ChatError{
			Kind: ErrUnauthorized,
			Err:  err,
		}
	case 429:
		return &ChatError{
			Kind:        ErrRateLimited,
			UserMessage: apiErr.Message,
			Err:         err,
		}
	case 500:
		return &ChatError{
			Kind:        ErrProviderError,
			UserMessage: apiErr.Message,
			Err:         err,
		}
	default:
		return &ChatError{
			UserMessage: apiErr.Message,
			Err:         err,
		}
	}
}

func (c *OpenAIClient) newContext() openai.ChatCompletionRequest {
	return openai.ChatCompletionRequest{
		Model: "gpt-5.2",
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "you are a helpful chatbot",
			},
		},
	}
}
