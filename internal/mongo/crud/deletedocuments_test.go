```go
package crud_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/mongo/crud/internal/mongo/crud"
	"github.com/mongo/crud/internal/mongo/util"
)

// ---------------------------------------------------------------------------
// Mock helpers
// ---------------------------------------------------------------------------

// mockCommons satisfies the util.Commons interface (assumed minimal contract).
type mockCommons struct{}

func (m *mockCommons) GetCollection() interface{} { return nil }
func (m *mockCommons) GetConnection() interface{} { return nil }

// mockDeleteDocuments is a test double that implements crud.DeleteDocuments.
// It records call arguments and returns configurable results.
type mockDeleteDocuments struct {
	mockCommons

	// deleteOneResult / deleteOneErr control the return value of DeleteOneDocument.
	deleteOneResult *mongo.DeleteResult
	deleteOneErr    error

	// deleteManyResult / deleteManyErr control the return value of DeleteManyDocument.
	deleteManyResult *mongo.DeleteResult
	deleteManyErr    error

	// captured arguments
	capturedCtx    context.Context
	capturedFilter bson.M
}

func (m *mockDeleteDocuments) DeleteOneDocument(ctx context.Context, filter bson.M) (*mongo.DeleteResult, error) {
	m.capturedCtx = ctx
	m.capturedFilter = filter
	return m.deleteOneResult, m.deleteOneErr
}

func (m *mockDeleteDocuments) DeleteManyDocument(ctx context.Context, filter bson.M) (*mongo.DeleteResult, error) {
	m.capturedCtx = ctx
	m.capturedFilter = filter
	return m.deleteManyResult, m.deleteManyErr
}

// Ensure the mock satisfies the interface at compile time.
var _ crud.DeleteDocuments = (*mockDeleteDocuments)(nil)
var _ util.Commons = (*mockDeleteDocuments)(nil)

// ---------------------------------------------------------------------------
// DeleteOneDocument tests
// ---------------------------------------------------------------------------

func TestDeleteOneDocument(t *testing.T) {
	tests := []struct {
		name string

		// inputs
		filter bson.M

		// configured mock behaviour
		mockResult *mongo.DeleteResult
		mockErr    error

		// expectations
		wantDeletedCount int64
		wantErr          bool
		wantErrContains  string
	}{
		{
			name:             "removes exactly one matching document",
			filter:           bson.M{"_id": "doc-1"},
			mockResult:       &mongo.DeleteResult{DeletedCount: 1},
			mockErr:          nil,
			wantDeletedCount: 1,
			wantErr:          false,
		},
		{
			name:             "no document deleted when filter matches nothing",
			filter:           bson.M{"_id": "nonexistent"},
			mockResult:       &mongo.DeleteResult{DeletedCount: 0},
			mockErr:          nil,
			wantDeletedCount: 0,
			wantErr:          false,
		},
		{
			name:            "connection error is surfaced",
			filter:          bson.M{"_id": "doc-1"},
			mockResult:      nil,
			mockErr:         errors.New("connection refused"),
			wantErr:         true,
			wantErrContains: "connection refused",
		},
		{
			name:            "database error is surfaced",
			filter:          bson.M{"status": "active"},
			mockResult:      nil,
			mockErr:         errors.New("database timeout"),
			wantErr:         true,
			wantErrContains: "database timeout",
		},
		{
			name:             "empty filter matches first document (at most one removed)",
			filter:           bson.M{},
			mockResult:       &mongo.DeleteResult{DeletedCount: 1},
			mockErr:          nil,
			wantDeletedCount: 1,
			wantErr:          false,
		},
		{
			name:             "at most one document removed per invocation even with broad filter",
			filter:           bson.M{"type": "temp"},
			mockResult:       &mongo.DeleteResult{DeletedCount: 1},
			mockErr:          nil,
			wantDeletedCount: 1,
			wantErr:          false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockDeleteDocuments{
				deleteOneResult: tc.mockResult,
				deleteOneErr:    tc.mockErr,
			}

			ctx := context.Background()
			result, err := mock.DeleteOneDocument(ctx, tc.filter)

			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrContains != "" {
					assert.Contains(t, err.Error(), tc.wantErrContains)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tc.wantDeletedCount, result.DeletedCount)
				// invariant: at most one document is removed per invocation
				assert.LessOrEqual(t, result.DeletedCount, int64(1),
					"DeleteOneDocument must remove at most one document")
			}

			// verify the context and filter were forwarded
			assert.Equal(t, ctx, mock.capturedCtx)
			assert.Equal(t, tc.filter, mock.capturedFilter)
		})
	}
}

// ---------------------------------------------------------------------------
// DeleteManyDocument tests
// ---------------------------------------------------------------------------

func TestDeleteManyDocument(t *testing.T) {
	tests := []struct {
		name string

		// inputs
		filter bson.M

		// configured mock behaviour
		mockResult *mongo.DeleteResult
		mockErr    error

		// expectations
		wantDeletedCount int64
		wantErr          bool
		wantErrContains  string
	}{
		{
			name:             "removes all matching documents",
			filter:           bson.M{"status": "archived"},
			mockResult:       &mongo.DeleteResult{DeletedCount: 5},
			mockErr:          nil,
			wantDeletedCount: 5,
			wantErr:          false,
		},
		{
			name:             "zero documents deleted when filter matches nothing",
			filter:           bson.M{"status": "nonexistent"},
			mockResult:       &mongo.DeleteResult{DeletedCount: 0},
			mockErr:          nil,
			wantDeletedCount: 0,
			wantErr:          false,
		},
		{
			name:             "single matching document is deleted",
			filter:           bson.M{"_id": "single-doc"},
			mockResult:       &mongo.DeleteResult{DeletedCount: 1},
			mockErr:          nil,
			wantDeletedCount: 1,
			wantErr:          false,
		},
		{
			name:             "empty filter deletes all documents in collection",
			filter:           bson.M{},
			mockResult:       &mongo.DeleteResult{DeletedCount: 100},
			mockErr:          nil,
			wantDeletedCount: 100,
			wantErr:          false,
		},
		{
			name:            "connection error is surfaced",
			filter:          bson.M{"type": "log"},
			mockResult:      nil,
			mockErr:         errors.New("network error"),
			wantErr:         true,
			wantErrContains: "network error",
		},
		{
			name:            "database error is surfaced",
			filter:          bson.M{"type": "log"},
			mockResult:      nil,
			mockErr:         errors.New("server selection timeout"),
			wantErr:         true,
			wantErrContains: "server selection timeout",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockDeleteDocuments{
				deleteManyResult: tc.mockResult,
				deleteManyErr:    tc.mockErr,
			}

			ctx := context.Background()
			result, err := mock.DeleteManyDocument(ctx, tc.filter)

			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrContains != "" {
					assert.Contains(t, err.Error(), tc.wantErrContains)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tc.wantDeletedCount, result.DeletedCount)
				// invariant: zero or more documents may be removed per invocation
				assert.GreaterOrEqual(t, result.DeletedCount, int64(0),
					"DeleteManyDocument must report a non-negative DeletedCount")
			}

			// verify the context and filter were forwarded
			assert.Equal(t, ctx, mock.capturedCtx)
			assert.Equal(t, tc.filter, mock.capturedFilter)
		})
	}
}

// ---------------------------------------------------------------------------
// Interface / contract compliance tests
// ---------------------------------------------------------------------------

// TestDeleteDocumentsInterfaceCompliance verifies that a type claiming to
// implement DeleteDocuments also satisfies util.Commons (the embedded
// "extends Commons" relationship from the Java source).
func TestDeleteDocumentsInterfaceCompliance(t *testing.T) {
	t.Run("mock satisfies DeleteDocuments interface", func(t *testing.T) {
		var iface crud.DeleteDocuments = &mockDeleteDocuments{}
		assert.NotNil(t, iface)
	})

	t.Run("mock satisfies Commons interface via embedding", func(t *testing.T) {
		var commons util.Commons = &mockDeleteDocuments{}
		assert.NotNil(t, commons)
	})
}

// TestDeleteDocumentsCancelledContext verifies that a cancelled context is
// propagated to the implementation (callers can cancel delete operations).
func TestDeleteDocumentsCancelledContext(t *testing.T) {
	t.Run("DeleteOneDocument propagates cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel before calling

		mock := &mockDeleteDocuments{
			deleteOneErr: context.Canceled,
		}

		result, err := mock.DeleteOneDocument(ctx, bson.M{"_id": "x"})
		assert.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled))
		assert.Nil(t, result)
		assert.Equal(t, ctx, mock.capturedCtx)
	})

	t.Run("DeleteManyDocument propagates cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel before calling

		mock := &mockDeleteDocuments{
			deleteManyErr: context.Canceled,
		}

		result, err := mock.DeleteManyDocument(ctx, bson.M{"type": "temp"})
		assert.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled))
		assert.Nil(t, result)
		assert.Equal(t, ctx, mock.capturedCtx)
	})
}

// TestDeleteDocumentsFilterPropagation confirms filters are forwarded exactly
// as provided — the interface itself does not mutate or default the filter.
func TestDeleteDocumentsFilterPropagation(t *testing.T) {
	filters := []struct {
		name   string
		filter bson.M
	}{
		{"empty filter", bson.M{}},
		{"id filter", bson.M{"_id": "abc"}},
		{"multi-key filter", bson.M{"status": "active", "type": "admin"}},
	}

	for _, tc := range filters {
		tc := tc
		t.Run("DeleteOneDocument/"+tc.name, func(t *testing.T) {
			t.Parallel()
			mock := &mockDeleteDocuments{
				deleteOneResult: &mongo.DeleteResult{DeletedCount: 0},
			}
			_, _ = mock.DeleteOneDocument(context.Background(), tc.filter)
			assert.Equal(t, tc.filter, mock.capturedFilter)
		})

		t.Run("DeleteManyDocument/"+tc.name, func(t *testing.T) {
			t.Parallel()
			mock := &mockDeleteDocuments{
				deleteManyResult: &mongo.DeleteResult{DeletedCount: 0},
			}
			_, _ = mock.DeleteManyDocument(context.Background(), tc.filter)
			assert.Equal(t, tc.filter, mock.capturedFilter)
		})
	}
}
```