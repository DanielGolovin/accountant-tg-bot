package database

type Transaction struct {
	Category string  `json:"category"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

func CreateTransaction(category string, amount float64, currency string) Transaction {
	return Transaction{
		Category: category,
		Amount:   amount,
		Currency: currency,
	}
}

func (db *Database) AddTransaction(date string, transaction Transaction) {
	if transaction.Amount == 0 {
		return
	}

	dbData := db.readRaw()
	if dbData.Transactions == nil {
		dbData.Transactions = make(map[string][]Transaction)
	}
	if dbData.Transactions[date] == nil {
		dbData.Transactions[date] = make([]Transaction, 0)
	}

	dbData.Transactions[date] = append(dbData.Transactions[date], transaction)

	db.writeRaw(dbData)
}

func (db *Database) GetMonthlyTransactions(data string) []Transaction {
	dbData := db.readRaw()
	transactions := dbData.Transactions[data]
	return transactions
}
