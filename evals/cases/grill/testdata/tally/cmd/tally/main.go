// Command tally records and reports expenses (synthetic eval fixture; never built).
package main

import (
	"flag"
	"fmt"
	"os"

	"example.com/tally/internal/store"
)

var (
	configPath = flag.String("config", "~/.tally/config.toml", "configuration file")
	dataDir    = flag.String("data-dir", "~/.tally/data", "where expenses are stored")
	noColor    = flag.Bool("no-color", false, "disable colour")
)

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: tally [--config file] [--data-dir dir] [--no-color] <init|add|list|report|export|config> [flags]")
		os.Exit(1)
	}
	var err error
	switch args := flag.Args(); args[0] {
	case "init":
		err = runInit(args[1:])
	case "add":
		err = runAdd(args[1:])
	case "list":
		err = runList(args[1:])
	case "report":
		err = runReport(args[1:])
	case "export":
		err = runExport(args[1:])
	case "config":
		err = runConfig(args[1:])
	default:
		err = fmt.Errorf("unknown command %q", args[0])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "tally:", err)
		os.Exit(1)
	}
}

func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	currency := fs.String("currency", "GBP", "default currency")
	force := fs.Bool("force", false, "overwrite an existing configuration")
	_ = fs.Parse(args)
	return store.Init(*configPath, *dataDir, *currency, *force)
}

func runAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	amount := fs.String("amount", "", "amount, required")
	category := fs.String("category", "", "category, required")
	currency := fs.String("currency", "", "currency (default: the configured one)")
	date := fs.String("date", "", "date, YYYY-MM-DD, required")
	note := fs.String("note", "", "free-text note")
	tag := fs.String("tag", "", "comma-separated tags")
	split := fs.Int("split", 1, "split the amount between N people")
	_ = fs.Parse(args)
	if *amount == "" || *category == "" || *date == "" {
		return fmt.Errorf("missing required flag")
	}
	cfg, err := store.Load(*configPath)
	if err != nil {
		return err
	}
	return store.Add(cfg, *dataDir, store.Expense{Amount: *amount, Category: *category, Currency: *currency, Date: *date, Note: *note, Tags: *tag, Split: *split})
}

func runList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	from := fs.String("from", "", "first date")
	to := fs.String("to", "", "last date")
	category := fs.String("category", "", "only this category")
	tag := fs.String("tag", "", "only this tag")
	limit := fs.Int("limit", 50, "at most this many")
	format := fs.String("format", "table", "table or csv")
	_ = fs.Parse(args)
	return store.List(*dataDir, *from, *to, *category, *tag, *limit, *format, *noColor)
}

func runReport(args []string) error {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	from := fs.String("from", "", "first date")
	to := fs.String("to", "", "last date")
	by := fs.String("by", "category", "category, tag, or month")
	format := fs.String("format", "table", "table or csv")
	currency := fs.String("currency", "", "report in this currency")
	_ = fs.Parse(args)
	return store.Report(*dataDir, *from, *to, *by, *format, *currency)
}

func runExport(args []string) error {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	format := fs.String("format", "csv", "csv or json")
	out := fs.String("out", "", "output file, required")
	from := fs.String("from", "", "first date")
	to := fs.String("to", "", "last date")
	_ = fs.Parse(args)
	return store.Export(*dataDir, *format, *out, *from, *to)
}

func runConfig(args []string) error {
	fs := flag.NewFlagSet("config", flag.ExitOnError)
	_ = fs.Parse(args)
	return store.Config(*configPath, fs.Args())
}
