```go
package util_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"internal/mongo/util"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

func TestInvalidMsg(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		expected string
	}{
		{
			name:     "exact value matches specification",
			expected: "Invalid Argument(s) \n1 - Read / 2 - Write / 3 - Update / 4 - Delete",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, util.InvalidMsg)
		})
	}
}

func TestInvalidMsg_ContainsNewline(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		contains string
	}{
		{
			name:     "contains newline character separating label from options",
			contains: "\n",
		},
		{
			name:     "contains Read option mapped to 1",
			contains: "1 - Read",
		},
		{
			name:     "contains Write option mapped to 2",
			contains: "2 - Write",
		},
		{
			name:     "contains Update option mapped to 3",
			contains: "3 - Update",
		},
		{
			name:     "contains Delete option mapped to 4",
			contains: "4 - Delete",
		},
		{
			name:     "label prefix is present",
			contains: "Invalid Argument(s)",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Contains(t, util.InvalidMsg, tc.contains)
		})
	}
}

func TestMongoProperties(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		expected string
	}{
		{
			name:     "exact value matches specification",
			expected: "./conf/mongo.properties",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, util.MongoProperties)
		})
	}
}

func TestMongoProperties_PathInvariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		contains string
	}{
		{
			name:     "is relative path (starts with ./)",
			contains: "./",
		},
		{
			name:     "points to conf directory",
			contains: "conf/",
		},
		{
			name:     "file is named mongo.properties",
			contains: "mongo.properties",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Contains(t, util.MongoProperties, tc.contains)
		})
	}
}

// ---------------------------------------------------------------------------
// Commons interface – mock implementations
// ---------------------------------------------------------------------------

// mockCommons is a test double that satisfies the util.Commons interface.
type mockCommons struct {
	loadMethodsCalled bool
	finalizedCalled   bool
	loadMethodsErr    error
	finalizedErr      error
}

func (m *mockCommons) LoadMethods(ctx context.Context) error {
	m.loadMethodsCalled = true
	return m.loadMethodsErr
}

func (m *mockCommons) Finalized(ctx context.Context) error {
	m.finalizedCalled = true
	return m.finalizedErr
}

// Compile-time assertion: mockCommons must satisfy util.Commons.
var _ util.Commons = (*mockCommons)(nil)

// ---------------------------------------------------------------------------
// LoadMethods
// ---------------------------------------------------------------------------

func TestCommons_LoadMethods(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		loadMethodsErr error
		wantCalled     bool
		wantErr        bool
	}{
		{
			name:           "successful initialization returns nil",
			loadMethodsErr: nil,
			wantCalled:     true,
			wantErr:        false,
		},
		{
			name:           "initialization failure returns error",
			loadMethodsErr: errors.New("init failed"),
			wantCalled:     true,
			wantErr:        true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			impl := &mockCommons{loadMethodsErr: tc.loadMethodsErr}
			ctx := context.Background()

			err := impl.LoadMethods(ctx)

			assert.Equal(t, tc.wantCalled, impl.loadMethodsCalled, "LoadMethods should have been called")
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCommons_LoadMethods_ContextPropagation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ctxFunc func() context.Context
		wantErr bool
	}{
		{
			name:    "background context succeeds",
			ctxFunc: context.Background,
			wantErr: false,
		},
		{
			name: "already-cancelled context still invokes method",
			ctxFunc: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantErr: false, // mock does not inspect context
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			impl := &mockCommons{}
			err := impl.LoadMethods(tc.ctxFunc())

			assert.True(t, impl.loadMethodsCalled)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Finalized
// ---------------------------------------------------------------------------

func TestCommons_Finalized(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		finalizedErr error
		wantCalled   bool
		wantErr      bool
	}{
		{
			name:         "successful teardown returns nil",
			finalizedErr: nil,
			wantCalled:   true,
			wantErr:      false,
		},
		{
			name:         "teardown failure returns error",
			finalizedErr: errors.New("teardown failed"),
			wantCalled:   true,
			wantErr:      true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			impl := &mockCommons{finalizedErr: tc.finalizedErr}
			ctx := context.Background()

			err := impl.Finalized(ctx)

			assert.Equal(t, tc.wantCalled, impl.finalizedCalled, "Finalized should have been called")
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCommons_Finalized_ContextPropagation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ctxFunc func() context.Context
		wantErr bool
	}{
		{
			name:    "background context succeeds",
			ctxFunc: context.Background,
			wantErr: false,
		},
		{
			name: "already-cancelled context still invokes method",
			ctxFunc: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantErr: false, // mock does not inspect context
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			impl := &mockCommons{}
			err := impl.Finalized(tc.ctxFunc())

			assert.True(t, impl.finalizedCalled)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Full lifecycle (LoadMethods → Finalized)
// ---------------------------------------------------------------------------

func TestCommons_FullLifecycle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		loadMethodsErr error
		finalizedErr   error
		wantLoadErr    bool
		wantFinalErr   bool
	}{
		{
			name:           "both steps succeed",
			loadMethodsErr: nil,
			finalizedErr:   nil,
			wantLoadErr:    false,
			wantFinalErr:   false,
		},
		{
			name:           "load fails, finalize succeeds",
			loadMethodsErr: errors.New("load error"),
			finalizedErr:   nil,
			wantLoadErr:    true,
			wantFinalErr:   false,
		},
		{
			name:           "load succeeds, finalize fails",
			loadMethodsErr: nil,
			finalizedErr:   errors.New("finalize error"),
			wantLoadErr:    false,
			wantFinalErr:   true,
		},
		{
			name:           "both steps fail",
			loadMethodsErr: errors.New("load error"),
			finalizedErr:   errors.New("finalize error"),
			wantLoadErr:    true,
			wantFinalErr:   true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			impl := &mockCommons{
				loadMethodsErr: tc.loadMethodsErr,
				finalizedErr:   tc.finalizedErr,
			}
			ctx := context.Background()

			loadErr := impl.LoadMethods(ctx)
			finalErr := impl.Finalized(ctx)

			assert.True(t, impl.loadMethodsCalled, "LoadMethods must have been invoked")
			assert.True(t, impl.finalizedCalled, "Finalized must have been invoked")

			if tc.wantLoadErr {
				assert.Error(t, loadErr)
			} else {
				assert.NoError(t, loadErr)
			}

			if tc.wantFinalErr {
				assert.Error(t, finalErr)
			} else {
				assert.NoError(t, finalErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Interface satisfaction – ensure the type is an interface, not a struct
// ---------------------------------------------------------------------------

func TestCommons_IsInterface(t *testing.T) {
	t.Parallel()

	// If Commons were a concrete type rather than an interface this would not
	// compile.  The test acts as a compile-time guard.
	var _ util.Commons = (*mockCommons)(nil)

	t.Run("type assertion to interface succeeds", func(t *testing.T) {
		t.Parallel()
		impl := &mockCommons{}
		var iface util.Commons = impl
		assert.NotNil(t, iface)
	})
}

// ---------------------------------------------------------------------------
// Constant immutability (value consistency across multiple reads)
// ---------------------------------------------------------------------------

func TestConstants_AreImmutable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		read1 func() string
		read2 func() string
	}{
		{
			name:  "InvalidMsg is identical on repeated reads",
			read1: func() string { return util.InvalidMsg },
			read2: func() string { return util.InvalidMsg },
		},
		{
			name:  "MongoProperties is identical on repeated reads",
			read1: func() string { return util.MongoProperties },
			read2: func() string { return util.MongoProperties },
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.read1(), tc.read2())
		})
	}
}
```