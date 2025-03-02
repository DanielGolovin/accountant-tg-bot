package database

import (
	"accountant-bot/pkg/exchange"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
)

// DB structure
// {
//   "settings": {
//     "defaultCurrency": "USD"
//   },
//   "2021-01-01": {
//     "category": float64[],  // All amounts are stored in USD with 2 decimal places
//   }
// }

// Database represents the DB operations
type Database struct {
	Path     string
	Exchange *exchange.RateService
}

// InitDB initializes the database
func InitDB() *Database {
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

		// Initialize with empty data and default settings
		initialData := map[string]interface{}{
			"settings": map[string]string{
				"defaultCurrency": "USD",
			},
		}

		jsonData, err := json.Marshal(initialData)
		if err != nil {
			panic(err)
		}

		err = os.WriteFile(dbPath, jsonData, 0644)
		if err != nil {
			panic(err)
		}
	}

	db := &Database{
		Path:     dbPath,
		Exchange: exchange.NewRateService(),
	}

	// Ensure settings exist in the database
	db.ensureSettings()

	return db
}

// ensureSettings makes sure settings section exists in the database
func (db *Database) ensureSettings() {
	rawData := db.ReadRaw()

	// Check if settings exist
	if _, ok := rawData["settings"]; !ok {
		rawData["settings"] = map[string]string{
			"defaultCurrency": "USD",
		}
		db.WriteRaw(rawData)
	}
}

// Read reads the database (excluding settings)
func (db *Database) Read() map[string]map[string][]float64 {
	rawData := db.ReadRaw()
	result := make(map[string]map[string][]float64)

	// Copy only the data sections (excluding settings)
	for key, value := range rawData {
		if key != "settings" {
			if monthData, ok := value.(map[string]interface{}); ok {
				result[key] = make(map[string][]float64)
				for category, amounts := range monthData {
					if amtArray, ok := amounts.([]interface{}); ok {
						floatArray := make([]float64, len(amtArray))
						for i, amt := range amtArray {
							if floatVal, ok := amt.(float64); ok {
								floatArray[i] = floatVal
							}
						}
						result[key][category] = floatArray
					}
				}
			}
		}
	}

	return result
}

// ReadRaw reads the raw database including settings
func (db *Database) ReadRaw() map[string]interface{} {
	result := make(map[string]interface{})

	buf, err := os.ReadFile(db.Path)
	if err != nil {
		panic(err)
	}

	if len(buf) == 0 {
		return result
	}

	err = json.Unmarshal(buf, &result)
	if err != nil {
		panic(err)
	}

	return result
}

// Write writes to the database (preserving settings)
func (db *Database) Write(data map[string]map[string][]float64) {
	rawData := db.ReadRaw()

	// Update data sections but preserve settings
	for key, value := range data {
		rawData[key] = value
	}

	db.WriteRaw(rawData)
}

// WriteRaw writes the raw data to the database
func (db *Database) WriteRaw(data map[string]interface{}) {
	buf, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(db.Path, buf, 0644)
	if err != nil {
		panic(err)
	}
}

// GetDefaultCurrency gets the default currency from settings
func (db *Database) GetDefaultCurrency() string {
	rawData := db.ReadRaw()

	if settings, ok := rawData["settings"].(map[string]interface{}); ok {
		if currency, ok := settings["defaultCurrency"].(string); ok {
			return currency
		}
	}

	// Default fallback
	return "USD"
}

// SetDefaultCurrency sets the default currency in settings
func (db *Database) SetDefaultCurrency(currency string) {
	rawData := db.ReadRaw()

	if settings, ok := rawData["settings"].(map[string]interface{}); ok {
		settings["defaultCurrency"] = currency
	} else {
		rawData["settings"] = map[string]interface{}{
			"defaultCurrency": currency,
		}
	}

	db.WriteRaw(rawData)
}

// RoundToTwoDecimalPlaces rounds a float to two decimal places
func RoundToTwoDecimalPlaces(value float64) float64 {
	return math.Round(value*100) / 100
}

// AddRecord adds a new record to the database
// If the currency is not USD, it will convert the amount to USD before storing
// Amount can be provided as int or float64, will be stored as float64 with 2 decimal places
func (db *Database) AddRecord(date string, category string, amount interface{}, currency string) error {
	// Convert amount to float64
	var amountFloat float64
	switch v := amount.(type) {
	case int:
		amountFloat = float64(v)
	case float64:
		amountFloat = v
	}

	// Convert amount to USD if not already in USD
	usdAmount := amountFloat
	if currency != "USD" {
		convertedAmount, err := db.Exchange.ConvertToUSD(amountFloat, currency)
		if err != nil {
			return err
		}
		// Round to 2 decimal places for storage
		usdAmount = RoundToTwoDecimalPlaces(convertedAmount)
	} else {
		// Still round to ensure 2 decimal places
		usdAmount = RoundToTwoDecimalPlaces(usdAmount)
	}

	data := db.Read()

	if _, ok := data[date]; !ok {
		data[date] = make(map[string][]float64)
	}

	if _, ok := data[date][category]; !ok {
		data[date][category] = make([]float64, 0)
	}

	data[date][category] = append(data[date][category], usdAmount)

	db.Write(data)

	return nil
}

// SumMonthByCategory sums all records by category for each month
func (db *Database) SumMonthByCategory() map[string]map[string]float64 {
	data := db.Read()
	result := make(map[string]map[string]float64)

	for date, records := range data {
		for category, amounts := range records {
			if _, ok := result[date]; !ok {
				result[date] = make(map[string]float64)
			}

			var sum float64 = 0
			for _, amount := range amounts {
				sum += amount
			}

			// Round the final sum to 2 decimal places
			result[date][category] = RoundToTwoDecimalPlaces(sum)
		}
	}

	return result
}

// SumMonth sums all records for a specific month
func (db *Database) SumMonth(date string) float64 {
	data := db.Read()
	var sum float64 = 0

	if _, ok := data[date]; !ok {
		return 0
	}

	for _, amounts := range data[date] {
		for _, amount := range amounts {
			sum += amount
		}
	}

	// Round the final sum to 2 decimal places
	return RoundToTwoDecimalPlaces(sum)
}
