package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/ewohltman/discordgo-mock/mockchannel"
	"github.com/ewohltman/discordgo-mock/mockconstants"
	"github.com/ewohltman/discordgo-mock/mockguild"
	"github.com/ewohltman/discordgo-mock/mockmember"
	"github.com/ewohltman/discordgo-mock/mockrest"
	"github.com/ewohltman/discordgo-mock/mockrole"
	"github.com/ewohltman/discordgo-mock/mocksession"
	"github.com/ewohltman/discordgo-mock/mockstate"
	"github.com/ewohltman/discordgo-mock/mockuser"
)

type mockResponder struct {
	replyFunc     func(ctx context.Context, channelID, prompt string) (string, error)
	resetFunc     func(ctx context.Context, channelID string) error
	resetChannels []string
}

func (m *mockResponder) Reply(ctx context.Context, channelID, prompt string) (string, error) {
	return m.replyFunc(ctx, channelID, prompt)
}

func (m *mockResponder) Reset(ctx context.Context, channelID string) error {
	m.resetChannels = append(m.resetChannels, channelID)
	if m.resetFunc != nil {
		return m.resetFunc(ctx, channelID)
	}
	return nil
}

// MockSender is a mock implementation of Sender for testing.
type MockSender struct {
	Messages map[string][]string
}

func (ms *MockSender) ChannelSend(s *discordgo.Session, channelID string, content string) (*discordgo.Message, error) {
	if ms.Messages == nil {
		ms.Messages = make(map[string][]string)
	}
	ms.Messages[channelID] = append(ms.Messages[channelID], content)
	return nil, nil
}

func (ms *MockSender) ReplySend(s *discordgo.Session, channelID string, content string, reference *discordgo.MessageReference) (*discordgo.Message, error) {
	return ms.ChannelSend(s, channelID, content)
}

func TestRemoveMention(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"NoMention", "Hello, World!", "Hello, World!"},
		{"SingleMention", "Hey <@123>, how are you?", "Hey , how are you?"},
		{"MultipleMentions", "Hi <@456>, <@789>!", "Hi , !"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := removeMention(test.input)
			if result != test.expected {
				t.Errorf("removeMention(%v) = %v, want %v", test.input, result, test.expected)
			}
		})
	}
}

func TestIsTalkingToBot(t *testing.T) {
	tests := []struct {
		name          string
		session       *discordgo.Session
		message       *discordgo.MessageCreate
		expected      bool
		expectedError error
	}{
		{
			"IncludesMention",
			newSession(),
			&discordgo.MessageCreate{
				Message: &discordgo.Message{
					ID:        "dummy_message_id",
					Content:   "Hello <@123>, how are you?",
					ChannelID: mockconstants.TestChannel,
					Author: &discordgo.User{
						ID:       "dummy",
						Username: "Test user",
					},
					Mentions: []*discordgo.User{
						mockuser.New(
							mockuser.WithID("123"),
							mockuser.WithUsername(mockconstants.TestUser+"Bot"),
							mockuser.WithBotFlag(true),
						),
					},
				},
			},
			true,
			nil,
		},
		{
			"DMchannel",
			newSession(),
			&discordgo.MessageCreate{
				Message: &discordgo.Message{
					ID:        "dummy_message_id",
					Content:   "Hello",
					ChannelID: mockconstants.TestPrivateChannel,
					Author: &discordgo.User{
						ID:       "dummy",
						Username: "Test user",
					},
					Mentions: []*discordgo.User{},
				},
			},
			true,
			nil,
		},
		{
			"DMchannelWithMention",
			newSession(),
			&discordgo.MessageCreate{
				Message: &discordgo.Message{
					ID:        "dummy_message_id",
					Content:   "Hello <@123>",
					ChannelID: mockconstants.TestPrivateChannel,
					Author: &discordgo.User{
						ID:       "dummy",
						Username: "Test user",
					},
					Mentions: []*discordgo.User{
						mockuser.New(
							mockuser.WithID("123"),
							mockuser.WithUsername(mockconstants.TestUser+"Bot"),
							mockuser.WithBotFlag(true),
						),
					},
				},
			},
			true,
			nil,
		},
		{
			"NotTalking",
			newSession(),
			&discordgo.MessageCreate{
				Message: &discordgo.Message{
					ID:        "dummy_message_id",
					Content:   "Hello",
					ChannelID: mockconstants.TestChannel,
					Author: &discordgo.User{
						ID:       "dummy",
						Username: "Test user",
					},
				},
			},
			false,
			nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := isTalkingToBot(test.session, test.message)
			if result != test.expected {
				t.Errorf("isTalkingToBot(%v, %v) = %v, want %v", test.session, test.message, result, test.expected)
			}
			if err != nil && test.expectedError == nil {
				t.Errorf("isTalkingToBot(%v, %v) returned unexpected error: %v", test.session, test.message, err)
			}
			if err == nil && test.expectedError != nil {
				t.Errorf("isTalkingToBot(%v, %v) expected error: %v, got nil", test.session, test.message, test.expectedError)
			}
			if err != nil && test.expectedError != nil && err.Error() != test.expectedError.Error() {
				t.Errorf("isTalkingToBot(%v, %v) expected error: %v, got: %v", test.session, test.message, test.expectedError, err)
			}
		})
	}
}

