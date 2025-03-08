package database

type Settings struct {
	DefaultInputCurrency  string `json:"defaultInputCurrency"`
	DefaultOutputCurrency string `json:"defaultOutputCurrency"`
}

func (db *Database) ensureSettings() {
	rawData := db.readRaw()

	if rawData.Settings.DefaultInputCurrency == "" {
		rawData.Settings.DefaultInputCurrency = "USD"
	}

	if rawData.Settings.DefaultOutputCurrency == "" {
		rawData.Settings.DefaultOutputCurrency = "USD"
	}

	db.writeRaw(rawData)
}

func (db *Database) GetDefaultCurrency() string {
	rawData := db.readRaw()

	return rawData.Settings.DefaultOutputCurrency
}

func (db *Database) SetDefaultCurrency(currency string) {
	rawData := db.readRaw()

	rawData.Settings.DefaultOutputCurrency = currency

	db.writeRaw(rawData)
}

func (db *Database) GetDefaultInputCurrency() string {
	rawData := db.readRaw()

	return rawData.Settings.DefaultInputCurrency
}

func (db *Database) SetDefaultInputCurrency(currency string) {
	rawData := db.readRaw()

	rawData.Settings.DefaultInputCurrency = currency

	db.writeRaw(rawData)
}
