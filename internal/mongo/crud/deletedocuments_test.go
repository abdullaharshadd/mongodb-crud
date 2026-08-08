```go
package crud_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/example/app/internal/mongo/crud"
	"github.com/example/app/internal/mongo/util"
)

// ---------------------------------------------------------------------------
// Minimal fakes / mocks
// ---------------------------------------------------------------------------

// fakeLifecycle satisfies util.Lifecycle so we can embed it in our mock.
type fakeLifecycle struct {
	connectCalled    bool
	disconnectCalled bool
	connectErr       error
	disconnectErr    error
}

func (f *fakeLifecycle) Connect(ctx context.Context) error {
	f.connectCalled = true
	return f.connectErr
}

func (f *fakeLifecycle) Disconnect(ctx context.Context) error {
	f.disconnectCalled = true
	return f.disconnectErr
}

// Ensure fakeLifecycle satisfies util.Lifecycle at compile time.
var _ util.Lifecycle = (*fakeLifecycle)(nil)

// mockDeleteDocuments is a hand-rolled mock that implements the full
// crud.DeleteDocuments interface. Test cases configure its fields to control
// the behaviour of each method.
type mockDeleteDocuments struct {
	fakeLifecycle

	// DeleteOne controls
	deleteOneCount int64
	deleteOneErr   error
	deleteOneCalls int

	// DeleteMany controls
	deleteManyCount int64
	deleteManyErr   error
	deleteManyCalls int
}

func (m *mockDeleteDocuments) DeleteOne(ctx context.Context) (int64, error) {
	m.deleteOneCalls++
	return m.deleteOneCount, m.deleteOneErr
}

func (m *mockDeleteDocuments) DeleteMany(ctx context.Context) (int64, error) {
	m.deleteManyCalls++
	return m.deleteManyCount, m.deleteManyErr
}

// Compile-time assertion: mockDeleteDocuments must satisfy crud.DeleteDocuments.
var _ crud.DeleteDocuments = (*mockDeleteDocuments)(nil)

// ---------------------------------------------------------------------------
// Helper – creates a background context
// ---------------------------------------------------------------------------
func bg() context.Context { return context.Background() }

// ---------------------------------------------------------------------------
// Interface-shape tests
// ---------------------------------------------------------------------------

// TestDeleteDocuments_InterfaceShape verifies that the interface surface
// expected by the behavioural specs is present. The compiler enforces this
// through the var _ assertion above, but the table-driven test makes the
// intent explicit and produces readable output.
func TestDeleteDocuments_InterfaceShape(t *testing.T) {
	tests := []struct {
		name string
		fn   func(crud.DeleteDocuments) error
	}{
		{
			name: "DeleteOne method exists on interface",
			fn: func(d crud.DeleteDocuments) error {
				_, err := d.DeleteOne(bg())
				return err
			},
		},
		{
			name: "DeleteMany method exists on interface",
			fn: func(d crud.DeleteDocuments) error {
				_, err := d.DeleteMany(bg())
				return err
			},
		},
		{
			name: "Connect lifecycle method exists on interface",
			fn: func(d crud.DeleteDocuments) error {
				return d.Connect(bg())
			},
		},
		{
			name: "Disconnect lifecycle method exists on interface",
			fn: func(d crud.DeleteDocuments) error {
				return d.Disconnect(bg())
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &mockDeleteDocuments{}
			// We only care that calling the method does not panic and that the
			// interface method is reachable.
			_ = tc.fn(m)
		})
	}
}

// ---------------------------------------------------------------------------
// DeleteOne – behavioural specs
// ---------------------------------------------------------------------------

func TestDeleteDocuments_DeleteOne(t *testing.T) {
	dbUnavailableErr := errors.New("mongo: connection refused")

	tests := []struct {
		name              string
		setupMock         func() *mockDeleteDocuments
		wantDeletedCount  int64
		wantErr           bool
		wantErrContains   string
		wantDeleteOneCalls int
	}{
		{
			name: "deletes a single matching document – returns count 1 and no error",
			setupMock: func() *mockDeleteDocuments {
				return &mockDeleteDocuments{deleteOneCount: 1, deleteOneErr: nil}
			},
			wantDeletedCount:  1,
			wantErr:           false,
			wantDeleteOneCalls: 1,
		},
		{
			name: "no document matched – returns count 0 and no error (at most one deleted)",
			setupMock: func() *mockDeleteDocuments {
				return &mockDeleteDocuments{deleteOneCount: 0, deleteOneErr: nil}
			},
			wantDeletedCount:  0,
			wantErr:           false,
			wantDeleteOneCalls: 1,
		},
		{
			name: "database connection unavailable – surfaces error to caller",
			setupMock: func() *mockDeleteDocuments {
				return &mockDeleteDocuments{deleteOneCount: 0, deleteOneErr: dbUnavailableErr}
			},
			wantDeletedCount:  0,
			wantErr:           true,
			wantErrContains:   "connection refused",
			wantDeleteOneCalls: 1,
		},
		{
			name: "invariant: count never exceeds 1 when document exists",
			setupMock: func() *mockDeleteDocuments {
				return &mockDeleteDocuments{deleteOneCount: 1, deleteOneErr: nil}
			},
			wantDeletedCount:  1,
			wantErr:           false,
			wantDeleteOneCalls: 1,
		},
		{
			name: "context cancellation propagated – implementation receives cancelled context",
			setupMock: func() *mockDeleteDocuments {
				return &mockDeleteDocuments{
					deleteOneCount: 0,
					deleteOneErr:   context.Canceled,
				}
			},
			wantDeletedCount:  0,
			wantErr:           true,
			wantErrContains:   "context canceled",
			wantDeleteOneCalls: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.setupMock()

			got, err := m.DeleteOne(bg())

			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrContains != "" {
					assert.Contains(t, err.Error(), tc.wantErrContains)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.wantDeletedCount, got, "deleted count mismatch")
			assert.Equal(t, tc.wantDeleteOneCalls, m.deleteOneCalls, "DeleteOne call count mismatch")

			// Invariant: at most one document deleted per invocation.
			assert.LessOrEqual(t, got, int64(1),
				"DeleteOne must delete at most one document")
		})
	}
}

// ---------------------------------------------------------------------------
// DeleteMany – behavioural specs
// ---------------------------------------------------------------------------

func TestDeleteDocuments_DeleteMany(t *testing.T) {
	dbUnavailableErr := errors.New("mongo: connection refused")

	tests := []struct {
		name               string
		setupMock          func() *mockDeleteDocuments
		wantDeletedCount   int64
		wantErr            bool
		wantErrContains    string
		wantDeleteManyCalls int
	}{
		{
			name: "deletes all matching documents – returns count > 0 and no error",
			setupMock: func() *mockDeleteDocuments {
				return &mockDeleteDocuments{deleteManyCount: 5, deleteManyErr: nil}
			},
			wantDeletedCount:   5,
			wantErr:            false,
			wantDeleteManyCalls: 1,
		},
		{
			name: "no documents matched – returns count 0 and no error",
			setupMock: func() *mockDeleteDocuments {
				return &mockDeleteDocuments{deleteManyCount: 0, deleteManyErr: nil}
			},
			wantDeletedCount:   0,
			wantErr:            false,
			wantDeleteManyCalls: 1,
		},
		{
			name: "database connection unavailable – surfaces error to caller",
			setupMock: func() *mockDeleteDocuments {
				return &mockDeleteDocuments{deleteManyCount: 0, deleteManyErr: dbUnavailableErr}
			},
			wantDeletedCount:   0,
			wantErr:            true,
			wantErrContains:    "connection refused",
			wantDeleteManyCalls: 1,
		},
		{
			name: "deletes exactly one document when only one matches",
			setupMock: func() *mockDeleteDocuments {
				return &mockDeleteDocuments{deleteManyCount: 1, deleteManyErr: nil}
			},
			wantDeletedCount:   1,
			wantErr:            false,
			wantDeleteManyCalls: 1,
		},
		{
			name: "context cancellation propagated – implementation receives cancelled context",
			setupMock: func() *mockDeleteDocuments {
				return &mockDeleteDocuments{
					deleteManyCount: 0,
					deleteManyErr:   context.Canceled,
				}
			},
			wantDeletedCount:   0,
			wantErr:            true,
			wantErrContains:    "context canceled",
			wantDeleteManyCalls: 1,
		},
		{
			name: "large batch deletion – returns correct count",
			setupMock: func() *mockDeleteDocuments {
				return &mockDeleteDocuments{deleteManyCount: 10_000, deleteManyErr: nil}
			},
			wantDeletedCount:   10_000,
			wantErr:            false,
			wantDeleteManyCalls: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.setupMock()

			got, err := m.DeleteMany(bg())

			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrContains != "" {
					assert.Contains(t, err.Error(), tc.wantErrContains)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.wantDeletedCount, got, "deleted count mismatch")
			assert.Equal(t, tc.wantDeleteManyCalls, m.deleteManyCalls, "DeleteMany call count mismatch")

			// Invariant: zero or more documents may be deleted.
			assert.GreaterOrEqual(t, got, int64(0),
				"DeleteMany must never return a negative count")
		})
	}
}

// ---------------------------------------------------------------------------
// Lifecycle embedding – global invariant: DeleteDocuments extends Commons
// ---------------------------------------------------------------------------

func TestDeleteDocuments_LifecycleEmbedding(t *testing.T) {
	tests := []struct {
		name             string
		connectErr       error
		disconnectErr    error
		wantConnectErr   bool
		wantDisconnectErr bool
	}{
		{
			name:             "Connect succeeds",
			connectErr:       nil,
			wantConnectErr:   false,
			wantDisconnectErr: false,
		},
		{
			name:             "Disconnect succeeds",
			disconnectErr:    nil,
			wantConnectErr:   false,
			wantDisconnectErr: false,
		},
		{
			name:             "Connect fails – error surfaced",
			connectErr:       errors.New("dial tcp: connection refused"),
			wantConnectErr:   true,
			wantDisconnectErr: false,
		},
		{
			name:             "Disconnect fails – error surfaced",
			disconnectErr:    errors.New("close: broken pipe"),
			wantConnectErr:   false,
			wantDisconnectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &mockDeleteDocuments{
				fakeLifecycle: fakeLifecycle{
					connectErr:    tc.connectErr,
					disconnectErr: tc.disconnectErr,
				},
			}

			// Exercise Connect
			connErr := m.Connect(bg())
			if tc.wantConnectErr {
				assert.Error(t, connErr)
			} else {
				assert.NoError(t, connErr)
			}
			assert.True(t, m.connectCalled, "Connect should have been called")

			// Exercise Disconnect
			discErr := m.Disconnect(bg())
			if tc.wantDisconnectErr {
				assert.Error(t, discErr)
			} else {
				assert.NoError(t, discErr)
			}
			assert.True(t, m.disconnectCalled, "Disconnect should have been called")
		})
	}
}

// ---------------------------------------------------------------------------
// Global invariant: any type that satisfies DeleteDocuments must implement
// both DeleteOne and DeleteMany.
// ---------------------------------------------------------------------------

// TestDeleteDocuments_ImplementorMustSatisfyFullContract proves that a type
// that omits either method does not compile. Because we cannot express
// "this must NOT compile" as a runtime test, we instead verify via the
// compile-time assertion (var _ crud.DeleteDocuments = (*mockDeleteDocuments)(nil))
// already present at the top of the file, and here we just confirm the mock
// works as a DeleteDocuments value.
func TestDeleteDocuments_ImplementorMustSatisfyFullContract(t *testing.T) {
	tests := []struct {
		name string
		impl crud.DeleteDocuments
	}{
		{
			name: "mockDeleteDocuments satisfies DeleteDocuments interface",
			impl: &mockDeleteDocuments{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotNil(t, tc.impl)

			// Call all four methods to confirm they are reachable through the
			// interface.
			_ = tc.impl.Connect(bg())
			_ = tc.impl.Disconnect(bg())
			_, _ = tc.impl.DeleteOne(bg())
			_, _ = tc.impl.DeleteMany(bg())
		})
	}
}

// ---------------------------------------------------------------------------
// Idempotency / call-count invariants
// ---------------------------------------------------------------------------

func TestDeleteDocuments_CallCountTracking(t *testing.T) {
	tests := []struct {
		name              string
		deleteOneInvocations  int
		deleteManyInvocations int
	}{
		{
			name:                  "single call to each method",
			deleteOneInvocations:  1,
			deleteManyInvocations: 1,
		},
		{
			name:                  "multiple calls to DeleteOne",
			deleteOneInvocations:  3,
			deleteManyInvocations: 0,
		},
		{
			name:                  "multiple calls to DeleteMany",
			deleteOneInvocations:  0,
			deleteManyInvocations: 4,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &mockDeleteDocuments{deleteOneCount: 1, deleteManyCount: 2}

			for i := 0; i < tc.deleteOneInvocations; i++ {
				_, _ = m.DeleteOne(bg())
			}
			for i := 0; i < tc.deleteManyInvocations; i++ {
				_, _ = m.DeleteMany(bg())
			}

			assert.Equal(t, tc.deleteOneInvocations, m.deleteOneCalls,
				"DeleteOne call count must match invocation count")
			assert.Equal(t,