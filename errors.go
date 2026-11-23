package main

import "errors"

var (
	ErrContextTooLong = errors.New("chat context exceeded maximum token length")
	ErrUnauthorized   = errors.New("invalid AI provider credentials")
	ErrRateLimited    = errors.New("AI provider rate limited or overloaded")
	ErrProviderError  = errors.New("AI provider server error")
)

// ChatError represents a categorized error from a Responder.
type ChatError struct {
	Kind        error
	UserMessage string
	Err         error
}

func (e *ChatError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	if e.Kind != nil {
		return e.Kind.Error()
	}
	return e.UserMessage
}

func (e *ChatError) Unwrap() []error {
	return []error{e.Kind, e.Err}
}
