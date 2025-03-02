package bot

import (
	"accountant-bot/pkg/database"
	"accountant-bot/pkg/exchange"
	"accountant-bot/pkg/utils"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot represents the Telegram bot
type Bot struct {
	API      *tgbotapi.BotAPI
	DB       *database.Database
	Exchange *exchange.RateService
}

// NewBot creates a new bot instance
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

// Start starts the bot
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
	// Parse the expense message
	amount, category, currency, err := utils.ParseExpenseMessage(message.Text)
	if err != nil {
		b.API.Send(tgbotapi.NewMessage(message.Chat.ID, "Invalid message format. Should be: <amount> <category> or <amount> <currency> <category>"))
		return
	}

	currentDate := time.Now().Format("2006-01")

	// Store the original amount for message display
	originalAmount := amount

	// If currency is USD in the parsing result but user didn't specify it,
	// it means we should use the default currency from DB
	defaultCurrency := b.DB.GetDefaultCurrency()

	if currency == "USD" && !strings.Contains(strings.ToUpper(message.Text), "USD") {
		// User didn't explicitly specify USD, they just didn't specify any currency
		// So we should use the default currency from DB
		currency = defaultCurrency
	}

	// The currency to display in the message (what the user actually entered or the default)
	displayCurrency := currency

	err = b.DB.AddRecord(currentDate, category, amount, currency)
	if err != nil {
		b.API.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Error adding record: %v", err)))
		return
	}

	sums := b.DB.SumMonthByCategory()
	sumsForCurrentMonth := sums[currentDate]

	jsonBytes, err := json.Marshal(sumsForCurrentMonth)
	if err != nil {
		b.API.Send(tgbotapi.NewMessage(message.Chat.ID, "Error processing data"))
		return
	}

	totalUSD := b.DB.SumMonth(currentDate)

	// Calculate totals in default currency if it's not USD
	var totalInDefaultCurrency float64
	var defaultCurrencyStr string

	if defaultCurrency != "USD" {
		rate, err := b.Exchange.GetRateToUSD(defaultCurrency)
		if err == nil && rate > 0 {
			// Convert from USD to default currency
			totalInDefaultCurrency = totalUSD / rate
			defaultCurrencyStr = fmt.Sprintf(" (≈ %.2f %s)", totalInDefaultCurrency, defaultCurrency)
		}
	}

	var responseMsg string

	// Format the message for the specific added expense
	// If user's input currency is not USD, show both input currency and USD values
	if displayCurrency != "USD" {
		var usdAmount float64
		var err error

		// Convert the amount to USD for display
		switch v := originalAmount.(type) {
		case int:
			usdAmount, err = b.Exchange.ConvertIntToUSD(v, displayCurrency)
			if err == nil {
				responseMsg = fmt.Sprintf("Added %d %s (≈ %.2f USD) to category '%s'\n", v, displayCurrency, usdAmount, category)
			}
		case float64:
			usdAmount, err = b.Exchange.ConvertToUSD(v, displayCurrency)
			if err == nil {
				responseMsg = fmt.Sprintf("Added %.2f %s (≈ %.2f USD) to category '%s'\n", v, displayCurrency, usdAmount, category)
			}
		}

		if err != nil {
			responseMsg = fmt.Sprintf("Added amount in %s to category '%s' (conversion error)\n", displayCurrency, category)
		}
	} else {
		// Currency is USD
		switch v := originalAmount.(type) {
		case int:
			responseMsg = fmt.Sprintf("Added %d USD to category '%s'\n", v, category)
		case float64:
			responseMsg = fmt.Sprintf("Added %.2f USD to category '%s'\n", v, category)
		}
	}

	// Add total for the month (in USD and default currency if different)
	responseMsg += fmt.Sprintf("Total for this month: %.2f USD%s\n", totalUSD, defaultCurrencyStr)

	// Add the category breakdown
	responseMsg += "```json\n"
	responseMsg += string(jsonBytes)
	responseMsg += "\n```"

	responseMsg = utils.EscapeMarkdownV2(responseMsg)

	msg := tgbotapi.NewMessage(message.Chat.ID, responseMsg)
	msg.ParseMode = "MarkdownV2"

	b.API.Send(msg)
}

