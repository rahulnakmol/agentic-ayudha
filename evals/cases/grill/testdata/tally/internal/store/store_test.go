package store

import "testing"

func TestAddNeedsInit(t *testing.T) {
	if _, err := Load("/nonexistent/config.toml"); err == nil {
		t.Fatal("want an error before tally init")
	}
}
