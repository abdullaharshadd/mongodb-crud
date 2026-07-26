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

func Test_Validate(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantValid bool
		wantError bool
	}{
		{
			name:      "all args in range 1-4",
			args:      []string{"1", "2", "3", "4"},
			wantValid: true,
		},
		{
			name:      "single arg equal to max (4)",
			args:      []string{"4"},
			wantValid: true,
		},
		{
			name:      "single arg equal to 1",
			args:      []string{"1"},
			wantValid: true,
		},
		{
			name:      "negative integers pass validation (only > 4 upper bound checked)",
			args:      []string{"-1", "-100"},
			wantValid: true,
		},
		{
			name:      "zero passes validation",
			args:      []string{"0"},
			wantValid: true,
		},
		{
			name:      "arg equal to 5 fails validation",
			args:      []string{"5"},
			wantValid: false,
		},
		{
			name:      "mixed valid and one too large",
			args:      []string{"1", "2", "5"},
			wantValid: false,
		},
		{
			name:      "all args too large",
			args:      []string{"10", "20"},
			wantValid: false,
		},
		{
			name:      "non-numeric argument causes error",
			args:      []string{"abc"},
			wantError: true,
		},
		{
			name:      "empty args passes validation",
			args:      []string{},
			wantValid: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			valid, err := Validate(tc.args)
			if tc.wantError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantValid, valid)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// HasDispatchable
// ---------------------------------------------------------------------------

func Test_HasDispatchable(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantHas   bool
		wantError bool
	}{
		{
			name:    "single op 1 is dispatchable",
			args:    []string{"1"},
			wantHas: true,
		},
		{
			name:    "single op 2 is dispatchable",
			args:    []string{"2"},
			wantHas: true,
		},
		{
			name:    "single op 3 is dispatchable",
			args:    []string{"3"},
			wantHas: true,
		},
		{
			name:    "single op 4 is dispatchable",
			args:    []string{"4"},
			wantHas: true,
		},
		{
			name:    "multiple valid dispatchable ops",
			args:    []string{"1", "2", "3", "4"},
			wantHas: true,
		},
		{
			name:    "zero is not dispatchable",
			args:    []string{"0"},
			wantHas: false,
		},
		{
			name:    "negative is not dispatchable",
			args:    []string{"-1"},
			wantHas: false,
		},
		{
			name:    "value greater than 4 is not dispatchable (but also not in 1-4)",
			args:    []string{"5"},
			wantHas: false,
		},
		{
			name:    "empty args has no dispatchable",
			args:    []string{},
			wantHas: false,
		},
		{
			name:    "mix of non-dispatchable and dispatchable returns true on first match",
			args:    []string{"0", "2"},
			wantHas: true,
		},
		{
			name:      "non-numeric causes error",
			args:      []string{"abc"},
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			has, err := HasDispatchable(tc.args)
			if tc.wantError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantHas, has)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NewRunner
// ---------------------------------------------------------------------------

func Test_NewRunner(t *testing.T) {
	t.Run("nil logger falls back to default", func(t *testing.T) {
		r := NewRunner(nil)
		assert.NotNil(t, r)
		assert.NotNil(t, r.logger)
	})

	t.Run("provided logger is used", func(t *testing.T) {
		var buf bytes.Buffer
		logger := newTestLogger(&buf)
		r := NewRunner(logger)
		assert.NotNil(t, r)
		assert.Same(t, logger, r.logger)
	})
}

// ---------------------------------------------------------------------------
// Runner.Run – welcome message and argument-count gating
// ---------------------------------------------------------------------------

func Test_Runner_Run_WelcomeMessage(t *testing.T) {
	// The welcome message must always be logged first.
	var buf bytes.Buffer
	r := NewRunner(newTestLogger(&buf))

	// We pass 0 args so we don't reach dispatch (which would need a real DB).
	_ = r.Run(context.Background(), []string{})

	assert.Contains(t, buf.String(), "Welcome to MongoDB CRUD Operations!!!")
}

func Test_Runner_Run_ArgCountGating(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantLogContain string
		wantNoLog      string
	}{
		{
			name:           "zero args",
			args:           []string{},
			wantLogContain: "At least one argument",
		},
		{
			name:           "five args – too many",
			args:           []string{"1", "2", "3", "4", "1"},
			wantLogContain: "At least one argument",
		},
		{
			name:           "exactly one arg",
			args:           []string{"0"}, // 0 is valid (<=4) but not dispatchable; dispatch logs "Invalid Argument"
			wantLogContain: "Invalid Argument",
		},
		{
			name:           "exactly four args all valid non-dispatchable",
			args:           []string{"0", "0", "0", "0"},
			wantLogContain: "Invalid Argument",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			r := NewRunner(newTestLogger(&buf))
			err := r.Run(context.Background(), tc.args)
			require.NoError(t, err)
			logged := buf.String()
			if tc.wantLogContain != "" {
				assert.Contains(t, logged, tc.wantLogContain)
			}
			if tc.wantNoLog != "" {
				assert.NotContains(t, logged, tc.wantNoLog)
			}
		})
	}
}

