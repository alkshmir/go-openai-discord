package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

func NewChatBot() (IchatBot, error) {
	cb := &OpenAIChatBot{}
	cb.Init()
	return cb, nil
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	gpt, err := NewChatBot()
	if err != nil {
		log.Fatal("Error creating chat bot: ", err)
	}
	log.Println(gpt)

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
	dg.AddHandler(gpt.HandleReply)
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
