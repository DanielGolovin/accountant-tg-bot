package bot

import (
	"accountant-bot/pkg/database"
	"accountant-bot/pkg/exchange"
	"accountant-bot/pkg/utils"
	"encoding/json"
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	API      *tgbotapi.BotAPI
	DB       *database.Database
	Exchange *exchange.RateService
}

func NewBot(token string, db *database.Database) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	return &Bot{
		API:      api,
		DB:       db,
		Exchange: exchange.NewRateService(),
	}, nil
}

func (b *Bot) Start() {
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30

	updates := b.API.GetUpdatesChan(updateConfig)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		log.Printf("Received message: %s", update.Message.Text)

		if update.Message.IsCommand() {
			b.handleCommand(update.Message)
		} else if update.Message.Document != nil {
			b.handleDbUpload(update.Message)
		} else {
			b.handleRegularMessage(update.Message)
		}
	}
}

func (b *Bot) handleRegularMessage(message *tgbotapi.Message) {
	amount, category, currency, err := utils.ParseExpenseMessage(message.Text, b.DB.GetDefaultInputCurrency())
	if err != nil {
		b.API.Send(tgbotapi.NewMessage(message.Chat.ID, "Invalid message format. Should be: <amount> <category> or <amount> <currency> <category>"))
		return
	}

	transaction := database.CreateTransaction(category, amount, currency)
	currentDate := time.Now().Format("2006-01-02")
	b.DB.AddTransaction(currentDate, transaction)

	responseMsg := b.formatResponseMessage(currentDate)
	msg := tgbotapi.NewMessage(message.Chat.ID, responseMsg)
	msg.ParseMode = "MarkdownV2"
	b.API.Send(msg)
}

func (b *Bot) formatResponseMessage(currentDate string) string {
	monthlyTransactions := b.DB.GetMonthlyTransactions(currentDate)
	defaultOutputCurrency := b.DB.GetDefaultCurrency()

	totalInDefaultCurrency := 0.0
	totalByCategory := make(map[string]map[string]float64)

	for _, transaction := range monthlyTransactions {
		if totalByCategory[transaction.Category] == nil {
			totalByCategory[transaction.Category] = make(map[string]float64)
		}
		totalByCategory[transaction.Category][transaction.Currency] += transaction.Amount
	}

	for _, currencyTotalMap := range totalByCategory {
		for currency, total := range currencyTotalMap {
			convertedAmount, err := b.Exchange.Convert(total, currency, defaultOutputCurrency)
			if err != nil {
				log.Printf("Error converting amount: %v", err)
				continue
			}

			totalInDefaultCurrency += convertedAmount
		}
	}

	jsonBytes, err := json.Marshal(totalByCategory)
	if err != nil {
		log.Printf("Error marshalling totalInDefaultCurrencyByCategory: %v", err)
	}

	defaultCurrency := b.DB.GetDefaultCurrency()
	responseMsg := fmt.Sprintf("Total for this month in %s: %.2f\n", defaultCurrency, totalInDefaultCurrency)
	responseMsg += "```json\n" + string(jsonBytes) + "\n```"
	return utils.EscapeMarkdownV2(responseMsg)
}
