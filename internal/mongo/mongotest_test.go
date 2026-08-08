```go
package mongo

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newTestLogger returns a *slog.Logger that writes into buf so tests can
// inspect emitted log lines.
func newTestLogger(buf *bytes.Buffer) *slog.Logger {
	handler := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(handler)
}

// logContains is a small helper that checks whether the logged text contains
// the given substring (case-sensitive).
func logContains(buf *bytes.Buffer, substr string) bool {
	return strings.Contains(buf.String(), substr)
}

// ---------------------------------------------------------------------------
// Tests for validateArgs
// ---------------------------------------------------------------------------

func TestValidateArgs(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantResult bool
		wantLog    string // non-empty ⟹ that substring must appear in the log
	}{
		{
			name:       "all valid single arg 1",
			args:       []string{"1"},
			wantResult: true,
		},
		{
			name:       "all valid single arg 4",
			args:       []string{"4"},
			wantResult: true,
		},
		{
			name:       "all valid multiple args",
			args:       []string{"1", "2", "3", "4"},
			wantResult: true,
		},
		{
			name:       "arg exactly 4 is valid",
			args:       []string{"4"},
			wantResult: true,
		},
		{
			name:       "arg greater than 4 is invalid",
			args:       []string{"5"},
			wantResult: false,
		},
		{
			name:       "mix of valid and invalid (>4) returns false",
			args:       []string{"1", "2", "5"},
			wantResult: false,
		},
		{
			name:       "non-numeric arg returns false and logs",
			args:       []string{"abc"},
			wantResult: false,
			wantLog:    "not a number",
		},
		{
			name:       "non-numeric mixed with valid",
			args:       []string{"1", "xyz"},
			wantResult: false,
			wantLog:    "not a number",
		},
		{
			name:       "empty args slice is valid (nothing to reject)",
			args:       []string{},
			wantResult: true,
		},
		{
			name:       "zero is valid (<=4)",
			args:       []string{"0"},
			wantResult: true,
		},
		{
			name:       "negative value is valid (<=4)",
			args:       []string{"-1"},
			wantResult: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := newTestLogger(&buf)

			got := validateArgs(tc.args, logger)
			assert.Equal(t, tc.wantResult, got, "validateArgs return value mismatch")

			if tc.wantLog != "" {
				assert.True(t, logContains(&buf, tc.wantLog),
					"expected log to contain %q, got: %s", tc.wantLog, buf.String())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for Run (top-level entry point)
// ---------------------------------------------------------------------------

func TestRun_InvalidArgLogged(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantLog     string
		wantNoOp    bool // true ⟹ no operation separator should appear
		wantErrNil  bool
	}{
		{
			name:       "arg > 4 logs invalid and returns nil",
			args:       []string{"9"},
			wantLog:    invalidMsg,
			wantNoOp:   true,
			wantErrNil: true,
		},
		{
			name:       "non-numeric arg logs invalid and returns nil",
			args:       []string{"bad"},
			wantLog:    "not a number",
			wantNoOp:   true,
			wantErrNil: true,
		},
		{
			name:       "mix valid and >4 logs invalid",
			args:       []string{"1", "5"},
			wantLog:    invalidMsg,
			wantNoOp:   true,
			wantErrNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := newTestLogger(&buf)

			err := Run(context.Background(), tc.args, logger)

			if tc.wantErrNil {
				assert.NoError(t, err)
			}
			if tc.wantLog != "" {
				assert.True(t, logContains(&buf, tc.wantLog),
					"expected log to contain %q, got: %s", tc.wantLog, buf.String())
			}
			if tc.wantNoOp {
				assert.False(t, logContains(&buf, separator+" msg="),
					"expected no operation separator but found one in: %s", buf.String())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for startProcess (dispatch loop)
// ---------------------------------------------------------------------------

// dispatchTestCase describes one scenario for startProcess / dispatch.
type dispatchTestCase struct {
	name           string
	args           []string
	wantLogs       []string  // all these substrings must appear
	wantAbsentLogs []string  // none of these should appear
}

func TestStartProcess(t *testing.T) {
	// NOTE: because impl.NewXxx constructors reach out to a real MongoDB
	// connection (not available in unit tests), we only verify the log
	// behaviour up to the point where dispatch() calls LoadMethods().
	// The error from LoadMethods() is logged but does not abort the loop.
	// We therefore check that the "process started" messages appear.

	tests := []dispatchTestCase{
		{
			name: "op 1 logs read process started",
			args: []string{"1"},
			wantLogs: []string{
				separator,
				"Read Process is started...",
			},
		},
		{
			name: "op 2 logs write process started",
			args: []string{"2"},
			wantLogs: []string{
				separator,
				"Write Process is started...",
			},
		},
		{
			name: "op 3 logs update process started",
			args: []string{"3"},
			wantLogs: []string{
				separator,
				"Update Process is started...",
			},
		},
		{
			name: "op 4 logs delete process started",
			args: []string{"4"},
			wantLogs: []string{
				separator,
				"Delete Process is started...",
			},
		},
		{
			name: "op 0 logs invalid argument",
			args: []string{"0"},
			wantLogs: []string{
				separator,
				"Invalid Argument : 0",
			},
		},
		{
			name: "negative op logs invalid argument",
			args: []string{"-1"},
			wantLogs: []string{
				"Invalid Argument : -1",
			},
		},
		{
			name: "multiple valid ops are all dispatched",
			args: []string{"1", "2", "3", "4"},
			wantLogs: []string{
				"Read Process is started...",
				"Write Process is started...",
				"Update Process is started...",
				"Delete Process is started...",
			},
		},
		{
			name: "op > 4 mid-loop aborts remaining",
			args: []string{"1", "5", "2"}, // "2" should NOT be reached
			wantLogs: []string{
				"Read Process is started...",
				invalidMsg,
			},
			wantAbsentLogs: []string{
				"Write Process is started...",
			},
		},
		{
			name: "single op > 4 logs invalid immediately",
			args: []string{"5"},
			wantLogs: []string{
				invalidMsg,
			},
			wantAbsentLogs: []string{
				"Read Process is started...",
				"Write Process is started...",
				"Update Process is started...",
				"Delete Process is started...",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := newTestLogger(&buf)

			err := startProcess(context.Background(), tc.args, logger)
			assert.NoError(t, err, "startProcess should never return a non-nil error")

			logged := buf.String()
			for _, want := range tc.wantLogs {
				assert.True(t, strings.Contains(logged, want),
					"expected log to contain %q\nfull log:\n%s", want, logged)
			}
			for _, absent := range tc.wantAbsentLogs {
				assert.False(t, strings.Contains(logged, absent),
					"expected log NOT to contain %q\nfull log:\n%s", absent, logged)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for dispatch (unit-level, using a stub-friendly wrapper)
// ---------------------------------------------------------------------------

// dispatchable is an interface that allows us to stub out the real impl
// constructors in dispatch tests.
type dispatchable interface {
	LoadMethods(ctx context.Context) error
}

// stubDispatchable always returns the configured error (or nil).
type stubDispatchable struct {
	err error
}

func (s *stubDispatchable) LoadMethods(_ context.Context) error {
	return s.err
}

// dispatchWithStub is a copy of the dispatch switch that accepts injected
// dispatchable instances rather than constructing real impls. This lets us
// test the dispatch routing and error-handling logic without a live MongoDB.
func dispatchWithStub(ctx context.Context, op operation, logger *slog.Logger,
	readImpl, writeImpl, updateImpl, deleteImpl dispatchable) error {
	switch op {
	case opRead:
		logger.Info("Read Process is started...")
		logger.Info(separator)
		return readImpl.LoadMethods(ctx)
	case opWrite:
		logger.Info("Write Process is started...")
		logger.Info(separator)
		return writeImpl.LoadMethods(ctx)
	case opUpdate:
		logger.Info("Update Process is started...")
		logger.Info(separator)
		return updateImpl.LoadMethods(ctx)
	case opDelete:
		logger.Info("Delete Process is started...")
		logger.Info(separator)
		return deleteImpl.LoadMethods(ctx)
	default:
		logger.Info("Invalid Argument : " + strconv.Itoa(int(op)))
		logger.Info(separator)
		return nil
	}
}

// strconv import needed inside dispatchWithStub above.
var _ = strconv.Itoa // ensure import is used

func TestDispatchWithStub(t *testing.T) {
	successStub := &stubDispatchable{err: nil}
	failStub := &stubDispatchable{err: errors.New("connection refused")}

	tests := []struct {
		name        string
		op          operation
		readImpl    dispatchable
		writeImpl   dispatchable
		updateImpl  dispatchable
		deleteImpl  dispatchable
		wantLog     string
		wantErrNil  bool
	}{
		{
			name:       "op 1 read - success",
			op:         opRead,
			readImpl:   successStub,
			writeImpl:  successStub,
			updateImpl: successStub,
			deleteImpl: successStub,
			wantLog:    "Read Process is started...",
			wantErrNil: true,
		},
		{
			name:       "op 1 read - error propagated",
			op:         opRead,
			readImpl:   failStub,
			writeImpl:  successStub,
			updateImpl: successStub,
			deleteImpl: successStub,
			wantLog:    "Read Process is started...",
			wantErrNil: false,
		},
		{
			name:       "op 2 write - success",
			op:         opWrite,
			readImpl:   successStub,
			writeImpl:  successStub,
			updateImpl: successStub,
			deleteImpl: successStub,
			wantLog:    "Write Process is started...",
			wantErrNil: true,
		},
		{
			name:       "op 2 write - error propagated",
			op:         opWrite,
			readImpl:   successStub,
			writeImpl:  failStub,
			updateImpl: successStub,
			deleteImpl: successStub,
			wantLog:    "Write Process is started...",
			wantErrNil: false,
		},
		{
			name:       "op 3 update - success",
			op:         opUpdate,
			readImpl:   successStub,
			writeImpl:  successStub,
			updateImpl: successStub,
			deleteImpl: successStub,
			wantLog:    "Update Process is started...",
			wantErrNil: true,
		},
		{
			name:       "op 3 update - error propagated",
			op:         opUpdate,
			readImpl:   successStub,
			writeImpl:  successStub,
			updateImpl: failStub,
			deleteImpl: successStub,
			wantLog:    "Update Process is started...",
			wantErrNil: false,
		},
		{
			name:       "op 4 delete - success",
			op:         opDelete,
			readImpl:   successStub,
			writeImpl:  successStub,
			updateImpl: successStub,
			deleteImpl: successStub,
			wantLog:    "Delete Process is started...",
			wantErrNil: true,
		},
		{
			name:       "op 4 delete - error propagated",
			op:         opDelete,
			readImpl:   successStub,
			writeImpl:  successStub,
			updateImpl: successStub,
			deleteImpl: failStub,
			wantLog:    "Delete Process is started...",
			wantErrNil: false,
		},
		{
			name:       "op 0 - default invalid argument",
			op:         operation(0),
			readImpl:   successStub,
			writeImpl:  successStub,
			updateImpl: successStub,
			deleteImpl: successStub,
			wantLog:    "Invalid Argument : 0",
			wantErrNil: true,
		},
		{
			name:       "op -1 - default invalid argument",
			op:         operation(-1),
			readImpl:   successStub,
			writeImpl:  successStub,
			updateImpl: successStub,
			deleteImpl: successStub,
			wantLog:    "Invalid Argument : -1",
			wantErrNil: true,
		},
		{
			name:       "op 99 - default invalid argument",
			op:         operation(99),
			readImpl:   successStub,
			writeImpl:  successStub,
			updateImpl: successStub,
			deleteImpl: successStub,
			wantLog:    "Invalid Argument : 99",
			wantErrNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := newTestLogger(&buf)

			err := dispatchWithStub(context.Background(), tc.op, logger,
				tc.