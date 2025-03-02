package exchange

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// RateService provides exchange rate functionality
type RateService struct {
	BaseURL string
}

// NewRateService creates a new rate service
func NewRateService() *RateService {
	return &RateService{
		BaseURL: "https://api.exchangerate-api.com/v4/latest/",
	}
}

// GetRateToUSD gets the exchange rate from any currency to USD
func (s *RateService) GetRateToUSD(fromCurrency string) (float64, error) {
	// If the source currency is already USD, return 1.0 (1:1 conversion)
	if strings.ToUpper(fromCurrency) == "USD" {
		return 1.0, nil
	}

	// For any other currency, fetch the rate
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

// ConvertToUSD converts an amount in any currency to USD
func (s *RateService) ConvertToUSD(amount float64, fromCurrency string) (float64, error) {
	rate, err := s.GetRateToUSD(fromCurrency)
	if err != nil {
		return 0, err
	}

	return amount * rate, nil
}

// ConvertIntToUSD converts an integer amount in any currency to USD (maintained for backward compatibility)
func (s *RateService) ConvertIntToUSD(amount int, fromCurrency string) (float64, error) {
	return s.ConvertToUSD(float64(amount), fromCurrency)
}

// GetRSDToUSD gets the exchange rate from RSD to USD (maintained for backward compatibility)
func (s *RateService) GetRSDToUSD() (float64, error) {
	return s.GetRateToUSD("RSD")
}

// ConvertRSDToUSD converts an amount in RSD to USD (maintained for backward compatibility)
func (s *RateService) ConvertRSDToUSD(amountRSD int) (float64, error) {
	return s.ConvertToUSD(float64(amountRSD), "RSD")
}
