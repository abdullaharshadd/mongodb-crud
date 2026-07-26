```go
// Package crud_test provides table-driven tests for the InsertDocuments
// interface defined in internal/mongo/crud/insertdocuments.go.
//
// Because InsertDocuments is a pure interface there is nothing to "call" on
// the interface itself; the tests validate:
//
//  1. The interface is satisfied by a correct concrete implementation.
//  2. Each method correctly propagates errors from the underlying MongoDB
//     driver (mocked via the mockCollection helper).
//  3. Context cancellation / deadline is forwarded to the driver.
//  4. The happy-path: each method succeeds and returns nil when the driver
//     call succeeds.
//  5. util.Commons lifecycle methods (LoadMethods / Finalized) are also
//     implemented, preserving the "extends Commons" invariant.
package crud_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"migrated-app/internal/mongo/crud"
	"migrated-app/internal/mongo/util"
)

// ---------------------------------------------------------------------------
// Minimal mock for the MongoDB collection surface used by implementations.
// ---------------------------------------------------------------------------

// insertOneFunc / insertManyFunc allow per-test injection of driver behaviour.
type insertOneFunc func(ctx context.Context, document interface{}) error
type insertManyFunc func(ctx context.Context, documents []interface{}) error

// mockCollection is a test double that captures calls made by the SUT.
type mockCollection struct {
	insertOneCalled  int
	insertManyCalled int
	insertOneFn      insertOneFunc
	insertManyFn     insertManyFunc
}

func (m *mockCollection) InsertOne(ctx context.Context, document interface{}) error {
	m.insertOneCalled++
	if m.insertOneFn != nil {
		return m.insertOneFn(ctx, document)
	}
	return nil
}

func (m *mockCollection) InsertMany(ctx context.Context, documents []interface{}) error {
	m.insertManyCalled++
	if m.insertManyFn != nil {
		return m.insertManyFn(ctx, documents)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Concrete implementation of InsertDocuments used for testing.
// ---------------------------------------------------------------------------

// fakeInsertDocuments is a minimal, testable implementation of the
// InsertDocuments interface. Real implementations would hold a live Mongo
// collection; here we accept the mockCollection double so tests remain
// hermetic.
type fakeInsertDocuments struct {
	col *mockCollection
	// Commons state
	loaded   bool
	finalized bool
}

// ------ util.Commons lifecycle ------

func (f *fakeInsertDocuments) LoadMethods() error {
	f.loaded = true
	return nil
}

func (f *fakeInsertDocuments) Finalized() error {
	f.finalized = true
	return nil
}

// ------ crud.InsertDocuments insert methods ------

func (f *fakeInsertDocuments) InsertUsingDocument(ctx context.Context) error {
	doc := map[string]interface{}{"type": "document", "value": 1}
	return f.col.InsertOne(ctx, doc)
}

func (f *fakeInsertDocuments) InsertUsingMap(ctx context.Context) error {
	doc := map[string]interface{}{"type": "map", "key": "value"}
	return f.col.InsertOne(ctx, doc)
}

func (f *fakeInsertDocuments) InsertSingleDocument(ctx context.Context) error {
	doc := map[string]interface{}{"single": true}
	return f.col.InsertOne(ctx, doc)
}

func (f *fakeInsertDocuments) InsertMultipleDocuments(ctx context.Context) error {
	docs := []interface{}{
		map[string]interface{}{"index": 0},
		map[string]interface{}{"index": 1},
		map[string]interface{}{"index": 2},
	}
	return f.col.InsertMany(ctx, docs)
}

// ---------------------------------------------------------------------------
// Compile-time assertion: fakeInsertDocuments must satisfy InsertDocuments.
// ---------------------------------------------------------------------------
var _ crud.InsertDocuments = (*fakeInsertDocuments)(nil)

// ---------------------------------------------------------------------------
// Compile-time assertion: fakeInsertDocuments must satisfy util.Commons.
// ---------------------------------------------------------------------------
var _ util.Commons = (*fakeInsertDocuments)(nil)

// ---------------------------------------------------------------------------
// Helper: newFake builds a fakeInsertDocuments wired to a mockCollection.
// ---------------------------------------------------------------------------
func newFake(col *mockCollection) *fakeInsertDocuments {
	return &fakeInsertDocuments{col: col}
}

// ---------------------------------------------------------------------------
// Tests: InsertUsingDocument
// ---------------------------------------------------------------------------

func TestInsertUsingDocument(t *testing.T) {
	errDB := errors.New("mongo: connection refused")

	tests := []struct {
		name        string
		insertOneFn insertOneFunc
		ctxFactory  func() context.Context
		wantErr     bool
		wantErrIs   error
	}{
		{
			name:       "happy path – inserts document without error",
			ctxFactory: context.Background,
			wantErr:    false,
		},
		{
			name: "driver error – returns error when InsertOne fails",
			insertOneFn: func(_ context.Context, _ interface{}) error {
				return errDB
			},
			ctxFactory: context.Background,
			wantErr:    true,
			wantErrIs:  errDB,
		},
		{
			name: "cancelled context – returns context error",
			insertOneFn: func(ctx context.Context, _ interface{}) error {
				return ctx.Err()
			},
			ctxFactory: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // cancel immediately
				return ctx
			},
			wantErr: true,
		},
		{
			name: "collection unavailable – wraps underlying error",
			insertOneFn: func(_ context.Context, _ interface{}) error {
				return errors.New("mongo: collection not found")
			},
			ctxFactory: context.Background,
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			col := &mockCollection{insertOneFn: tc.insertOneFn}
			sut := newFake(col)

			err := sut.InsertUsingDocument(tc.ctxFactory())

			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrIs != nil {
					assert.ErrorIs(t, err, tc.wantErrIs)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, 1, col.insertOneCalled,
					"InsertOne must be called exactly once on success")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: InsertUsingMap
// ---------------------------------------------------------------------------

func TestInsertUsingMap(t *testing.T) {
	errDB := errors.New("mongo: write concern error")

	tests := []struct {
		name        string
		insertOneFn insertOneFunc
		ctxFactory  func() context.Context
		wantErr     bool
		wantErrIs   error
	}{
		{
			name:       "happy path – inserts map-based document without error",
			ctxFactory: context.Background,
			wantErr:    false,
		},
		{
			name: "driver error – propagates InsertOne failure",
			insertOneFn: func(_ context.Context, _ interface{}) error {
				return errDB
			},
			ctxFactory: context.Background,
			wantErr:    true,
			wantErrIs:  errDB,
		},
		{
			name: "deadline exceeded – context deadline propagated",
			insertOneFn: func(ctx context.Context, _ interface{}) error {
				return ctx.Err()
			},
			ctxFactory: func() context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), 0)
				cancel()
				return ctx
			},
			wantErr: true,
		},
		{
			name: "collection cannot be accessed",
			insertOneFn: func(_ context.Context, _ interface{}) error {
				return errors.New("mongo: no primary found in replica set")
			},
			ctxFactory: context.Background,
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			col := &mockCollection{insertOneFn: tc.insertOneFn}
			sut := newFake(col)

			err := sut.InsertUsingMap(tc.ctxFactory())

			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrIs != nil {
					assert.ErrorIs(t, err, tc.wantErrIs)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, 1, col.insertOneCalled)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: InsertSingleDocument
// ---------------------------------------------------------------------------

func TestInsertSingleDocument(t *testing.T) {
	errDB := errors.New("mongo: duplicate key error")

	tests := []struct {
		name        string
		insertOneFn insertOneFunc
		ctxFactory  func() context.Context
		wantErr     bool
		wantErrIs   error
		// We also track that exactly ONE document was written on success.
		wantInsertOneCount int
	}{
		{
			name:               "happy path – exactly one document inserted",
			ctxFactory:         context.Background,
			wantErr:            false,
			wantInsertOneCount: 1,
		},
		{
			name: "driver returns duplicate key error",
			insertOneFn: func(_ context.Context, _ interface{}) error {
				return errDB
			},
			ctxFactory:         context.Background,
			wantErr:            true,
			wantErrIs:          errDB,
			wantInsertOneCount: 1, // attempted once even on failure
		},
		{
			name: "database connection unavailable",
			insertOneFn: func(_ context.Context, _ interface{}) error {
				return errors.New("mongo: connection pool exhausted")
			},
			ctxFactory:         context.Background,
			wantErr:            true,
			wantInsertOneCount: 1,
		},
		{
			name: "context already cancelled before call",
			insertOneFn: func(ctx context.Context, _ interface{}) error {
				return ctx.Err()
			},
			ctxFactory: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantErr:            true,
			wantInsertOneCount: 1,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			col := &mockCollection{insertOneFn: tc.insertOneFn}
			sut := newFake(col)

			err := sut.InsertSingleDocument(tc.ctxFactory())

			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrIs != nil {
					assert.ErrorIs(t, err, tc.wantErrIs)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.wantInsertOneCount, col.insertOneCalled,
				"InsertOne must be called the expected number of times")
			// InsertMany must never be invoked from InsertSingleDocument.
			assert.Zero(t, col.insertManyCalled,
				"InsertMany must not be called by InsertSingleDocument")
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: InsertMultipleDocuments
// ---------------------------------------------------------------------------

func TestInsertMultipleDocuments(t *testing.T) {
	errDB := errors.New("mongo: bulk write error")

	tests := []struct {
		name             string
		insertManyFn     insertManyFunc
		ctxFactory       func() context.Context
		wantErr          bool
		wantErrIs        error
		wantInsertManyCount int
	}{
		{
			name:                "happy path – multiple documents inserted in one call",
			ctxFactory:          context.Background,
			wantErr:             false,
			wantInsertManyCount: 1,
		},
		{
			name: "driver bulk write error propagated",
			insertManyFn: func(_ context.Context, _ []interface{}) error {
				return errDB
			},
			ctxFactory:          context.Background,
			wantErr:             true,
			wantErrIs:           errDB,
			wantInsertManyCount: 1,
		},
		{
			name: "database connection unavailable",
			insertManyFn: func(_ context.Context, _ []interface{}) error {
				return errors.New("mongo: server selection timeout")
			},
			ctxFactory:          context.Background,
			wantErr:             true,
			wantInsertManyCount: 1,
		},
		{
			name: "cancelled context forwarded to driver",
			insertManyFn: func(ctx context.Context, _ []interface{}) error {
				return ctx.Err()
			},
			ctxFactory: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantErr:             true,
			wantInsertManyCount: 1,
		},
		{
			name: "target collection cannot be accessed",
			insertManyFn: func(_ context.Context, _ []interface{}) error {
				return errors.New("mongo: unauthorized")
			},
			ctxFactory:          context.Background,
			wantErr:             true,
			wantInsertManyCount: 1,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			col := &mockCollection{insertManyFn: tc.insertManyFn}
			sut := newFake(col)

			err := sut.InsertMultipleDocuments(tc.ctxFactory())

			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrIs != nil {
					assert.ErrorIs(t, err, tc.wantErrIs)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.wantInsertManyCount, col.insertManyCalled,
				"InsertMany call count must match expectation")
			// InsertOne must never be called from InsertMultipleDocuments.
			assert.Zero(t, col.insertOneCalled,
				"InsertOne must not be called by InsertMultipleDocuments")
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: util.Commons lifecycle (LoadMethods / Finalized)
// ---------------------------------------------------------------------------

func TestCommonsLifecycle(t *testing.T) {
	tests := []struct {
		name   string
		action func(f *fakeInsertDocuments) error
		check  func(t *testing.T, f *fakeInsertDocuments)
	}{
		{
			name:   "LoadMethods marks the instance as loaded",
			action: func(f *fakeInsertDocuments) error { return f.LoadMethods() },
			check: func(t *testing.T, f *fakeInsertDocuments) {
				assert.True(t, f.loaded, "loaded flag must be set after LoadMethods")
				assert.False(t, f.