func Test_Runner_Run_InvalidArgs_TooLarge(t *testing.T) {
	// Any arg > 4 must result in InvalidMsg being logged and no dispatch.
	tests := []struct {
		name string
		args []string
	}{
		{"single arg 5", []string{"5"}},
		{"arg 100", []string{"100"}},
		{"mixed – one arg is 5", []string{"1", "5"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			r := NewRunner(newTestLogger(&buf))
			err := r.Run(context.Background(), tc.args)
			require.NoError(t, err)
			assert.Contains(t, buf.String(), InvalidMsg)
			// Dispatch should not have been reached – no "Process is started" text.
			assert.NotContains(t, buf.String(), "Process is started")
		})
	}
}

func Test_Runner_Run_NonNumericArg(t *testing.T) {
	var buf bytes.Buffer
	r := NewRunner(newTestLogger(&buf))
	err := r.Run(context.Background(), []string{"abc"})
	// Non-numeric args cause a parsing error that must be propagated.
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validating arguments")
}

// ---------------------------------------------------------------------------
// Runner.RunLazy – dispatch behaviour without real DB
//
// We can test the logging / flow without invoking LoadMethods by relying on
// the fact that dispatch logs "X Process is started..." BEFORE calling
// LoadMethods. If LoadMethods fails we still see the log line. We accept that
// RunLazy will return an error from the impl when a real connection is absent –
// we only assert on the logged output, not on a nil error.
// ---------------------------------------------------------------------------

func Test_Runner_RunLazy_Dispatch(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantLogContain []string
		wantLogAbsent  []string
		// When true the test accepts either nil or non-nil error because the
		// impl may fail to connect to MongoDB (not mocked).
		errorIsOk bool
	}{
		{
			name:           "op 1 logs Read Process",
			args:           []string{"1"},
			wantLogContain: []string{"Read Process is started"},
			errorIsOk:      true,
		},
		{
			name:           "op 2 logs Write Process",
			args:           []string{"2"},
			wantLogContain: []string{"Write Process is started"},
			errorIsOk:      true,
		},
		{
			name:           "op 3 logs Update Process",
			args:           []string{"3"},
			wantLogContain: []string{"Update Process is started"},
			errorIsOk:      true,
		},
		{
			name:           "op 4 logs Delete Process",
			args:           []string{"4"},
			wantLogContain: []string{"Delete Process is started"},
			errorIsOk:      true,
		},
		{
			name:           "op 0 logs Invalid Argument",
			args:           []string{"0"},
			wantLogContain: []string{"Invalid Argument"},
		},
		{
			name:           "negative op logs Invalid Argument",
			args:           []string{"-1"},
			wantLogContain: []string{"Invalid Argument"},
		},
		{
			name:           "op 5 logs InvalidMsg and stops",
			args:           []string{"5"},
			wantLogContain: []string{InvalidMsg},
			wantLogAbsent:  []string{"Process is started"},
		},
		{
			name:           "op 5 mid-sequence stops further processing",
			args:           []string{"0", "5", "0"},
			wantLogContain: []string{InvalidMsg},
		},
		{
			name:           "non-numeric returns error",
			args:           []string{"abc"},
			wantLogContain: []string{},
			errorIsOk:      false, // we expect an error specifically
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			r := NewRunner(newTestLogger(&buf))
			err := r.RunLazy(context.Background(), tc.args)
			logged := buf.String()

			for _, want := range tc.wantLogContain {
				assert.Contains(t, logged, want)
			}
			for _, absent := range tc.wantLogAbsent {
				assert.NotContains(t, logged, absent)
			}

			// For non-numeric we always want an error.
			if tc.name == "non-numeric returns error" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "parsing argument")
			}
		})
	}
}

func Test_Runner_RunLazy_MultipleOps_ProcessedInOrder(t *testing.T) {
	// Provide args 0, 0, 0 (all non-dispatchable, no DB needed) and confirm
	// each produces the separator line in order.
	var buf bytes.Buffer
	r := NewRunner(newTestLogger(&buf))
	err := r.RunLazy(context.Background(), []string{"0", "0", "0"})
	require.NoError(t, err)

	logged := buf.String()
	// Three "Invalid Argument" entries expected (one per arg).
	count := strings.Count(logged, "Invalid Argument")
	assert.Equal(t, 3, count)
}

func Test_Runner_RunLazy_OpGreaterThan4_BreaksLoop(t *testing.T) {
	// First arg is 0 (processed), second is 5 (breaks loop), third is 0 (never reached).
	var buf bytes