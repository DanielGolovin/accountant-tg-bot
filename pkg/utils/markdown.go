package utils

import (
	"regexp"
	"strconv"
	"strings"
)

func EscapeMarkdownV2(text string) string {
	escapeChars := []string{"_", "*", "[", "]", "(", ")", "~", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	for _, char := range escapeChars {
		text = strings.ReplaceAll(text, char, "\\"+char)
	}
	return text
}

// ParseExpenseMessage parses a message like "1000 shop" into amount, category, and optional currency
// Example messages: "1000 shop", "500.50 food", "127.99 something else", "100.25 EUR shop", "50 RSD food"
func ParseExpenseMessage(message string) (interface{}, string, string, error) {
	// First check for message with currency: "100 EUR shop" or "100.50 EUR shop"
	reCurrency := regexp.MustCompile(`^(\d+(?:\.\d{1,2})?)\s+([A-Z]{3})\s+(.+)$`)
	matches := reCurrency.FindStringSubmatch(message)

	if len(matches) == 4 {
		amountStr := strings.TrimSpace(matches[1])
		currency := strings.TrimSpace(matches[2])
		category := strings.TrimSpace(matches[3])

		// Try parsing as float first
		if strings.Contains(amountStr, ".") {
			amount, err := strconv.ParseFloat(amountStr, 64)
			if err != nil {
				return 0, "", "", &ParseError{Message: "invalid amount", Cause: err}
			}
			amount = float64(int(amount*100)) / 100
			return amount, category, currency, nil
		} else {
			amount, err := strconv.Atoi(amountStr)
			if err != nil {
				return 0, "", "", &ParseError{Message: "invalid amount", Cause: err}
			}
			return amount, category, currency, nil
		}
	}

	// If no currency format is detected, fallback to the original pattern and use USD as default
	reOld := regexp.MustCompile(`^(\d+(?:\.\d{1,2})?)\s(.*)$`)
	matches = reOld.FindStringSubmatch(message)

	if len(matches) != 3 {
		return 0, "", "", &ParseError{Message: "invalid message format"}
	}

	amountStr := strings.TrimSpace(matches[1])
	category := strings.TrimSpace(matches[2])

	// Try parsing as float first
	if strings.Contains(amountStr, ".") {
		amount, err := strconv.ParseFloat(amountStr, 64)
		if err != nil {
			return 0, "", "", &ParseError{Message: "invalid amount", Cause: err}
		}
		amount = float64(int(amount*100)) / 100
		return amount, category, "USD", nil
	} else {
		// parse as int if no decimal point
		amount, err := strconv.Atoi(amountStr)
		if err != nil {
			return 0, "", "", &ParseError{Message: "invalid amount", Cause: err}
		}
		return amount, category, "USD", nil
	}
}

type ParseError struct {
	Message string
	Cause   error
}

func (e *ParseError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}
