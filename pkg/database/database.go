package database

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
)

// DB structure
// {
//     "transactions": {
//			"2025-03-02": [
//				{ "category": "one", "amount": 1000, "currency": "RSD" },
//				{ "category": "two", "amount": 2000, "currency": "USD" },
//				{ "category": "three", "amount": 3000, "currency": "RSD" },
//				{ "category": "three", "amount": 3000, "currency": "RSD" }
//			]
//  	},
//         "defaultInputCurrency": "RSD",
//         "defaultOutputCurrency": "RSD"
//     }
// }

type DBData struct {
	Transactions map[string][]Transaction `json:"transactions"`
	Settings     Settings                 `json:"settings"`
}

type Database struct {
	Path string
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
	}

	db := &Database{
		Path: dbPath,
	}

	db.ensureSettings()

	return db
}

func (db *Database) readRaw() DBData {
	file, err := os.Open(db.Path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	var data DBData
	err = decoder.Decode(&data)
	if err != nil {
		panic(err)
	}

	return data
}

func (db *Database) writeRaw(data DBData) {
	file, err := os.Create(db.Path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	err = encoder.Encode(data)
	if err != nil {
		panic(err)
	}
}

func (db *Database) Dump() DBData {
	return db.readRaw()
}

func (db *Database) DBFrom(data DBData) {
	db.writeRaw(data)
}

func RoundToTwoDecimalPlaces(value float64) float64 {
	return math.Round(value*100) / 100
}
