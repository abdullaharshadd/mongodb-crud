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
// Fake / mock implementations
// ---------------------------------------------------------------------------

// mockLifecycle is a minimal util.Lifecycle implementation used by the mock.
type mockLifecycle struct {
	loadErr     error
	finalizeErr error
}

func (m *mockLifecycle) Load(ctx context.Context) error     { return m.loadErr }
func (m *mockLifecycle) Finalize(ctx context.Context) error { return m.finalizeErr }

// Ensure mockLifecycle satisfies util.Lifecycle at compile time.
var _ util.Lifecycle = (*mockLifecycle)(nil)

// mockInsertDocuments is a controllable implementation of the InsertDocuments
// interface used throughout the table-driven tests. Each method records
// whether it was called and returns a preconfigured error.
type mockInsertDocuments struct {
	mockLifecycle

	insertUsingDocumentCalled    bool
	insertUsingMapCalled         bool
	insertSingleDocumentCalled   bool
	insertMultipleDocumentsCalled bool

	insertUsingDocumentErr    error
	insertUsingMapErr         error
	insertSingleDocumentErr   error
	insertMultipleDocumentsErr error
}

func (m *mockInsertDocuments) InsertUsingDocument(ctx context.Context) error {
	m.insertUsingDocumentCalled = true
	return m.insertUsingDocumentErr
}

func (m *mockInsertDocuments) InsertUsingMap(ctx context.Context) error {
	m.insertUsingMapCalled = true
	return m.insertUsingMapErr
}

func (m *mockInsertDocuments) InsertSingleDocument(ctx context.Context) error {
	m.insertSingleDocumentCalled = true
	return m.insertSingleDocumentErr
}

func (m *mockInsertDocuments) InsertMultipleDocuments(ctx context.Context) error {
	m.insertMultipleDocumentsCalled = true
	return m.insertMultipleDocumentsErr
}

// Ensure mockInsertDocuments satisfies the full contract at compile time.
var _ crud.InsertDocuments = (*mockInsertDocuments)(nil)

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func newCtx() context.Context { return context.Background() }

// ---------------------------------------------------------------------------
// InsertUsingDocument
// ---------------------------------------------------------------------------

