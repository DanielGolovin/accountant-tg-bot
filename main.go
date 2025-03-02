package main

import (
	"accountant-bot/pkg/bot"
	"accountant-bot/pkg/database"
	"log"
	"os"
)

func main() {
	// Initialize database
	db := database.InitDB()

	// Get API token from environment
	token := os.Getenv("TELEGRAM_BOT_API_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_API_TOKEN env variable is required")
	}

	// Create and start the bot
	telegramBot, err := bot.NewBot(token, db)
	if err != nil {
		log.Fatalf("Error creating bot: %v", err)
	}

	log.Println("Bot started successfully!")
	telegramBot.Start()
}
