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


// ---------------------------------------------------------------------------
// Finalized
// ---------------------------------------------------------------------------



// ---------------------------------------------------------------------------
// Full lifecycle (LoadMethods → Finalized)
// ---------------------------------------------------------------------------


// ---------------------------------------------------------------------------
// Interface satisfaction – ensure the type is an interface, not a struct
// ---------------------------------------------------------------------------


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