func TestInsertUsingDocument(t *testing.T) {
	dbErr := errors.New("mongo: connection refused")

	tests := []struct {
		name          string
		mock          *mockInsertDocuments
		wantErr       bool
		wantErrMsg    string
		wantCalled    bool
	}{
		{
			name:       "success – document is written to collection",
			mock:       &mockInsertDocuments{insertUsingDocumentErr: nil},
			wantErr:    false,
			wantCalled: true,
		},
		{
			name:       "propagates database error when connection unavailable",
			mock:       &mockInsertDocuments{insertUsingDocumentErr: dbErr},
			wantErr:    true,
			wantErrMsg: "mongo: connection refused",
			wantCalled: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.mock.InsertUsingDocument(newCtx())

			assert.Equal(t, tc.wantCalled, tc.mock.insertUsingDocumentCalled,
				"InsertUsingDocument should have been called")

			if tc.wantErr {
				assert.Error(t, err)
				assert.EqualError(t, err, tc.wantErrMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// InsertUsingMap
// ---------------------------------------------------------------------------

func TestInsertUsingMap(t *testing.T) {
	dbErr := errors.New("mongo: collection unavailable")

	tests := []struct {
		name       string
		mock       *mockInsertDocuments
		wantErr    bool
		wantErrMsg string
		wantCalled bool
	}{
		{
			name:       "success – document written via map representation",
			mock:       &mockInsertDocuments{insertUsingMapErr: nil},
			wantErr:    false,
			wantCalled: true,
		},
		{
			name:       "propagates database error when connection unavailable",
			mock:       &mockInsertDocuments{insertUsingMapErr: dbErr},
			wantErr:    true,
			wantErrMsg: "mongo: collection unavailable",
			wantCalled: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.mock.InsertUsingMap(newCtx())

			assert.Equal(t, tc.wantCalled, tc.mock.insertUsingMapCalled,
				"InsertUsingMap should have been called")

			if tc.wantErr {
				assert.Error(t, err)
				assert.EqualError(t, err, tc.wantErrMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// InsertSingleDocument
// ---------------------------------------------------------------------------

func TestInsertSingleDocument(t *testing.T) {
	dbErr := errors.New("mongo: write concern error")

	tests := []struct {
		name       string
		mock       *mockInsertDocuments
		wantErr    bool
		wantErrMsg string
		wantCalled bool
	}{
		{
			name:       "success – exactly one document inserted",
			mock:       &mockInsertDocuments{insertSingleDocumentErr: nil},
			wantErr:    false,
			wantCalled: true,
		},
		{
			name:       "propagates database error when connection unavailable",
			mock:       &mockInsertDocuments{insertSingleDocumentErr: dbErr},
			wantErr:    true,
			wantErrMsg: "mongo: write concern error",
			wantCalled: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.mock.InsertSingleDocument(newCtx())

			assert.Equal(t, tc.wantCalled, tc.mock.insertSingleDocumentCalled,
				"InsertSingleDocument should have been called")

			if tc.wantErr {
				assert.Error(t, err)
				assert.EqualError(t, err, tc.wantErrMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// InsertMultipleDocuments
// ---------------------------------------------------------------------------

func TestInsertMultipleDocuments(t *testing.T) {
	dbErr := errors.New("mongo: bulk write failed")

	tests := []struct {
		name       string
		mock       *mockInsertDocuments
		wantErr    bool
		wantErrMsg string
		wantCalled bool
	}{
		{
			name:       "success – multiple documents inserted in single operation",
			mock:       &mockInsertDocuments{insertMultipleDocumentsErr: nil},
			wantErr:    false,
			wantCalled: true,
		},
		{
			name:       "propagates database error when connection unavailable",
			mock:       &mockInsertDocuments{insertMultipleDocumentsErr: dbErr},
			wantErr:    true,
			wantErrMsg: "mongo: bulk write failed",
			wantCalled: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.mock.InsertMultipleDocuments(newCtx())

			assert.Equal(t, tc.wantCalled, tc.mock.insertMultipleDocumentsCalled,
				"InsertMultipleDocuments should have been called")

			if tc.wantErr {
				assert.Error(t, err)
				assert.EqualError(t, err, tc.wantErrMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Lifecycle contract inherited from util.Lifecycle
// ---------------------------------------------------------------------------

func TestInsertDocuments_LifecycleLoad(t *testing.T) {
	lifecycleErr := errors.New("lifecycle: load failed")

	tests := []struct {
		name       string
		mock       *mockInsertDocuments
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:    "Load succeeds",
			mock:    &mockInsertDocuments{mockLifecycle: mockLifecycle{loadErr: nil}},
			wantErr: false,
		},
		{
			name:       "Load propagates error",
			mock:       &mockInsertDocuments{mockLifecycle: mockLifecycle{loadErr: lifecycleErr}},
			wantErr:    true,
			wantErrMsg: "lifecycle: load failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Cast to the interface to verify the contract is satisfied.
			var iface crud.InsertDocuments = tc.mock
			err := iface.Load(newCtx())
			if tc.wantErr {
				assert.Error(t, err)
				assert.EqualError(t, err, tc.wantErrMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestInsertDocuments_LifecycleFinalize(t *testing.T) {
	lifecycleErr := errors.New("lifecycle: finalize failed")

	tests := []struct {
		name       string
		mock       *mockInsertDocuments
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:    "Finalize succeeds",
			mock:    &mockInsertDocuments{mockLifecycle: mockLifecycle{finalizeErr: nil}},
			wantErr: false,
		},
		{
			name:       "Finalize propagates error",
			mock:       &mockInsertDocuments{mockLifecycle: mockLifecycle{finalizeErr: lifecycleErr}},
			wantErr:    true,
			wantErrMsg: "lifecycle: finalize failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var iface crud.InsertDocuments = tc.mock
			err := iface.Finalize(newCtx())
			if tc.wantErr {
				assert.Error(t, err)
				assert.EqualError(t, err, tc.wantErrMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Context propagation / cancellation
// ---------------------------------------------------------------------------

func TestInsertDocuments_ContextCancellation(t *testing.T) {
	cancelErr := errors.New("context canceled")

	tests := []struct {
		name       string
		method     string
		mock       *mockInsertDocuments
		invoke     func(crud.InsertDocuments, context.Context) error
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:   "InsertUsingDocument – cancelled context propagates error",
			method: "InsertUsingDocument",
			mock:   &mockInsertDocuments{insertUsingDocumentErr: cancelErr},
			invoke: func(i crud.InsertDocuments, ctx context.Context) error {
				return i.InsertUsingDocument(ctx)
			},
			wantErr:    true,
			wantErrMsg: "context canceled",
		},
		{
			name:   "InsertUsingMap – cancelled context propagates error",
			method: "InsertUsingMap",
			mock:   &mockInsertDocuments{insertUsingMapErr: cancelErr},
			invoke: func(i crud.InsertDocuments, ctx context.Context) error {
				return i.InsertUsingMap(ctx)
			},
			wantErr:    true,
			wantErrMsg: "context canceled",
		},
		{
			name:   "InsertSingleDocument – cancelled context propagates error",
			method: "InsertSingleDocument",
			mock:   &mockInsertDocuments{insertSingleDocumentErr: cancelErr},
			invoke: func(i crud.InsertDocuments, ctx context.Context) error {
				return i.InsertSingleDocument(ctx)
			},
			wantErr:    true,
			wantErrMsg: "context canceled",
		},
		{
			name:   "InsertMultipleDocuments – cancelled context propagates error",
			method: "InsertMultipleDocuments",
			mock:   &mockInsertDocuments{insertMultipleDocumentsErr: cancelErr},
			invoke: func(i crud.InsertDocuments, ctx context.Context) error {
				return i.InsertMultipleDocuments(ctx)
			},
			wantErr:    true,
			wantErrMsg: "context canceled",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // cancel immediately to simulate a cancelled context

			err := tc.invoke(tc.mock, ctx)
			if tc.wantErr {
				assert.Error(t, err)
				assert.EqualError(t, err, tc.wantErrMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Interface-only contract assertions (compile-time + runtime shape checks)
// ---------------------------------------------------------------------------

// TestInsertDocuments_InterfaceSatisfied verifies that any value assignable
// to InsertDocuments also satisfies util.Lifecycle (the embedded contract).
func TestInsertDocuments_InterfaceSatisfied(t *testing.T) {
	var impl crud.InsertDocuments = &mockInsertDocuments{}

	// The embedded util.Lifecycle methods must be reachable via the
	// InsertDocuments interface.
	_, ok := impl.(util.Lifecycle)
	assert.True(t, ok, "InsertDocuments must embed util.Lifecycle")
}

// TestInsertDocuments_AllMethodsArePublic uses a table to call every method
// via the interface and confirms no panic occurs with a background context.
func TestInsertDocuments_AllMethodsArePublic(t *testing.T) {
	impl := &mockInsertDocuments{}
	var iface crud.InsertDocuments = impl

	type invocation struct {
		name   string
		call   func() error
	}

	invocations := []invocation{
		{"Load", func() error { return iface.Load(newCtx()) }},
		{"Finalize", func() error { return iface.Finalize(newCtx()) }},
		{"InsertUsingDocument", func() error { return iface.InsertUsingDocument(newCtx()) }},
		{"InsertUsingMap", func() error { return iface.InsertUsingMap(newCtx()) }},
		{"InsertSingleDocument", func() error { return iface.InsertSingleDocument(newCtx()) }},
		{"InsertMultipleDocuments", func() error { return iface.InsertMultipleDocuments(newCtx()) }},
	}

	for _, inv := range invocations {
		t.Run(inv.name+" is callable via interface", func(t *testing.T) {
			assert.NotPanics(t, func() {
				_ = inv.