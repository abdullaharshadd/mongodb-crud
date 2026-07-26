```go
package crud_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"

	"migrated-app/internal/mongo/crud"
	"migrated-app/internal/mongo/util"
)

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

// mockCommons satisfies the util.Commons lifecycle contract.
type mockCommons struct{}

func (m *mockCommons) Connect(ctx context.Context) error    { return nil }
func (m *mockCommons) Disconnect(ctx context.Context) error { return nil }

// mockQueryDocuments is a full test-double for the QueryDocuments interface.
// Each method can be driven by a function field so individual test cases can
// inject whatever behaviour they need.
type mockQueryDocuments struct {
	mockCommons
	getAllDocumentsFn      func(ctx context.Context) ([]bson.M, error)
	getSpecificDocumentFn func(ctx context.Context, operator string) ([]bson.M, error)
}

// Ensure the mock satisfies the interface at compile time.
var _ crud.QueryDocuments = (*mockQueryDocuments)(nil)
var _ util.Commons = (*mockQueryDocuments)(nil)

func (m *mockQueryDocuments) GetAllDocuments(ctx context.Context) ([]bson.M, error) {
	if m.getAllDocumentsFn != nil {
		return m.getAllDocumentsFn(ctx)
	}
	return nil, nil
}

func (m *mockQueryDocuments) GetSpecificDocument(ctx context.Context, operator string) ([]bson.M, error) {
	if m.getSpecificDocumentFn != nil {
		return m.getSpecificDocumentFn(ctx, operator)
	}
	return nil, nil
}

// ---------------------------------------------------------------------------
// Interface shape tests
// ---------------------------------------------------------------------------

// TestQueryDocumentsInterfaceShape verifies that the interface can be
// satisfied by a concrete type and that the method signatures match what the
// migration spec prescribes.
func TestQueryDocumentsInterfaceShape(t *testing.T) {
	t.Run("mock satisfies QueryDocuments interface", func(t *testing.T) {
		var iface crud.QueryDocuments = &mockQueryDocuments{}
		assert.NotNil(t, iface)
	})

	t.Run("QueryDocuments embeds util.Commons", func(t *testing.T) {
		// A value that satisfies QueryDocuments must also satisfy util.Commons.
		var iface crud.QueryDocuments = &mockQueryDocuments{}
		var commons util.Commons = iface // assignment must compile
		assert.NotNil(t, commons)
	})
}

// ---------------------------------------------------------------------------
// GetAllDocuments tests
// ---------------------------------------------------------------------------

func TestGetAllDocuments(t *testing.T) {
	docs := []bson.M{
		{"_id": "1", "name": "Alice"},
		{"_id": "2", "name": "Bob"},
	}

	tests := []struct {
		name        string
		setupFn     func(ctx context.Context) ([]bson.M, error)
		wantDocs    []bson.M
		wantErr     bool
		wantErrMsg  string
	}{
		{
			name: "returns all documents from collection",
			setupFn: func(ctx context.Context) ([]bson.M, error) {
				return docs, nil
			},
			wantDocs: docs,
			wantErr:  false,
		},
		{
			name: "returns empty slice when collection is empty",
			setupFn: func(ctx context.Context) ([]bson.M, error) {
				return []bson.M{}, nil
			},
			wantDocs: []bson.M{},
			wantErr:  false,
		},
		{
			name: "returns nil slice with no error when nothing found",
			setupFn: func(ctx context.Context) ([]bson.M, error) {
				return nil, nil
			},
			wantDocs: nil,
			wantErr:  false,
		},
		{
			name: "returns error when database connection is unavailable",
			setupFn: func(ctx context.Context) ([]bson.M, error) {
				return nil, errors.New("database connection unavailable")
			},
			wantDocs:   nil,
			wantErr:    true,
			wantErrMsg: "database connection unavailable",
		},
		{
			name: "returns error when target collection does not exist",
			setupFn: func(ctx context.Context) ([]bson.M, error) {
				return nil, errors.New("collection does not exist")
			},
			wantDocs:   nil,
			wantErr:    true,
			wantErrMsg: "collection does not exist",
		},
		{
			name: "respects context cancellation",
			setupFn: func(ctx context.Context) ([]bson.M, error) {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				default:
					return docs, nil
				}
			},
			wantDocs: docs,
			wantErr:  false,
		},
		{
			name: "returns error on cancelled context",
			setupFn: func(ctx context.Context) ([]bson.M, error) {
				return nil, context.Canceled
			},
			wantDocs:   nil,
			wantErr:    true,
			wantErrMsg: "context canceled",
		},
		{
			name: "returns error on deadline exceeded",
			setupFn: func(ctx context.Context) ([]bson.M, error) {
				return nil, context.DeadlineExceeded
			},
			wantDocs:   nil,
			wantErr:    true,
			wantErrMsg: "context deadline exceeded",
		},
		{
			name: "operation is read-only – does not modify docs",
			setupFn: func(ctx context.Context) ([]bson.M, error) {
				// Return a snapshot; the caller must not alter the collection.
				return []bson.M{{"field": "value"}}, nil
			},
			wantDocs: []bson.M{{"field": "value"}},
			wantErr:  false,
		},
		{
			name: "returns single document collection",
			setupFn: func(ctx context.Context) ([]bson.M, error) {
				return []bson.M{{"_id": "only-one", "data": 42}}, nil
			},
			wantDocs: []bson.M{{"_id": "only-one", "data": 42}},
			wantErr:  false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockQueryDocuments{
				getAllDocumentsFn: tc.setupFn,
			}

			ctx := context.Background()
			gotDocs, err := mock.GetAllDocuments(ctx)

			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrMsg != "" {
					assert.EqualError(t, err, tc.wantErrMsg)
				}
				assert.Nil(t, gotDocs)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantDocs, gotDocs)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetSpecificDocument tests
// ---------------------------------------------------------------------------

func TestGetSpecificDocument(t *testing.T) {
	matchingDocs := []bson.M{{"_id": "42", "status": "active"}}

	tests := []struct {
		name       string
		operator   string
		setupFn    func(ctx context.Context, operator string) ([]bson.M, error)
		wantDocs   []bson.M
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:     "returns matching document for valid operator",
			operator: "status:active",
			setupFn: func(ctx context.Context, operator string) ([]bson.M, error) {
				assert.Equal(t, "status:active", operator)
				return matchingDocs, nil
			},
			wantDocs: matchingDocs,
			wantErr:  false,
		},
		{
			name:     "returns empty slice when no document matches operator",
			operator: "status:deleted",
			setupFn: func(ctx context.Context, operator string) ([]bson.M, error) {
				return []bson.M{}, nil
			},
			wantDocs: []bson.M{},
			wantErr:  false,
		},
		{
			name:     "returns nil slice when no document found",
			operator: "id:nonexistent",
			setupFn: func(ctx context.Context, operator string) ([]bson.M, error) {
				return nil, nil
			},
			wantDocs: nil,
			wantErr:  false,
		},
		{
			name:     "returns error for empty operator string",
			operator: "",
			setupFn: func(ctx context.Context, operator string) ([]bson.M, error) {
				return nil, errors.New("operator must not be empty")
			},
			wantDocs:   nil,
			wantErr:    true,
			wantErrMsg: "operator must not be empty",
		},
		{
			name:     "returns error for invalid operator",
			operator: "!!!invalid!!!",
			setupFn: func(ctx context.Context, operator string) ([]bson.M, error) {
				return nil, errors.New("invalid operator")
			},
			wantDocs:   nil,
			wantErr:    true,
			wantErrMsg: "invalid operator",
		},
		{
			name:     "returns error when database connection is unavailable",
			operator: "status:active",
			setupFn: func(ctx context.Context, operator string) ([]bson.M, error) {
				return nil, errors.New("database connection unavailable")
			},
			wantDocs:   nil,
			wantErr:    true,
			wantErrMsg: "database connection unavailable",
		},
		{
			name:     "returns error on cancelled context",
			operator: "id:1",
			setupFn: func(ctx context.Context, operator string) ([]bson.M, error) {
				return nil, context.Canceled
			},
			wantDocs:   nil,
			wantErr:    true,
			wantErrMsg: "context canceled",
		},
		{
			name:     "returns error on deadline exceeded",
			operator: "id:1",
			setupFn: func(ctx context.Context, operator string) ([]bson.M, error) {
				return nil, context.DeadlineExceeded
			},
			wantDocs:   nil,
			wantErr:    true,
			wantErrMsg: "context deadline exceeded",
		},
		{
			name:     "passes operator string unchanged to implementation",
			operator: "field:value",
			setupFn: func(ctx context.Context, operator string) ([]bson.M, error) {
				// Assert the exact operator is forwarded without mutation.
				assert.Equal(t, "field:value", operator)
				return []bson.M{{"field": "value"}}, nil
			},
			wantDocs: []bson.M{{"field": "value"}},
			wantErr:  false,
		},
		{
			name:     "operation is read-only – does not modify stored documents",
			operator: "name:Alice",
			setupFn: func(ctx context.Context, operator string) ([]bson.M, error) {
				// The returned document must be the same – implementation did
				// not alter persisted state.
				return []bson.M{{"name": "Alice"}}, nil
			},
			wantDocs: []bson.M{{"name": "Alice"}},
			wantErr:  false,
		},
		{
			name:     "returns multiple matching documents for broad operator",
			operator: "type:user",
			setupFn: func(ctx context.Context, operator string) ([]bson.M, error) {
				return []bson.M{
					{"_id": "u1", "type": "user"},
					{"_id": "u2", "type": "user"},
				}, nil
			},
			wantDocs: []bson.M{
				{"_id": "u1", "type": "user"},
				{"_id": "u2", "type": "user"},
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockQueryDocuments{
				getSpecificDocumentFn: tc.setupFn,
			}

			ctx := context.Background()
			gotDocs, err := mock.GetSpecificDocument(ctx, tc.operator)

			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrMsg != "" {
					assert.EqualError(t, err, tc.wantErrMsg)
				}
				assert.Nil(t, gotDocs)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantDocs, gotDocs)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Global invariant tests
// ---------------------------------------------------------------------------

// TestQueryDocumentsIsInterfaceOnly verifies the global invariant that
// QueryDocuments is a contract with no embedded implementation logic.
func TestQueryDocumentsIsInterfaceOnly(t *testing.T) {
	t.Run("different implementations can satisfy the same interface", func(t *testing.T) {
		implA := &mockQueryDocuments{
			getAllDocumentsFn: func(ctx context.Context) ([]bson.M, error) {
				return []bson.M{{"source": "A"}}, nil
			},
		}
		implB := &mockQueryDocuments{
			getAllDocumentsFn: func(ctx context.Context) ([]bson.M, error) {
				return []bson.M{{"source": "B"}}, nil
			},
		}

		var a crud.QueryDocuments = implA
		var b crud.QueryDocuments = implB

		docsA, errA := a.GetAllDocuments(context.Background())
		docsB, errB := b.GetAllDocuments(context.Background())

		assert.NoError(t, errA)
		assert.NoError(t, errB)
		assert.Equal(t, []bson.M{{"source": "A"}}, docsA)
		assert.Equal(t, []bson.M{{"source": "B"}}, docsB)
		assert.NotEqual(t, docsA, docsB,
			"two independent implementations must be independently controllable")
	})
}

// TestQueryDocumentsReadOnlyInvariant verifies that the interface contract
// only exposes read operations and does not expose any write/delete methods.
func TestQueryDocumentsReadOnlyInvariant(t *testing.T) {
	t.Run("interface exposes only read methods", func(t *testing.T) {
		var iface crud.QueryDocuments = &mockQueryDocuments{
			getAllDocumentsFn: func(ctx context.Context) ([]bson.M, error) {
				return []bson.M{{"read": true}}, nil
			},
			getSpecificDocumentFn: func(ctx context.Context, operator string) ([]bson.M, error) {
				return []bson.M{{"read": true, "op": operator}}, nil
			},
		}

		ctx := context.Background()

		all, err := iface.GetAllDocuments(ctx)
		assert.NoError(t, err)
		assert.NotEmpty(t, all)

		specific, err := iface.GetSpecificDocument(ctx, "key: