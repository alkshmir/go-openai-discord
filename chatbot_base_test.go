package main

import (
	"fmt"
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

// MockLogger is a mock implementation of Logger for testing
type MockLogger struct {
	printLogs []string
	fatalLogs []string
}

func (m *MockLogger) Println(v ...any) {
	msg := fmt.Sprint(v...)
	fmt.Println(msg)
	m.printLogs = append(m.printLogs, msg)
}

func (m *MockLogger) Fatal(v ...any) {
	msg := fmt.Sprint(v...)
	m.fatalLogs = append(m.fatalLogs, msg)
}

func (m *MockLogger) GetPrintLogs() []string {
	return m.printLogs
}

func (m *MockLogger) GetFatalLogs() []string {
	return m.fatalLogs
}

// mock implementation of sender interface
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
		name     string
		session  *discordgo.Session
		message  *discordgo.MessageCreate
		expected bool
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
					//MessageReference: nil,
				},
			},
			true,
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
					Mentions: []*discordgo.User{}, // no mention
				},
			},
			true,
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
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := isTalkingToBot(test.session, test.message)
			if result != test.expected {
				t.Errorf("isTalkingToBot(%v, %v) = %v, want %v", test.session, test.message, result, test.expected)
			}
		})
	}
}

func TestHandleReply(t *testing.T) {
	tests := []struct {
		name        string
		session     *discordgo.Session
		message     *discordgo.MessageCreate
		expectedMsg string
	}{
		{
			"Mentioned",
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
					//MessageReference: nil,
				},
			},
			"Test reply",
		},
	}
	chatbot := BaseChatBot{}
	mockLogger := MockLogger{}
	mockSender := MockSender{}
	chatbot.logger = &mockLogger
	chatbot.sender = &mockSender
	chatbot.ReplyFunc = func(m string) (string, error) { return "Test reply", nil }
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			chatbot.HandleReply(test.session, test.message)
			t.Log(mockSender.Messages)
			got := mockSender.Messages[mockconstants.TestChannel][len(mockSender.Messages)-1]
			if got != test.expectedMsg {
				t.Errorf("expected reply %v, got %v", test.expectedMsg, got)
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
