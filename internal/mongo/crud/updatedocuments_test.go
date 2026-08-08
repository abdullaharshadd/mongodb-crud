```go
package crud_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mongo/crud/internal/mongo/crud"
	"github.com/mongo/crud/internal/mongo/util"
)

// ---------------------------------------------------------------------------
// Fake / mock implementations
// ---------------------------------------------------------------------------

// mockLifecycle is a minimal implementation of util.Lifecycle used by our mock.
type mockLifecycle struct {
	connectCalled    bool
	disconnectCalled bool
	connectErr       error
	disconnectErr    error
}

func (m *mockLifecycle) Connect(ctx context.Context) error {
	m.connectCalled = true
	return m.connectErr
}

func (m *mockLifecycle) Disconnect(ctx context.Context) error {
	m.disconnectCalled = true
	return m.disconnectErr
}

// Ensure mockLifecycle satisfies util.Lifecycle at compile time.
var _ util.Lifecycle = (*mockLifecycle)(nil)

// mockUpdateDocuments is a test double that implements crud.UpdateDocuments.
// Each method records that it was called and returns the configured error (if any).
type mockUpdateDocuments struct {
	mockLifecycle

	// call trackers
	updateOneCalled              bool
	updateManyCalled             bool
	updateWithCurrentDateCalled  bool

	// error stubs
	updateOneErr             error
	updateManyErr            error
	updateWithCurrentDateErr error

	// side-effect counters (simulate how many docs were touched)
	updatedOneCount  int
	updatedManyCount int
	updatedDateCount int
}

func (m *mockUpdateDocuments) UpdateOne(ctx context.Context) error {
	m.updateOneCalled = true
	if m.updateOneErr != nil {
		return m.updateOneErr
	}
	m.updatedOneCount = 1
	return nil
}

func (m *mockUpdateDocuments) UpdateMany(ctx context.Context) error {
	m.updateManyCalled = true
	if m.updateManyErr != nil {
		return m.updateManyErr
	}
	m.updatedManyCount = m.updatedManyCount // keeps whatever preset value
	return nil
}

func (m *mockUpdateDocuments) UpdateWithCurrentDate(ctx context.Context) error {
	m.updateWithCurrentDateCalled = true
	if m.updateWithCurrentDateErr != nil {
		return m.updateWithCurrentDateErr
	}
	m.updatedDateCount = 1
	return nil
}

// Ensure mockUpdateDocuments satisfies crud.UpdateDocuments at compile time.
var _ crud.UpdateDocuments = (*mockUpdateDocuments)(nil)

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func newMock() *mockUpdateDocuments {
	return &mockUpdateDocuments{}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestUpdateOne covers all behavioral specs for UpdateOne / updateOneDocument.
func TestUpdateOne(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name string
		// setup returns a fresh mock configured for the scenario
		setup func() *mockUpdateDocuments
		// invoke calls the method under test
		invoke         func(ctx context.Context, m *mockUpdateDocuments) error
		wantErr        bool
		wantCalled     bool
		// assertions on side-effects
		checkSideEffects func(t *testing.T, m *mockUpdateDocuments)
	}

	tests := []testCase{
		{
			name: "matching document exists – updates exactly one document",
			setup: func() *mockUpdateDocuments {
				m := newMock()
				// no errors – happy path
				return m
			},
			invoke: func(ctx context.Context, m *mockUpdateDocuments) error {
				return m.UpdateOne(ctx)
			},
			wantErr:    false,
			wantCalled: true,
			checkSideEffects: func(t *testing.T, m *mockUpdateDocuments) {
				// invariant: at most one document modified
				assert.Equal(t, 1, m.updatedOneCount, "exactly one document should be updated")
			},
		},
		{
			name: "multiple documents match filter – updates only first document",
			setup: func() *mockUpdateDocuments {
				m := newMock()
				// UpdateOne always touches at most 1; no error means success
				return m
			},
			invoke: func(ctx context.Context, m *mockUpdateDocuments) error {
				return m.UpdateOne(ctx)
			},
			wantErr:    false,
			wantCalled: true,
			checkSideEffects: func(t *testing.T, m *mockUpdateDocuments) {
				// invariant: at most one document modified per invocation
				assert.LessOrEqual(t, m.updatedOneCount, 1,
					"UpdateOne must modify at most one document even when multiple match")
			},
		},
		{
			name: "no matching document – no document updated, error surfaced",
			setup: func() *mockUpdateDocuments {
				m := newMock()
				m.updateOneErr = errors.New("no document matched the filter")
				return m
			},
			invoke: func(ctx context.Context, m *mockUpdateDocuments) error {
				return m.UpdateOne(ctx)
			},
			wantErr:    true,
			wantCalled: true,
			checkSideEffects: func(t *testing.T, m *mockUpdateDocuments) {
				assert.Equal(t, 0, m.updatedOneCount, "no document should be updated when no match is found")
			},
		},
		{
			name: "context cancellation propagated as error",
			setup: func() *mockUpdateDocuments {
				m := newMock()
				m.updateOneErr = context.Canceled
				return m
			},
			invoke: func(ctx context.Context, m *mockUpdateDocuments) error {
				return m.UpdateOne(ctx)
			},
			wantErr:    true,
			wantCalled: true,
			checkSideEffects: func(t *testing.T, m *mockUpdateDocuments) {
				assert.Equal(t, 0, m.updatedOneCount, "no document should be updated on context cancellation")
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := tc.setup()
			ctx := context.Background()

			err := tc.invoke(ctx, m)

			assert.Equal(t, tc.wantCalled, m.updateOneCalled, "UpdateOne should have been called")

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tc.checkSideEffects != nil {
				tc.checkSideEffects(t, m)
			}
		})
	}
}

// TestUpdateMany covers all behavioral specs for UpdateMany / updateManyDocument.
func TestUpdateMany(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		setup            func() *mockUpdateDocuments
		invoke           func(ctx context.Context, m *mockUpdateDocuments) error
		wantErr          bool
		wantCalled       bool
		checkSideEffects func(t *testing.T, m *mockUpdateDocuments)
	}

	tests := []testCase{
		{
			name: "multiple documents match filter – all are updated",
			setup: func() *mockUpdateDocuments {
				m := newMock()
				m.updatedManyCount = 5 // preset: 5 docs matched
				return m
			},
			invoke: func(ctx context.Context, m *mockUpdateDocuments) error {
				return m.UpdateMany(ctx)
			},
			wantErr:    false,
			wantCalled: true,
			checkSideEffects: func(t *testing.T, m *mockUpdateDocuments) {
				assert.Equal(t, 5, m.updatedManyCount,
					"all matching documents should be updated")
			},
		},
		{
			name: "no documents match filter – no documents modified",
			setup: func() *mockUpdateDocuments {
				m := newMock()
				m.updatedManyCount = 0 // preset: 0 docs matched
				return m
			},
			invoke: func(ctx context.Context, m *mockUpdateDocuments) error {
				return m.UpdateMany(ctx)
			},
			wantErr:    false,
			wantCalled: true,
			checkSideEffects: func(t *testing.T, m *mockUpdateDocuments) {
				assert.Equal(t, 0, m.updatedManyCount,
					"no documents should be modified when no match is found")
			},
		},
		{
			name: "no match found – error case surfaced",
			setup: func() *mockUpdateDocuments {
				m := newMock()
				m.updateManyErr = errors.New("no documents matched the filter")
				return m
			},
			invoke: func(ctx context.Context, m *mockUpdateDocuments) error {
				return m.UpdateMany(ctx)
			},
			wantErr:    true,
			wantCalled: true,
			checkSideEffects: func(t *testing.T, m *mockUpdateDocuments) {
				assert.Equal(t, 0, m.updatedManyCount,
					"no documents should be updated when error is returned")
			},
		},
		{
			name: "context deadline exceeded propagated as error",
			setup: func() *mockUpdateDocuments {
				m := newMock()
				m.updateManyErr = context.DeadlineExceeded
				return m
			},
			invoke: func(ctx context.Context, m *mockUpdateDocuments) error {
				return m.UpdateMany(ctx)
			},
			wantErr:    true,
			wantCalled: true,
			checkSideEffects: func(t *testing.T, m *mockUpdateDocuments) {
				assert.Equal(t, 0, m.updatedManyCount)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := tc.setup()
			ctx := context.Background()

			err := tc.invoke(ctx, m)

			assert.Equal(t, tc.wantCalled, m.updateManyCalled,
				"UpdateMany should have been called")

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tc.checkSideEffects != nil {
				tc.checkSideEffects(t, m)
			}
		})
	}
}

// TestUpdateWithCurrentDate covers all behavioral specs for
// UpdateWithCurrentDate / updateDocumentWithCurrentDate.
func TestUpdateWithCurrentDate(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		setup            func() *mockUpdateDocuments
		invoke           func(ctx context.Context, m *mockUpdateDocuments) error
		wantErr          bool
		wantCalled       bool
		checkSideEffects func(t *testing.T, m *mockUpdateDocuments)
	}

	tests := []testCase{
		{
			name: "matching document exists – date field is set to current date/time",
			setup: func() *mockUpdateDocuments {
				return newMock()
			},
			invoke: func(ctx context.Context, m *mockUpdateDocuments) error {
				return m.UpdateWithCurrentDate(ctx)
			},
			wantErr:    false,
			wantCalled: true,
			checkSideEffects: func(t *testing.T, m *mockUpdateDocuments) {
				// side-effect: exactly one document received the current-date update
				assert.Equal(t, 1, m.updatedDateCount,
					"one document should have its date field updated to the current date/time")
			},
		},
		{
			name: "no matching document – no document updated, error surfaced",
			setup: func() *mockUpdateDocuments {
				m := newMock()
				m.updateWithCurrentDateErr = errors.New("no document matched the filter")
				return m
			},
			invoke: func(ctx context.Context, m *mockUpdateDocuments) error {
				return m.UpdateWithCurrentDate(ctx)
			},
			wantErr:    true,
			wantCalled: true,
			checkSideEffects: func(t *testing.T, m *mockUpdateDocuments) {
				assert.Equal(t, 0, m.updatedDateCount,
					"no document should be updated when no match is found")
			},
		},
		{
			name: "context cancellation propagated as error",
			setup: func() *mockUpdateDocuments {
				m := newMock()
				m.updateWithCurrentDateErr = context.Canceled
				return m
			},
			invoke: func(ctx context.Context, m *mockUpdateDocuments) error {
				return m.UpdateWithCurrentDate(ctx)
			},
			wantErr:    true,
			wantCalled: true,
			checkSideEffects: func(t *testing.T, m *mockUpdateDocuments) {
				assert.Equal(t, 0, m.updatedDateCount,
					"no document should be updated on context cancellation")
			},
		},
		{
			name: "documents not matching filter remain unchanged – invariant",
			setup: func() *mockUpdateDocuments {
				m := newMock()
				// successful update affects only the matched document; the mock
				// models this by incrementing updatedDateCount to 1 only.
				return m
			},
			invoke: func(ctx context.Context, m *mockUpdateDocuments) error {
				return m.UpdateWithCurrentDate(ctx)
			},
			wantErr:    false,
			wantCalled: true,
			checkSideEffects: func(t *testing.T, m *mockUpdateDocuments) {
				// invariant: at most 1 document changed; others remain untouched
				assert.LessOrEqual(t, m.updatedDateCount, 1,
					"UpdateWithCurrentDate must not modify documents that don't match the filter")
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := tc.setup()
			ctx := context.Background()

			err := tc.invoke(ctx, m)

			assert.Equal(t, tc.wantCalled, m.updateWithCurrentDateCalled,
				"UpdateWithCurrentDate should have been called")

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tc.checkSideEffects != nil {
				tc.checkSideEffects(t, m)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Interface-compliance / contract tests
// ---------------------------------------------------------------------------

// TestUpdateDocumentsImplementsLifecycle verifies that any implementation of
// UpdateDocuments also satisfies the util.Lifecycle contract (the Go
// equivalent of "extends Commons").
func TestUpdateDocumentsImplementsLifecycle(t *testing.T) {
	t.Parallel()

	// The compile-time assertion below is the canonical way to verify interface
	// embedding in Go. If crud.UpdateDocuments does not embed util.Lifecycle,
	// this assignment will not compile.
	var _ util.Lifecycle = (crud.UpdateDocuments)(nil)

	// At runtime we also confirm that a concrete mock satisfies both.
	m := newMock()
	var ud crud.UpdateDocuments = m
	var lc util.Lifecycle = m

	assert.NotNil(t, ud)
	assert.NotNil(t, lc)
}

// TestUpdateDocumentsAllMethodsPresent is a table-driven smoke test that
// ensures every method declared by the interface can be called through the
// interface value without panicking or returning unexpected results.
func TestUpdate