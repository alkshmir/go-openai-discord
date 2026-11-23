package main

import (
	"context"
	"errors"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestNewOpenAIClient_EmptyKey(t *testing.T) {
	if _, err := NewOpenAIClient(""); err == nil {
		t.Fatal("expected error for empty API key, got nil")
	}
}

func TestOpenAIClient_ClassifyError(t *testing.T) {
	t.Run("ContextTooLongResetsOnlyTargetChannel", func(t *testing.T) {
		c, err := NewOpenAIClient("test-key")
		if err != nil {
			t.Fatal(err)
		}
		c.chatContext["ch-1"] = c.newContext()
		c.chatContext["ch-2"] = c.newContext()

		apiErr := &openai.APIError{
			HTTPStatusCode: 400,
			Message:        "This model's maximum context length is exceeded. Please reduce the length of the messages.",
		}
		got := c.classifyError(context.Background(), "ch-1", apiErr)

		if !errors.Is(got, ErrContextTooLong) {
			t.Errorf("errors.Is(%v, ErrContextTooLong) = false, want true", got)
		}
		var chatErr *ChatError
		if !errors.As(got, &chatErr) || chatErr.UserMessage == "" {
			t.Errorf("expected non-empty UserMessage in ChatError, got %#v", got)
		}
		if _, exists := c.chatContext["ch-1"]; exists {
			t.Errorf("expected ch-1 context to be cleared")
		}
		if _, exists := c.chatContext["ch-2"]; !exists {
			t.Errorf("expected ch-2 context to be preserved")
		}
	})

	tests := []struct {
		name            string
		err             error
		wantKind        error
		wantUserMessage string
	}{
		{
			name:            "Unauthorized",
			err:             &openai.APIError{HTTPStatusCode: 401, Message: "bad key"},
			wantKind:        ErrUnauthorized,
			wantUserMessage: "",
		},
		{
			name:            "RateLimited",
			err:             &openai.APIError{HTTPStatusCode: 429, Message: "slow down"},
			wantKind:        ErrRateLimited,
			wantUserMessage: "slow down",
		},
		{
			name:            "ServerError",
			err:             &openai.APIError{HTTPStatusCode: 500, Message: "internal error"},
			wantKind:        ErrProviderError,
			wantUserMessage: "internal error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := NewOpenAIClient("test-key")
			got := c.classifyError(context.Background(), "ch-1", tt.err)
			if !errors.Is(got, tt.wantKind) {
				t.Errorf("errors.Is(%v, %v) = false, want true", got, tt.wantKind)
			}
			var chatErr *ChatError
			if !errors.As(got, &chatErr) {
				t.Fatalf("expected *ChatError, got %T", got)
			}
			if chatErr.UserMessage != tt.wantUserMessage {
				t.Errorf("UserMessage = %q, want %q", chatErr.UserMessage, tt.wantUserMessage)
			}
		})
	}
}

func TestOpenAIClient_Reset(t *testing.T) {
	c, _ := NewOpenAIClient("test-key")
	c.chatContext["ch-1"] = c.newContext()
	c.chatContext["ch-2"] = c.newContext()

	if err := c.Reset(context.Background(), "ch-1"); err != nil {
		t.Fatalf("Reset returned unexpected error: %v", err)
	}

	if _, exists := c.chatContext["ch-1"]; exists {
		t.Errorf("expected ch-1 to be deleted")
	}
	if _, exists := c.chatContext["ch-2"]; !exists {
		t.Errorf("expected ch-2 to remain")
	}
}