func (b *Bot) handleCommand(message *tgbotapi.Message) {
	switch message.Command() {
	case "dump_db":
		b.handleDumpDbCommand(message)
	case "setcurrency":
		b.handleSetCurrencyCommand(message)
	case "help":
		b.handleHelpCommand(message)
	}
}

func (b *Bot) handleHelpCommand(message *tgbotapi.Message) {
	defaultCurrency := b.DB.GetDefaultCurrency()

	helpText := fmt.Sprintf(`Available commands:
- Add expense: "<amount> <category>" (uses default currency: %s)
- Add expense with explicit currency: "<amount> <currency> <category>"
- /setcurrency <currency> - Set default currency (e.g. USD, EUR, RSD)
- /dump_db - Download database
- /help - Show this help`, defaultCurrency)

	b.API.Send(tgbotapi.NewMessage(message.Chat.ID, helpText))
}

func (b *Bot) handleSetCurrencyCommand(message *tgbotapi.Message) {
	args := strings.TrimSpace(message.CommandArguments())
	if args == "" {
		defaultCurrency := b.DB.GetDefaultCurrency()
		b.API.Send(tgbotapi.NewMessage(message.Chat.ID,
			fmt.Sprintf("Current default currency is %s. To change it, use /setcurrency <currency-code>", defaultCurrency)))
		return
	}

	// Convert to uppercase
	currency := strings.ToUpper(args)

	// Validate currency by trying to get exchange rate
	_, err := b.Exchange.GetRateToUSD(currency)
	if err != nil {
		b.API.Send(tgbotapi.NewMessage(message.Chat.ID,
			fmt.Sprintf("Invalid currency code '%s'. Please use a valid 3-letter currency code (e.g. USD, EUR, RSD).", currency)))
		return
	}

	// Save the currency setting
	b.DB.SetDefaultCurrency(currency)

	b.API.Send(tgbotapi.NewMessage(message.Chat.ID,
		fmt.Sprintf("Default currency set to %s. All new expenses will use this currency unless specified otherwise.", currency)))
}

func (b *Bot) handleDumpDbCommand(message *tgbotapi.Message) {
	data := b.DB.ReadRaw() // Use ReadRaw to include settings

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		b.API.Send(tgbotapi.NewMessage(message.Chat.ID, "Error dumping db. DB is corrupted"))
		return
	}

	answer := tgbotapi.NewDocument(message.Chat.ID, tgbotapi.FileBytes{
		Name:  "db.json",
		Bytes: jsonBytes,
	})

	b.API.Send(answer)
}

func (b *Bot) handleDbUpload(message *tgbotapi.Message) {
	if message.Document == nil {
		b.API.Send(tgbotapi.NewMessage(message.Chat.ID, "Please upload a file"))
		return
	}

	if message.Document.FileName != "db.json" {
		b.API.Send(tgbotapi.NewMessage(message.Chat.ID, "Invalid file name. Should be: db.json"))
		return
	}

	fileURL, err := b.API.GetFileDirectURL(message.Document.FileID)
	if err != nil {
		b.API.Send(tgbotapi.NewMessage(message.Chat.ID, "Error downloading file"))
		return
	}

	resp, err := http.Get(fileURL)
	if err != nil {
		b.API.Send(tgbotapi.NewMessage(message.Chat.ID, "Error downloading file"))
		return
	}

	defer resp.Body.Close()

	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		b.API.Send(tgbotapi.NewMessage(message.Chat.ID, "Error reading file"))
		return
	}

	err = os.WriteFile(b.DB.Path, buf, 0644)
	if err != nil {
		b.API.Send(tgbotapi.NewMessage(message.Chat.ID, "Error writing file"))
		return
	}

	b.API.Send(tgbotapi.NewMessage(message.Chat.ID, "DB updated"))
}
