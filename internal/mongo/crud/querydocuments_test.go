```go
package crud_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/mongo/crud/internal/mongo/crud"
)

// ---------------------------------------------------------------------------
// Mock implementation of util.Lifecycle
// ---------------------------------------------------------------------------

// mockLifecycle satisfies the util.Lifecycle interface (assumed to declare
// Init(context.Context) error and Close(context.Context) error based on the
// typical lifecycle contract).
type mockLifecycle struct {
	initErr  error
	closeErr error
}

func (m *mockLifecycle) Init(ctx context.Context) error  { return m.initErr }
func (m *mockLifecycle) Close(ctx context.Context) error { return m.closeErr }

// ---------------------------------------------------------------------------
// Mock implementation of crud.QueryDocuments
// ---------------------------------------------------------------------------

type mockQueryDocuments struct {
	mockLifecycle

	// getAllDocuments control knobs
	allDocs    []bson.M
	allDocsErr error

	// getSpecificDocument control knobs
	specificDoc    bson.M
	specificDocErr error

	// capture calls for invariant verification
	getAllCallCount      int
	getSpecificCallArgs []string
}

// Ensure mockQueryDocuments implements the interface at compile-time.
var _ crud.QueryDocuments = (*mockQueryDocuments)(nil)

func (m *mockQueryDocuments) GetAllDocuments(ctx context.Context) ([]bson.M, error) {
	m.getAllCallCount++
	return m.allDocs, m.allDocsErr
}

func (m *mockQueryDocuments) GetSpecificDocument(ctx context.Context, operator string) (bson.M, error) {
	m.getSpecificCallArgs = append(m.getSpecificCallArgs, operator)
	return m.specificDoc, m.specificDocErr
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func newCtx() context.Context { return context.Background() }

// ---------------------------------------------------------------------------
// Tests for GetAllDocuments
// ---------------------------------------------------------------------------

func TestGetAllDocuments(t *testing.T) {
	t.Parallel()

	doc1 := bson.M{"_id": "1", "name": "alpha"}
	doc2 := bson.M{"_id": "2", "name": "beta"}

	tests := []struct {
		name string
		// mock setup
		allDocs    []bson.M
		allDocsErr error
		// expectations
		wantDocs     []bson.M
		wantErr      bool
		wantErrMsg   string
		wantCallsInc bool // the method must have been called (read-only invariant)
	}{
		{
			name:         "collection contains multiple documents - all are retrieved",
			allDocs:      []bson.M{doc1, doc2},
			allDocsErr:   nil,
			wantDocs:     []bson.M{doc1, doc2},
			wantErr:      false,
			wantCallsInc: true,
		},
		{
			name:         "collection is empty - no documents returned",
			allDocs:      []bson.M{},
			allDocsErr:   nil,
			wantDocs:     []bson.M{},
			wantErr:      false,
			wantCallsInc: true,
		},
		{
			name:         "database connection unavailable - error is propagated",
			allDocs:      nil,
			allDocsErr:   errors.New("connection refused"),
			wantDocs:     nil,
			wantErr:      true,
			wantErrMsg:   "connection refused",
			wantCallsInc: true,
		},
		{
			name:         "single document in collection",
			allDocs:      []bson.M{doc1},
			allDocsErr:   nil,
			wantDocs:     []bson.M{doc1},
			wantErr:      false,
			wantCallsInc: true,
		},
		{
			name:         "timeout context causes error",
			allDocs:      nil,
			allDocsErr:   context.DeadlineExceeded,
			wantDocs:     nil,
			wantErr:      true,
			wantErrMsg:   "context deadline exceeded",
			wantCallsInc: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockQueryDocuments{
				allDocs:    tc.allDocs,
				allDocsErr: tc.allDocsErr,
			}

			prevCallCount := mock.getAllCallCount
			docs, err := mock.GetAllDocuments(newCtx())

			// ---- behavioral assertions ----
			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrMsg != "" {
					assert.EqualError(t, err, tc.wantErrMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantDocs, docs)
			}

			// ---- invariant: method was actually called (read-only, no writes) ----
			if tc.wantCallsInc {
				assert.Equal(t, prevCallCount+1, mock.getAllCallCount,
					"GetAllDocuments must have been invoked exactly once")
			}

			// ---- invariant: read-only – specificDoc side effects are zero ----
			assert.Empty(t, mock.getSpecificCallArgs,
				"GetAllDocuments must not invoke GetSpecificDocument")
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for GetSpecificDocument
// ---------------------------------------------------------------------------

func TestGetSpecificDocument(t *testing.T) {
	t.Parallel()

	matchedDoc := bson.M{"_id": "42", "operator": "opA", "value": 100}

	tests := []struct {
		name string
		// input
		operator string
		// mock setup
		specificDoc    bson.M
		specificDocErr error
		// expectations
		wantDoc      bson.M
		wantErr      bool
		wantErrMsg   string
		wantOperator string // captured argument must equal input
	}{
		{
			name:         "operator matches an existing document",
			operator:     "opA",
			specificDoc:  matchedDoc,
			wantDoc:      matchedDoc,
			wantErr:      false,
			wantOperator: "opA",
		},
		{
			name:           "operator matches no document - not found error",
			operator:       "nonExistentOp",
			specificDoc:    nil,
			specificDocErr: errors.New("mongo: no documents in result"),
			wantDoc:        nil,
			wantErr:        true,
			wantErrMsg:     "mongo: no documents in result",
			wantOperator:   "nonExistentOp",
		},
		{
			name:         "empty operator - behavior is implementation defined but call is made",
			operator:     "",
			specificDoc:  nil,
			specificDocErr: errors.New("operator must not be empty"),
			wantDoc:      nil,
			wantErr:      true,
			wantErrMsg:   "operator must not be empty",
			wantOperator: "",
		},
		{
			name:           "null-equivalent (zero-value) operator propagated to driver",
			operator:       "",
			specificDoc:    nil,
			specificDocErr: nil,
			wantDoc:        nil,
			wantErr:        false,
			wantOperator:   "",
		},
		{
			name:         "database connection unavailable during specific query",
			operator:     "opB",
			specificDoc:  nil,
			specificDocErr: errors.New("connection refused"),
			wantDoc:      nil,
			wantErr:      true,
			wantErrMsg:   "connection refused",
			wantOperator: "opB",
		},
		{
			name:           "context cancelled during specific query",
			operator:       "opC",
			specificDoc:    nil,
			specificDocErr: context.Canceled,
			wantDoc:        nil,
			wantErr:        true,
			wantErrMsg:     "context canceled",
			wantOperator:   "opC",
		},
		{
			name:         "operator with special characters",
			operator:     "op/$gt/100",
			specificDoc:  bson.M{"_id": "7", "value": 200},
			wantDoc:      bson.M{"_id": "7", "value": 200},
			wantErr:      false,
			wantOperator: "op/$gt/100",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockQueryDocuments{
				specificDoc:    tc.specificDoc,
				specificDocErr: tc.specificDocErr,
			}

			doc, err := mock.GetSpecificDocument(newCtx(), tc.operator)

			// ---- behavioral assertions ----
			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrMsg != "" {
					assert.EqualError(t, err, tc.wantErrMsg)
				}
				assert.Nil(t, doc,
					"on error the document must be nil")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantDoc, doc)
			}

			// ---- invariant: operator was forwarded unchanged ----
			assert.Len(t, mock.getSpecificCallArgs, 1,
				"GetSpecificDocument must be called exactly once")
			assert.Equal(t, tc.wantOperator, mock.getSpecificCallArgs[0],
				"operator argument must be forwarded verbatim")

			// ---- invariant: read-only – getAllDocuments side effects are zero ----
			assert.Equal(t, 0, mock.getAllCallCount,
				"GetSpecificDocument must not invoke GetAllDocuments")
		})
	}
}

// ---------------------------------------------------------------------------
// Tests verifying interface satisfaction and read-only invariants
// ---------------------------------------------------------------------------

func TestQueryDocumentsInterface_ReadOnlyInvariant(t *testing.T) {
	t.Parallel()

	t.Run("GetAllDocuments does not mutate collection state", func(t *testing.T) {
		t.Parallel()

		original := []bson.M{
			{"_id": "1", "v": 1},
			{"_id": "2", "v": 2},
		}
		// deep copy for comparison
		snapshot := make([]bson.M, len(original))
		copy(snapshot, original)

		mock := &mockQueryDocuments{allDocs: original}
		docs, err := mock.GetAllDocuments(newCtx())

		assert.NoError(t, err)
		assert.Equal(t, snapshot, docs,
			"returned documents must equal original collection state")
		// The mock itself is unchanged - no writes occurred
		assert.Equal(t, snapshot, mock.allDocs,
			"underlying collection must not be mutated by a read")
	})

	t.Run("GetSpecificDocument does not mutate collection state", func(t *testing.T) {
		t.Parallel()

		original := bson.M{"_id": "5", "data": "immutable"}
		snapshot := bson.M{"_id": "5", "data": "immutable"}

		mock := &mockQueryDocuments{specificDoc: original}
		doc, err := mock.GetSpecificDocument(newCtx(), "immutable")

		assert.NoError(t, err)
		assert.Equal(t, snapshot, doc)
		assert.Equal(t, snapshot, mock.specificDoc,
			"underlying document must not be mutated by a read")
	})
}

func TestQueryDocumentsInterface_LifecycleEmbedded(t *testing.T) {
	t.Parallel()

	t.Run("Init succeeds", func(t *testing.T) {
		t.Parallel()
		mock := &mockQueryDocuments{}
		assert.NoError(t, mock.Init(newCtx()))
	})

	t.Run("Init propagates error", func(t *testing.T) {
		t.Parallel()
		mock := &mockQueryDocuments{
			mockLifecycle: mockLifecycle{initErr: errors.New("init failed")},
		}
		assert.EqualError(t, mock.Init(newCtx()), "init failed")
	})

	t.Run("Close succeeds", func(t *testing.T) {
		t.Parallel()
		mock := &mockQueryDocuments{}
		assert.NoError(t, mock.Close(newCtx()))
	})

	t.Run("Close propagates error", func(t *testing.T) {
		t.Parallel()
		mock := &mockQueryDocuments{
			mockLifecycle: mockLifecycle{closeErr: errors.New("close failed")},
		}
		assert.EqualError(t, mock.Close(newCtx()), "close failed")
	})
}

// ---------------------------------------------------------------------------
// Table-driven integration-style test: exercise both methods via the
// QueryDocuments interface variable (not a concrete type) to ensure Go's
// static dispatch works correctly.
// ---------------------------------------------------------------------------

func TestQueryDocumentsViaInterface(t *testing.T) {
	t.Parallel()

	doc := bson.M{"_id": "99", "name": "test"}

	tests := []struct {
		name     string
		setup    func() crud.QueryDocuments
		runAll   bool
		operator string
		// expectations
		wantAllDocs  []bson.M
		wantSpecific bson.M
		wantAllErr   bool
		wantSpecErr  bool
	}{
		{
			name: "both methods succeed through interface",
			setup: func() crud.QueryDocuments {
				return &mockQueryDocuments{
					allDocs:     []bson.M{doc},
					specificDoc: doc,
				}
			},
			runAll:       true,
			operator:     "testOp",
			wantAllDocs:  []bson.M{doc},
			wantSpecific: doc,
		},
		{
			name: "getAllDocuments fails, getSpecificDocument succeeds",
			setup: func() crud.QueryDocuments {
				return &mockQueryDocuments{
					allDocsErr:  errors.New("db down"),
					specificDoc: doc,
				}
			},
			runAll:       true,
			operator:     "testOp",
			wantAllErr:   true,
			wantSpecific: doc,
		},
		{
			name: "getAllDocuments succeeds, getSpecificDocument fails",
			setup: func() crud.QueryDocuments {
				return &mockQueryDocuments{
					allDocs:        []bson.M{doc},
					specificDocErr: errors.New("not found"),
				}
			},
			runAll:      true,
			operator:    "missingOp",
			wantAllDocs: []bson.M{doc},
			wantSpecErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var qd crud.QueryDocuments = tc.setup()

			if tc.runAll {
				allDocs, err := qd.GetAllDocuments(newCtx())
				if tc.wantAllErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tc.wantAllDocs, allDocs)
				}
			}

			specificDoc, err := qd.GetSpecificDocument(newCtx(), tc.operator)
			if tc.wantSpecErr {
				assert.Error(t, err)
				assert.Nil(t, specificDoc)
			} else {
				assert.NoError(t, err)
				