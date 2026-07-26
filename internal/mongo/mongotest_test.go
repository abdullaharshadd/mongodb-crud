```go
package mongo

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newTestLogger returns a *log.Logger that writes into the supplied buffer so
// we can assert on logged messages without touching stdout/stderr.
func newTestLogger(buf *bytes.Buffer) *log.Logger {
	return log.New(buf, "", 0)
}

// ---------------------------------------------------------------------------
// parseArgs
// ---------------------------------------------------------------------------

func Test_parseArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantIDs   []int
		wantError bool
	}{
		{
			name:    "empty args",
			args:    []string{},
			wantIDs: []int{},
		},
		{
			name:    "single valid integer",
			args:    []string{"1"},
			wantIDs: []int{1},
		},
		{
			name:    "multiple valid integers",
			args:    []string{"1", "2", "3", "4"},
			wantIDs: []int{1, 2, 3, 4},
		},
		{
			name:    "zero and negative integers",
			args:    []string{"0", "-1", "-99"},
			wantIDs: []int{0, -1, -99},
		},
		{
			name:    "integers greater than 4",
			args:    []string{"5", "100"},
			wantIDs: []int{5, 100},
		},
		{
			name:      "non-numeric argument",
			args:      []string{"abc"},
			wantError: true,
		},
		{
			name:      "mixed valid and non-numeric",
			args:      []string{"1", "abc", "3"},
			wantError: true,
		},
		{
			name:      "float string is not valid integer",
			args:      []string{"1.5"},
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ids, err := parseArgs(tc.args)
			if tc.wantError {
				assert.Error(t, err)
				assert.Nil(t, ids)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantIDs, ids)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Validate
// ---------------------------------------------------------------------------


// ---------------------------------------------------------------------------
// HasDispatchable
// ---------------------------------------------------------------------------


// ---------------------------------------------------------------------------
// NewRunner
// ---------------------------------------------------------------------------


// ---------------------------------------------------------------------------
// Runner.Run – welcome message and argument-count gating
// ---------------------------------------------------------------------------





// ---------------------------------------------------------------------------
// Runner.RunLazy – dispatch behaviour without real DB
//
// We can test the logging / flow without invoking LoadMethods by relying on
// the fact that dispatch logs "X Process is started..." BEFORE calling
// LoadMethods. If LoadMethods fails we still see the log line. We accept that
// RunLazy will return an error from the impl when a real connection is absent –
// we only assert on the logged output, not on a nil error.
// ---------------------------------------------------------------------------



