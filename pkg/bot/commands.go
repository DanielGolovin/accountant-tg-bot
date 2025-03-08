package bot

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) handleCommand(message *tgbotapi.Message) {
	switch message.Command() {
	case "dump_db":
		b.handleDumpDbCommand(message)
	case "setcurrency":
		b.handleSetCurrencyCommand(message)
	case "setinputcurrency":
		b.handleSetInputCurrencyCommand(message)
	case "help":
		b.handleHelpCommand(message)
	}
}

func (b *Bot) handleHelpCommand(message *tgbotapi.Message) {
	defaultCurrency := b.DB.GetDefaultCurrency()
	defaultInputCurrency := b.DB.GetDefaultInputCurrency()

	helpText := fmt.Sprintf(`Available commands:
- Add expense: "<amount> <category>" (uses default input currency: %s)
- Add expense with explicit currency: "<amount> <currency> <category>"
- /setcurrency <currency> - Set default output currency (e.g. USD, EUR, RSD)
- /setinputcurrency <currency> - Set default input currency (e.g. USD, EUR, RSD)
- /dump_db - Download database
- /help - Show this help`, defaultCurrency, defaultInputCurrency)

	b.API.Send(tgbotapi.NewMessage(message.Chat.ID, helpText))
}

func (b *Bot) handleSetCurrencyCommand(message *tgbotapi.Message) {
	args := strings.TrimSpace(message.CommandArguments())
	if args == "" {
		defaultCurrency := b.DB.GetDefaultCurrency()
		b.API.Send(tgbotapi.NewMessage(message.Chat.ID,
			fmt.Sprintf("Current default output currency is %s. To change it, use /setcurrency <currency-code>", defaultCurrency)))
		return
	}

	currency := strings.ToUpper(args)

	b.DB.SetDefaultCurrency(currency)

	b.API.Send(tgbotapi.NewMessage(message.Chat.ID,
		fmt.Sprintf("Default output currency set to %s. All new expenses will use this currency unless specified otherwise.", currency)))
}

func (b *Bot) handleSetInputCurrencyCommand(message *tgbotapi.Message) {
	args := strings.TrimSpace(message.CommandArguments())
	if args == "" {
		defaultInputCurrency := b.DB.GetDefaultInputCurrency()
		b.API.Send(tgbotapi.NewMessage(message.Chat.ID,
			fmt.Sprintf("Current default input currency is %s. To change it, use /setinputcurrency <currency-code>", defaultInputCurrency)))
		return
	}

	currency := strings.ToUpper(args)

	b.DB.SetDefaultInputCurrency(currency)

	b.API.Send(tgbotapi.NewMessage(message.Chat.ID,
		fmt.Sprintf("Default input currency set to %s. All new expenses will use this currency unless specified otherwise.", currency)))
}

func (b *Bot) handleDumpDbCommand(message *tgbotapi.Message) {
	data := b.DB.Dump()

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
