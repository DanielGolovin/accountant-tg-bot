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
	amount, category, currency, err := utils.ParseExpenseMessage(message.Text)
	if err != nil {
		b.API.Send(tgbotapi.NewMessage(message.Chat.ID, "Invalid message format. Should be: <amount> <category> or <amount> <currency> <category>"))
		return
	}

	currentDate := time.Now().Format("2006-01")
	originalAmount := amount
	currency = b.determineCurrency(currency, message.Text)
	err = b.DB.AddRecord(currentDate, category, amount, currency)
	if err != nil {
		b.API.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Error adding record: %v", err)))
		return
	}

	responseMsg := b.formatResponseMessage(currentDate, category, originalAmount, currency)
	msg := tgbotapi.NewMessage(message.Chat.ID, responseMsg)
	msg.ParseMode = "MarkdownV2"
	b.API.Send(msg)
}

func (b *Bot) determineCurrency(currency, messageText string) string {
	defaultCurrency := b.DB.GetDefaultCurrency()
	if currency == "USD" && !strings.Contains(strings.ToUpper(messageText), "USD") {
		currency = defaultCurrency
	}
	return currency
}

func (b *Bot) formatResponseMessage(currentDate, category string, originalAmount interface{}, currency string) string {
	sums := b.DB.SumMonthByCategory()
	sumsForCurrentMonth := sums[currentDate]
	jsonBytes, err := json.Marshal(sumsForCurrentMonth)
	if err != nil {
		return "Error processing data"
	}
	totalUSD := b.DB.SumMonth(currentDate)
	defaultCurrency := b.DB.GetDefaultCurrency()
	_, defaultCurrencyStr := b.calculateTotalInDefaultCurrency(totalUSD, defaultCurrency)
	responseMsg := b.createExpenseMessage(originalAmount, category, currency, defaultCurrency)
	responseMsg += fmt.Sprintf("Total for this month: %.2f USD%s\n", totalUSD, defaultCurrencyStr)
	responseMsg += "```json\n" + string(jsonBytes) + "\n```"
	return utils.EscapeMarkdownV2(responseMsg)
}

func (b *Bot) calculateTotalInDefaultCurrency(totalUSD float64, defaultCurrency string) (float64, string) {
	var totalInDefaultCurrency float64
	var defaultCurrencyStr string
	if defaultCurrency != "USD" {
		rate, err := b.Exchange.GetRateToUSD(defaultCurrency)
		if err == nil && rate > 0 {
			totalInDefaultCurrency = totalUSD / rate
			defaultCurrencyStr = fmt.Sprintf(" (≈ %.2f %s)", totalInDefaultCurrency, defaultCurrency)
		}
	}
	return totalInDefaultCurrency, defaultCurrencyStr
}

func (b *Bot) createExpenseMessage(originalAmount interface{}, category, currency, defaultCurrency string) string {
	var responseMsg string
	if currency != "USD" {
		usdAmount, err := b.convertToUSD(originalAmount, currency)
		if err == nil {
			responseMsg = fmt.Sprintf("Added %v %s (≈ %.2f USD) to category '%s'\n", originalAmount, currency, usdAmount, category)
		} else {
			responseMsg = fmt.Sprintf("Added amount in %s to category '%s' (conversion error)\n", currency, category)
		}
	} else {
		switch v := originalAmount.(type) {
		case int:
			responseMsg = fmt.Sprintf("Added %d USD to category '%s'\n", v, category)
		case float64:
			responseMsg = fmt.Sprintf("Added %.2f USD to category '%s'\n", v, category)
		}
	}
	return responseMsg
}

func (b *Bot) convertToUSD(amount interface{}, currency string) (float64, error) {
	switch v := amount.(type) {
	case int:
		return b.Exchange.ConvertIntToUSD(v, currency)
	case float64:
		return b.Exchange.ConvertToUSD(v, currency)
	}
	return 0, fmt.Errorf("invalid amount type")
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

	currency := strings.ToUpper(args)

	_, err := b.Exchange.GetRateToUSD(currency)
	if err != nil {
		b.API.Send(tgbotapi.NewMessage(message.Chat.ID,
			fmt.Sprintf("Invalid currency code '%s'. Please use a valid 3-letter currency code (e.g. USD, EUR, RSD).", currency)))
		return
	}

	b.DB.SetDefaultCurrency(currency)

	b.API.Send(tgbotapi.NewMessage(message.Chat.ID,
		fmt.Sprintf("Default currency set to %s. All new expenses will use this currency unless specified otherwise.", currency)))
}

func (b *Bot) handleDumpDbCommand(message *tgbotapi.Message) {
	data := b.DB.ReadRaw()

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
