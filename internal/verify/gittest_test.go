package verify

import (
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/gittest"
)

// TestMain keeps git's background maintenance off the tests' temporary repositories.
func TestMain(m *testing.M) { gittest.Main(m) }
