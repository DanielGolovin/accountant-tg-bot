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
//     "defaultInputCurrency": "USD",
//     "defaultOutputCurrency": "USD"
//   },
//   "2021-01-01": {
//     "category": {
// 			"USD": [10.0, 20.0, 30.0],
// 			"EUR": [8.0, 16.0, 24.0],
//          ... etc
// 		},  // All amounts are stored in USD with 2 decimal places
//   }
// }

type Database struct {
	Path     string
	Exchange *exchange.RateService
}

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

		initialData := map[string]interface{}{
			"settings": map[string]string{
				"defaultInputCurrency":  "USD",
				"defaultOutputCurrency": "USD",
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

	db.ensureSettings()

	return db
}

func (db *Database) ensureSettings() {
	rawData := db.ReadRaw()

	if settings, ok := rawData["settings"].(map[string]interface{}); ok {
		if _, ok := settings["defaultInputCurrency"]; !ok {
			settings["defaultInputCurrency"] = "USD"
		}
		if _, ok := settings["defaultOutputCurrency"]; !ok {
			settings["defaultOutputCurrency"] = "USD"
		}
	} else {
		rawData["settings"] = map[string]interface{}{
			"defaultInputCurrency":  "USD",
			"defaultOutputCurrency": "USD",
		}
	}

	db.WriteRaw(rawData)
}

func (db *Database) Read() map[string]map[string]map[string][]float64 {
	rawData := db.ReadRaw()
	result := make(map[string]map[string]map[string][]float64)

	for key, value := range rawData {
		if key != "settings" {
			if monthData, ok := value.(map[string]interface{}); ok {
				result[key] = make(map[string]map[string][]float64)
				for category, amounts := range monthData {
					if amtMap, ok := amounts.(map[string]interface{}); ok {
						result[key][category] = make(map[string][]float64)
						for currency, amtArray := range amtMap {
							if floatArray, ok := amtArray.([]interface{}); ok {
								floatSlice := make([]float64, len(floatArray))
								for i, amt := range floatArray {
									if floatVal, ok := amt.(float64); ok {
										floatSlice[i] = floatVal
									}
								}
								result[key][category][currency] = floatSlice
							}
						}
					}
				}
			}
		}
	}

	return result
}

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

func (db *Database) Write(data map[string]map[string]map[string][]float64) {
	rawData := db.ReadRaw()

	for key, value := range data {
		rawData[key] = value
	}

	db.WriteRaw(rawData)
}

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

func (db *Database) GetDefaultCurrency() string {
	rawData := db.ReadRaw()

	if settings, ok := rawData["settings"].(map[string]interface{}); ok {
		if currency, ok := settings["defaultOutputCurrency"].(string); ok {
			return currency
		}
	}

	return "USD"
}

func (db *Database) SetDefaultCurrency(currency string) {
	rawData := db.ReadRaw()

	if settings, ok := rawData["settings"].(map[string]interface{}); ok {
		settings["defaultOutputCurrency"] = currency
	} else {
		rawData["settings"] = map[string]interface{}{
			"defaultOutputCurrency": currency,
		}
	}

	db.WriteRaw(rawData)
}

func (db *Database) GetDefaultInputCurrency() string {
	rawData := db.ReadRaw()

	if settings, ok := rawData["settings"].(map[string]interface{}); ok {
		if currency, ok := settings["defaultInputCurrency"].(string); ok {
			return currency
		}
	}

	return "USD"
}

func (db *Database) SetDefaultInputCurrency(currency string) {
	rawData := db.ReadRaw()

	if settings, ok := rawData["settings"].(map[string]interface{}); ok {
		settings["defaultInputCurrency"] = currency
	} else {
		rawData["settings"] = map[string]interface{}{
			"defaultInputCurrency": currency,
		}
	}

	db.WriteRaw(rawData)
}

func RoundToTwoDecimalPlaces(value float64) float64 {
	return math.Round(value*100) / 100
}

func (db *Database) AddRecord(date string, category string, amount interface{}, currency string) error {
	var amountFloat float64
	switch v := amount.(type) {
	case int:
		amountFloat = float64(v)
	case float64:
		amountFloat = v
	}

	usdAmount := amountFloat
	if currency != "USD" {
		convertedAmount, err := db.Exchange.ConvertToUSD(amountFloat, currency)
		if err != nil {
			return err
		}
		usdAmount = RoundToTwoDecimalPlaces(convertedAmount)
	} else {
		usdAmount = RoundToTwoDecimalPlaces(usdAmount)
	}

	data := db.Read()

	if _, ok := data[date]; !ok {
		data[date] = make(map[string]map[string][]float64)
	}

	if _, ok := data[date][category]; !ok {
		data[date][category] = make(map[string][]float64)
	}

	if _, ok := data[date][category][currency]; !ok {
		data[date][category][currency] = make([]float64, 0)
	}

	data[date][category][currency] = append(data[date][category][currency], amountFloat)

	db.Write(data)

	return nil
}

func (db *Database) SumMonthByCategory() map[string]map[string]float64 {
	data := db.Read()
	result := make(map[string]map[string]float64)

	for date, records := range data {
		for category, amountsByCurrency := range records {
			if _, ok := result[date]; !ok {
				result[date] = make(map[string]float64)
			}

			var sum float64 = 0
			for _, amounts := range amountsByCurrency {
				for _, amount := range amounts {
					sum += amount
				}
			}

			result[date][category] = RoundToTwoDecimalPlaces(sum)
		}
	}

	return result
}

func (db *Database) SumMonth(date string) float64 {
	data := db.Read()
	var sum float64 = 0

	if _, ok := data[date]; !ok {
		return 0
	}

	for _, amountsByCurrency := range data[date] {
		for _, amounts := range amountsByCurrency {
			for _, amount := range amounts {
				sum += amount
			}
		}
	}

	return RoundToTwoDecimalPlaces(sum)
}

func (db *Database) SumMonthInDefaultCurrency(date string) (float64, error) {
	data := db.Read()
	defaultCurrency := db.GetDefaultCurrency()
	var totalInDefaultCurrency float64

	for _, amountsByCurrency := range data[date] {
		for currency, amounts := range amountsByCurrency {
			var sum float64
			for _, amount := range amounts {
				sum += amount
			}

			if currency != defaultCurrency {
				rate, err := db.Exchange.GetRateToUSD(currency)
				if err != nil {
					return 0, err
				}
				sum /= rate
			}

			totalInDefaultCurrency += sum
		}
	}

	return RoundToTwoDecimalPlaces(totalInDefaultCurrency), nil
}
