package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

/* DB structure
{
	"2021-01-01": {
		"category": number[],
	}
}
*/

func initDB() string {
	dbFolder := "db"
	dbPath := filepath.Join(dbFolder, "db.json")

	if _, err := os.Stat(dbFolder); os.IsNotExist(err) {
		os.Mkdir(dbFolder, 0755)
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		_, err := os.Create(dbPath)
		if err != nil {
			panic(err)
		}

		err = os.WriteFile(dbPath, []byte("{}"), 0644)
		if err != nil {
			panic(err)
		}
	}

	return dbPath
}

func readDB(dbPath string) map[string]map[string][]int {
	db := make(map[string]map[string][]int)

	buf, err := os.ReadFile(dbPath)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(buf, &db)
	if err != nil {
		panic(err)
	}

	return db
}

func writeDB(dbPath string, db map[string]map[string][]int) {
	buf, err := json.Marshal(db)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(dbPath, buf, 0644)
	if err != nil {
		panic(err)
	}
}

func addRecord(db map[string]map[string][]int, date string, category string, amount int) {
	if _, ok := db[date]; !ok {
		db[date] = make(map[string][]int)
	}

	if _, ok := db[date][category]; !ok {
		db[date][category] = make([]int, 0)
	}

	db[date][category] = append(db[date][category], amount)
}

func sumMounthByCategory(db map[string]map[string][]int) map[string]map[string]int {
	result := make(map[string]map[string]int)

	for date, records := range db {
		for category, amounts := range records {
			if _, ok := result[date]; !ok {
				result[date] = make(map[string]int)
			}

			sum := 0
			for _, amount := range amounts {
				sum += amount
			}

			result[date][category] = sum
		}
	}

	return result
}

func sumMounth(db map[string]map[string][]int, date string) int {
	sum := 0

	for _, amounts := range db[date] {
		for _, amount := range amounts {
			sum += amount
		}
	}

	return sum
}

func main() {
	dbPath := initDB()

	token := os.Getenv("TELEGRAM_BOT_API_TOKEN")
	if token == "" {
		panic("TELEGRAM_BOT_API_TOKEN env variable is required")
	}

	bot, err := tgbotapi.NewBotAPI(token)

	if err != nil {
		panic(err)
	}

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30

	updatesChan := bot.GetUpdatesChan(updateConfig)

	for update := range updatesChan {
		log.Printf("Received message: %s", update.Message.Text)

		if update.Message == nil {
			continue
		}

		if update.Message.IsCommand() {
			handleCommand(update.Message, bot, dbPath)
		} else if update.Message.Document != nil {
			handleDbUpload(update.Message, bot, dbPath)
		} else {
			handleRegularMessage(update.Message, bot, dbPath)
		}

	}
}

func handleDbUpload(message *tgbotapi.Message, bot *tgbotapi.BotAPI, dbPath string) {
	if message.Document == nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Please upload a file"))
		return
	}

	if message.Document.FileName != "db.json" {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Invalid file name. Should be: db.json"))
		return
	}

	fileUrl, err := bot.GetFileDirectURL(message.Document.FileID)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Error downloading file"))
		return
	}

	resp, err := http.Get(fileUrl)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Error downloading file"))
		return
	}

	defer resp.Body.Close()

	buf, err := io.ReadAll(resp.Body)

	err = os.WriteFile(dbPath, buf, 0644)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Error writing file"))
		return
	}

	bot.Send(tgbotapi.NewMessage(message.Chat.ID, "DB updated"))

}

func handleRegularMessage(message *tgbotapi.Message, bot *tgbotapi.BotAPI, dbPath string) {
	parsed, err := parseMessage(message.Text)

	if err != nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Invalid message format. Should be: <amount> <category>"))
		return
	}

	currentDate := time.Now().Format("2006-01")

	db := readDB(dbPath)
	addRecord(db, currentDate, parsed.category, parsed.amount)
	writeDB(dbPath, db)

	sums := sumMounthByCategory(db)

	sumsForCurrentMonth := sums[currentDate]

	jsonBytes, err := json.Marshal(sumsForCurrentMonth)
	if err != nil {
		panic(err)
	}

	answer := fmt.Sprintf("Total for this month: %d\n", sumMounth(db, currentDate))
	answer += "```json\n"
	answer += string(jsonBytes)
	answer += "\n```"

	msg := tgbotapi.NewMessage(message.Chat.ID, answer)
	msg.ParseMode = "MarkdownV2"

	bot.Send(msg)
}

func handleCommand(message *tgbotapi.Message, bot *tgbotapi.BotAPI, dbPath string) {
	switch message.Command() {
	case "dump_db":
		handleDumpDbCommand(message, bot, dbPath)
	}

	return
}

func handleDumpDbCommand(message *tgbotapi.Message, bot *tgbotapi.BotAPI, dbPath string) {
	db := readDB(dbPath)

	jsonBytes, err := json.Marshal(db)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Error dumping db. DB is corrupted"))
		return
	}

	answer := tgbotapi.NewDocument(message.Chat.ID, tgbotapi.FileBytes{
		Name:  "db.json",
		Bytes: jsonBytes,
	})

	bot.Send(answer)
}

// Example message: "1000 shop", "500 food", "127 something else"
func parseMessage(message string) (*struct {
	category string
	amount   int
}, error) {
	re := regexp.MustCompile(`^(\d+)\s(.*)$`)

	matches := re.FindStringSubmatch(message)
	if len(matches) != 3 {
		return nil, fmt.Errorf("invalid message format")
	}

	amountStr := strings.TrimSpace(matches[1]) // everything before the number
	category := matches[2]                     // the number

	amount, err := strconv.Atoi(amountStr)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %v", err)
	}

	return &struct {
		category string
		amount   int
	}{
		category: category,
		amount:   amount,
	}, nil
}
