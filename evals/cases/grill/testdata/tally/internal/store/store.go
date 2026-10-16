// Package store keeps tally's expenses in files (synthetic eval fixture; never built).
package store

import "errors"

// Expense is one recorded expense.
type Expense struct {
	Amount, Category, Currency, Date, Note, Tags string
	Split                                        int
}

// Settings is the configuration tally init writes.
type Settings struct{ Currency string }

var errStub = errors.New("fixture")

func Init(config, dataDir, currency string, force bool) error { return errStub }
func Load(config string) (Settings, error)                     { return Settings{}, errStub }
func Add(s Settings, dataDir string, e Expense) error          { return errStub }
func List(dataDir, from, to, category, tag string, limit int, format string, noColor bool) error {
	return errStub
}
func Report(dataDir, from, to, by, format, currency string) error { return errStub }
func Export(dataDir, format, out, from, to string) error          { return errStub }
func Config(config string, args []string) error                   { return errStub }
