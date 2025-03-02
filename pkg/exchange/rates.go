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

func (s *RateService) GetRateToUSD(fromCurrency string) (float64, error) {
	// If the source currency is already USD, return 1.0 (1:1 conversion)
	if strings.ToUpper(fromCurrency) == "USD" {
		return 1.0, nil
	}

	apiURL := s.BaseURL + strings.ToUpper(fromCurrency)
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

	rate, ok := rates["USD"].(float64)
	if !ok {
		return 0, fmt.Errorf("USD rate not found")
	}

	return rate, nil
}

func (s *RateService) ConvertToUSD(amount float64, fromCurrency string) (float64, error) {
	rate, err := s.GetRateToUSD(fromCurrency)
	if err != nil {
		return 0, err
	}

	return amount * rate, nil
}

func (s *RateService) ConvertIntToUSD(amount int, fromCurrency string) (float64, error) {
	return s.ConvertToUSD(float64(amount), fromCurrency)
}

func (s *RateService) GetRSDToUSD() (float64, error) {
	return s.GetRateToUSD("RSD")
}

func (s *RateService) ConvertRSDToUSD(amountRSD int) (float64, error) {
	return s.ConvertToUSD(float64(amountRSD), "RSD")
}