func TestHandleMessage(t *testing.T) {
	mentionedMsg := func() *discordgo.MessageCreate {
		return &discordgo.MessageCreate{
			Message: &discordgo.Message{
				ID:        "dummy_message_id",
				Content:   "Hello <@123>, how are you?",
				ChannelID: mockconstants.TestChannel,
				Author: &discordgo.User{
					ID:       "dummy",
					Username: "Test user",
				},
				Mentions: []*discordgo.User{
					mockuser.New(
						mockuser.WithID("123"),
						mockuser.WithUsername(mockconstants.TestUser+"Bot"),
						mockuser.WithBotFlag(true),
					),
				},
			},
		}
	}

	tests := []struct {
		name        string
		replyFunc   func(ctx context.Context, channelID, prompt string) (string, error)
		expectedMsg []string
	}{
		{
			name: "Mentioned",
			replyFunc: func(ctx context.Context, channelID, prompt string) (string, error) {
				return "Test reply", nil
			},
			expectedMsg: []string{"Test reply"},
		},
		{
			name: "ContextTooLong",
			replyFunc: func(ctx context.Context, channelID, prompt string) (string, error) {
				return "", &ChatError{
					Kind:        ErrContextTooLong,
					UserMessage: "Cleared the message history as reached maximum token length. Please retry.",
					Err:         errors.New("token limit exceeded"),
				}
			},
			expectedMsg: []string{"Cleared the message history as reached maximum token length. Please retry."},
		},
		{
			name: "UnauthorizedDoesNotSendUserMessage",
			replyFunc: func(ctx context.Context, channelID, prompt string) (string, error) {
				return "", &ChatError{
					Kind: ErrUnauthorized,
					Err:  errors.New("invalid api key"),
				}
			},
			expectedMsg: nil,
		},
		{
			name: "RateLimited",
			replyFunc: func(ctx context.Context, channelID, prompt string) (string, error) {
				return "", &ChatError{
					Kind:        ErrRateLimited,
					UserMessage: "rate limit reached",
					Err:         errors.New("429"),
				}
			},
			expectedMsg: []string{"rate limit reached"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockSender := &MockSender{}
			bot := NewBot(
				&mockResponder{replyFunc: test.replyFunc},
				WithLogger(slog.New(slog.DiscardHandler)),
				WithSender(mockSender),
			)

			bot.HandleMessage(newSession(), mentionedMsg())

			got := mockSender.Messages[mockconstants.TestChannel]
			if len(got) != len(test.expectedMsg) {
				t.Fatalf("sent %d messages (%#v), want %d (%#v)", len(got), got, len(test.expectedMsg), test.expectedMsg)
			}
			for i := range got {
				if got[i] != test.expectedMsg[i] {
					t.Errorf("message[%d] = %#v, want %#v", i, got[i], test.expectedMsg[i])
				}
			}
		})
	}
}

func newSession() *discordgo.Session {
	state, err := newState()
	if err != nil {
		panic(err)
	}
	session, err := mocksession.New(
		mocksession.WithState(state),
		mocksession.WithClient(&http.Client{
			Transport: mockrest.NewTransport(state),
		}),
	)
	if err != nil {
		panic(err)
	}
	return session
}

func newState() (*discordgo.State, error) {
	role := mockrole.New(
		mockrole.WithID(mockconstants.TestRole),
		mockrole.WithName(mockconstants.TestRole),
		mockrole.WithPermissions(discordgo.PermissionViewChannel),
	)

	botUser := mockuser.New(
		mockuser.WithID("123"),
		mockuser.WithUsername(mockconstants.TestUser+"Bot"),
		mockuser.WithBotFlag(true),
	)

	botMember := mockmember.New(
		mockmember.WithUser(botUser),
		mockmember.WithGuildID(mockconstants.TestGuild),
		mockmember.WithRoles(role),
	)

	userMember := mockmember.New(
		mockmember.WithUser(mockuser.New(
			mockuser.WithID(mockconstants.TestUser),
			mockuser.WithUsername(mockconstants.TestUser),
		)),
		mockmember.WithGuildID(mockconstants.TestGuild),
		mockmember.WithRoles(role),
	)

	channel := mockchannel.New(
		mockchannel.WithID(mockconstants.TestChannel),
		mockchannel.WithGuildID(mockconstants.TestGuild),
		mockchannel.WithName(mockconstants.TestChannel),
		mockchannel.WithType(discordgo.ChannelTypeGuildText),
	)

	dmChannel := mockchannel.New(
		mockchannel.WithID(mockconstants.TestPrivateChannel),
		mockchannel.WithGuildID(mockconstants.TestGuild),
		mockchannel.WithName(mockconstants.TestPrivateChannel),
		mockchannel.WithType(discordgo.ChannelTypeDM),
		mockchannel.WithPermissionOverwrites(&discordgo.PermissionOverwrite{
			ID:   botMember.User.ID,
			Type: discordgo.PermissionOverwriteTypeMember,
			Deny: discordgo.PermissionViewChannel,
		}),
	)

	return mockstate.New(
		mockstate.WithUser(botUser),
		mockstate.WithGuilds(
			mockguild.New(
				mockguild.WithID(mockconstants.TestGuild),
				mockguild.WithName(mockconstants.TestGuild),
				mockguild.WithRoles(role),
				mockguild.WithChannels(channel, dmChannel),
				mockguild.WithMembers(botMember, userMember),
			),
		),
	)
}
