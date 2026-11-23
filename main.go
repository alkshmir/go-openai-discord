package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "basic-command",
		Description: "Basic command",
	},
	{
		Name:        "forget",
		Description: "forget chat context of this channel",
	},
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("application exited with error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	if err := godotenv.Load(); err != nil {
		logger.Warn("failed to load .env file, using environment variables", "error", err)
	}

	aiClient, err := NewOpenAIClient(os.Getenv("OPENAI_API_KEY"))
	if err != nil {
		return fmt.Errorf("creating OpenAI client: %w", err)
	}
	logger.Info("initialized OpenAI client")

	bot := NewBot(aiClient, WithLogger(logger))

	botToken := os.Getenv("DISCORD_BOT_TOKEN")
	if botToken == "" {
		return errors.New("DISCORD_BOT_TOKEN not found in .env file or environment variable")
	}
	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + botToken)
	if err != nil {
		return fmt.Errorf("creating Discord session: %w", err)
	}

	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(bot.HandleMessage)
	// In this example, we only care about receiving message events.
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages

	// Open a websocket connection to Discord and begin listening.
	if err := dg.Open(); err != nil {
		return fmt.Errorf("opening Discord connection: %w", err)
	}
	defer dg.Close()

	// Register commands
	commandHandlers := map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"forget": bot.HandleForget,
	}

	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})
	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands))
	for i, v := range commands {
		cmd, err := dg.ApplicationCommandCreate(dg.State.User.ID, "", v)
		if err != nil {
			return fmt.Errorf("creating command %q: %w", v.Name, err)
		}
		logger.Debug("command payload", "command", cmd)
		logger.Info("command created", "name", v.Name)
		registeredCommands[i] = cmd
	}

	// Wait here until CTRL-C or other term signal is received.
	logger.Info("bot is now running, press CTRL-C to exit")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	logger.Info("removing commands")

	for _, v := range registeredCommands {
		if err := dg.ApplicationCommandDelete(dg.State.User.ID, "", v.ID); err != nil {
			logger.Error("failed to delete command", "name", v.Name, "error", err)
			continue
		}
		logger.Info("command deleted", "name", v.Name)
	}

	return nil
}
