package exchange

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type RateService struct {
	BaseURL string
}

func NewRateService() *RateService {
	return &RateService{
		BaseURL: "https://api.exchangerate-api.com/v4/latest/",
	}
}

func (s *RateService) Convert(amount float64, from, to string) (float64, error) {
	if strings.ToUpper(from) == strings.ToUpper(to) {
		return amount, nil
	}

	apiURL := s.BaseURL + strings.ToUpper(from)
	resp, err := http.Get(apiURL)
	if err != nil {
		return 0, err
	}

	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	rates, ok := result["rates"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid response format")
	}

	rate, ok := rates[strings.ToUpper(to)].(float64)
	if !ok {
		return 0, fmt.Errorf("rate not found")
	}

	converted := amount * rate

	return converted, nil
}
