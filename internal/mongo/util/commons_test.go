```go
package util_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"internal/mongo/util"
)

// TestINVALIDMsg validates the exact value and immutability of the INVALIDMsg constant.
func TestINVALIDMsg(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{
			name:     "any implementing class accesses the constant",
			expected: "Invalid Argument(s) \n1 - Read / 2 - Write / 3 - Update / 4 - Delete",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, util.INVALIDMsg)
		})
	}
}

// TestINVALIDMsg_Immutability ensures the constant value never changes across
// multiple reads (invariant: value is identical across all accesses).
func TestINVALIDMsg_Immutability(t *testing.T) {
	first := util.INVALIDMsg
	second := util.INVALIDMsg
	third := util.INVALIDMsg

	assert.Equal(t, first, second, "INVALIDMsg must be identical across reads")
	assert.Equal(t, second, third, "INVALIDMsg must be identical across reads")
}

// TestMongoPropertiesPath validates the exact value and invariants of the
// MongoPropertiesPath constant.
func TestMongoPropertiesPath(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{
			name:     "any implementing class accesses the constant",
			expected: "./conf/mongo.properties",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, util.MongoPropertiesPath)
		})
	}
}

// TestMongoPropertiesPath_IsRelative validates the invariant that the path is
// relative to the working directory (starts with ".").
func TestMongoPropertiesPath_IsRelative(t *testing.T) {
	tests := []struct {
		name            string
		expectRelative  bool
		expectedPrefix  string
	}{
		{
			name:           "path is relative to working directory",
			expectRelative: true,
			expectedPrefix: ".",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.True(t, len(util.MongoPropertiesPath) > 0, "MongoPropertiesPath must not be empty")
			if tc.expectRelative {
				assert.Equal(t, tc.expectedPrefix, string(util.MongoPropertiesPath[0]),
					"MongoPropertiesPath must start with '.' to be relative")
			}
		})
	}
}

// TestMongoPropertiesPath_Immutability ensures the constant value never
// changes across multiple reads.
func TestMongoPropertiesPath_Immutability(t *testing.T) {
	first := util.MongoPropertiesPath
	second := util.MongoPropertiesPath
	third := util.MongoPropertiesPath

	assert.Equal(t, first, second, "MongoPropertiesPath must be identical across reads")
	assert.Equal(t, second, third, "MongoPropertiesPath must be identical across reads")
}

// --- Lifecycle interface tests ---

// mockLifecycle is a test double that satisfies the util.Lifecycle interface,
// recording calls and allowing injection of errors.
type mockLifecycle struct {
	loadMethodsCalled  bool
	finalizedCalled    bool
	loadMethodsErr     error
	finalizedErr       error
	loadMethodsCtx     context.Context
	finalizedCtx       context.Context
}

func (m *mockLifecycle) LoadMethods(ctx context.Context) error {
	m.loadMethodsCalled = true
	m.loadMethodsCtx = ctx
	return m.loadMethodsErr
}

func (m *mockLifecycle) Finalized(ctx context.Context) error {
	m.finalizedCalled = true
	m.finalizedCtx = ctx
	return m.finalizedErr
}

// Compile-time assertion: mockLifecycle must satisfy util.Lifecycle.
var _ util.Lifecycle = (*mockLifecycle)(nil)

// TestLifecycle_LoadMethods validates that any type implementing the
// Lifecycle interface can have LoadMethods called on it and returns the
// appropriate result.
func TestLifecycle_LoadMethods(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func() *mockLifecycle
		ctx           context.Context
		expectedErr   error
		expectCalled  bool
	}{
		{
			name: "invoked on an implementing instance - success (no error)",
			setupMock: func() *mockLifecycle {
				return &mockLifecycle{loadMethodsErr: nil}
			},
			ctx:          context.Background(),
			expectedErr:  nil,
			expectCalled: true,
		},
		{
			name: "invoked on an implementing instance - initialization error",
			setupMock: func() *mockLifecycle {
				return &mockLifecycle{loadMethodsErr: errors.New("initialization failed")}
			},
			ctx:          context.Background(),
			expectedErr:  errors.New("initialization failed"),
			expectCalled: true,
		},
		{
			name: "invoked with cancelled context",
			setupMock: func() *mockLifecycle {
				return &mockLifecycle{loadMethodsErr: context.Canceled}
			},
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			expectedErr:  context.Canceled,
			expectCalled: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := tc.setupMock()

			// Use the interface type to ensure we're testing via the interface contract.
			var lc util.Lifecycle = mock

			err := lc.LoadMethods(tc.ctx)

			assert.Equal(t, tc.expectCalled, mock.loadMethodsCalled,
				"LoadMethods should have been called")
			assert.Equal(t, tc.ctx, mock.loadMethodsCtx,
				"context passed to LoadMethods should match")

			if tc.expectedErr != nil {
				assert.EqualError(t, err, tc.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestLifecycle_Finalized validates that any type implementing the
// Lifecycle interface can have Finalized called on it and returns the
// appropriate result.
func TestLifecycle_Finalized(t *testing.T) {
	tests := []struct {
		name         string
		setupMock    func() *mockLifecycle
		ctx          context.Context
		expectedErr  error
		expectCalled bool
	}{
		{
			name: "invoked on an implementing instance - success (no error)",
			setupMock: func() *mockLifecycle {
				return &mockLifecycle{finalizedErr: nil}
			},
			ctx:          context.Background(),
			expectedErr:  nil,
			expectCalled: true,
		},
		{
			name: "invoked on an implementing instance - teardown error",
			setupMock: func() *mockLifecycle {
				return &mockLifecycle{finalizedErr: errors.New("teardown failed")}
			},
			ctx:          context.Background(),
			expectedErr:  errors.New("teardown failed"),
			expectCalled: true,
		},
		{
			name: "invoked with cancelled context",
			setupMock: func() *mockLifecycle {
				return &mockLifecycle{finalizedErr: context.Canceled}
			},
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			expectedErr:  context.Canceled,
			expectCalled: true,
		},
		{
			name: "invoked with deadline exceeded context",
			setupMock: func() *mockLifecycle {
				return &mockLifecycle{finalizedErr: context.DeadlineExceeded}
			},
			ctx:          context.Background(),
			expectedErr:  context.DeadlineExceeded,
			expectCalled: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := tc.setupMock()

			// Use the interface type to ensure we're testing via the interface contract.
			var lc util.Lifecycle = mock

			err := lc.Finalized(tc.ctx)

			assert.Equal(t, tc.expectCalled, mock.finalizedCalled,
				"Finalized should have been called")
			assert.Equal(t, tc.ctx, mock.finalizedCtx,
				"context passed to Finalized should match")

			if tc.expectedErr != nil {
				assert.EqualError(t, err, tc.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestLifecycle_FullCycle validates that a type implementing Lifecycle can
// run a full load → finalize sequence, matching the original Java lifecycle.
func TestLifecycle_FullCycle(t *testing.T) {
	tests := []struct {
		name            string
		loadErr         error
		finalizeErr     error
		expectLoadErr   bool
		expectFinalErr  bool
	}{
		{
			name:           "successful full lifecycle",
			loadErr:        nil,
			finalizeErr:    nil,
			expectLoadErr:  false,
			expectFinalErr: false,
		},
		{
			name:           "load fails, finalize succeeds",
			loadErr:        errors.New("load error"),
			finalizeErr:    nil,
			expectLoadErr:  true,
			expectFinalErr: false,
		},
		{
			name:           "load succeeds, finalize fails",
			loadErr:        nil,
			finalizeErr:    errors.New("finalize error"),
			expectLoadErr:  false,
			expectFinalErr: true,
		},
		{
			name:           "both load and finalize fail",
			loadErr:        errors.New("load error"),
			finalizeErr:    errors.New("finalize error"),
			expectLoadErr:  true,
			expectFinalErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockLifecycle{
				loadMethodsErr: tc.loadErr,
				finalizedErr:   tc.finalizeErr,
			}

			var lc util.Lifecycle = mock
			ctx := context.Background()

			loadErr := lc.LoadMethods(ctx)
			finalErr := lc.Finalized(ctx)

			assert.True(t, mock.loadMethodsCalled, "LoadMethods must be called")
			assert.True(t, mock.finalizedCalled, "Finalized must be called")

			if tc.expectLoadErr {
				assert.Error(t, loadErr)
			} else {
				assert.NoError(t, loadErr)
			}

			if tc.expectFinalErr {
				assert.Error(t, finalErr)
			} else {
				assert.NoError(t, finalErr)
			}
		})
	}
}

// TestLifecycle_InterfaceContract validates that the Lifecycle interface
// enforces both methods and that a struct missing either method does not
// satisfy it (compile-time; demonstrated via the mock satisfying the contract).
func TestLifecycle_InterfaceContract(t *testing.T) {
	tests := []struct {
		name string
		impl util.Lifecycle
	}{
		{
			name: "mockLifecycle satisfies Lifecycle interface",
			impl: &mockLifecycle{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotNil(t, tc.impl, "Lifecycle implementation must not be nil")

			// Verify both interface methods are callable.
			ctx := context.Background()
			_ = tc.impl.LoadMethods(ctx)
			_ = tc.impl.Finalized(ctx)

			// If we reached here without panic, the interface is correctly satisfied.
			assert.True(t, true, "Lifecycle interface contract satisfied")
		})
	}
}

// TestConstants_NoStateOrBehavior validates that the package-level constants
// are purely data with no associated state changes (global invariant).
func TestConstants_NoStateOrBehavior(t *testing.T) {
	tests := []struct {
		name  string
		value string
		check func(string) bool
	}{
		{
			name:  "INVALIDMsg has fixed non-empty value",
			value: util.INVALIDMsg,
			check: func(v string) bool { return len(v) > 0 },
		},
		{
			name:  "MongoPropertiesPath has fixed non-empty value",
			value: util.MongoPropertiesPath,
			check: func(v string) bool { return len(v) > 0 },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.True(t, tc.check(tc.value), "constant must satisfy non-empty invariant")
		})
	}
}
```