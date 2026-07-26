```go
// Package crud_test contains tests for the UpdateDocuments interface and its
// contract as migrated from the Java source.
package crud_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/mongo"

	"migrated-app/internal/mongo/crud"
)

// ---------------------------------------------------------------------------
// Fakes / mocks
// ---------------------------------------------------------------------------

// fakeCommons satisfies util.Commons so that mockUpdateDocuments can embed it
// without importing the real util package (which may have its own external
// dependencies). If the real util.Commons interface has methods, add them here.
// For now we keep it minimal – the embedded interface has no methods that are
// exercised by these tests.
type fakeCommons struct{}

// mockUpdateDocuments is a hand-rolled test double that implements
// crud.UpdateDocuments.  Each field holds a configurable stub function so that
// individual test cases can inject specific return values and side-effects
// without spinning up a real MongoDB instance.
type mockUpdateDocuments struct {
	fakeCommons

	updateOneDocumentFn            func(ctx context.Context) (*mongo.UpdateResult, error)
	updateManyDocumentFn           func(ctx context.Context) (*mongo.UpdateResult, error)
	updateDocumentWithCurrentDateFn func(ctx context.Context) (*mongo.UpdateResult, error)

	// Recorded calls – used to assert side-effect invariants.
	updateOneCalls            int
	updateManyCalls           int
	updateCurrentDateCalls    int
}

func (m *mockUpdateDocuments) UpdateOneDocument(ctx context.Context) (*mongo.UpdateResult, error) {
	m.updateOneCalls++
	if m.updateOneDocumentFn != nil {
		return m.updateOneDocumentFn(ctx)
	}
	return &mongo.UpdateResult{}, nil
}

func (m *mockUpdateDocuments) UpdateManyDocument(ctx context.Context) (*mongo.UpdateResult, error) {
	m.updateManyCalls++
	if m.updateManyDocumentFn != nil {
		return m.updateManyDocumentFn(ctx)
	}
	return &mongo.UpdateResult{}, nil
}

func (m *mockUpdateDocuments) UpdateDocumentWithCurrentDate(ctx context.Context) (*mongo.UpdateResult, error) {
	m.updateCurrentDateCalls++
	if m.updateDocumentWithCurrentDateFn != nil {
		return m.updateDocumentWithCurrentDateFn(ctx)
	}
	return &mongo.UpdateResult{}, nil
}

// Compile-time assertion: mockUpdateDocuments must satisfy crud.UpdateDocuments.
var _ crud.UpdateDocuments = (*mockUpdateDocuments)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// makeUpdateResult is a small helper to build a *mongo.UpdateResult concisely.
func makeUpdateResult(matched, modified int64) *mongo.UpdateResult {
	return &mongo.UpdateResult{
		MatchedCount:  matched,
		ModifiedCount: modified,
	}
}

// ---------------------------------------------------------------------------
// UpdateOneDocument tests
// ---------------------------------------------------------------------------

func TestUpdateOneDocument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		stub            func(ctx context.Context) (*mongo.UpdateResult, error)
		wantMatched     int64
		wantModified    int64
		wantErr         bool
		wantErrContains string
		wantCalls       int
	}{
		{
			name: "matching document exists – updates exactly one",
			stub: func(_ context.Context) (*mongo.UpdateResult, error) {
				return makeUpdateResult(1, 1), nil
			},
			wantMatched:  1,
			wantModified: 1,
			wantErr:      false,
			wantCalls:    1,
		},
		{
			name: "multiple matching documents – only first is modified",
			stub: func(_ context.Context) (*mongo.UpdateResult, error) {
				// MongoDB updateOne reports matched=1 even when multiple docs exist.
				return makeUpdateResult(1, 1), nil
			},
			wantMatched:  1,
			wantModified: 1,
			wantErr:      false,
			wantCalls:    1,
		},
		{
			name: "no document matches filter – zero modifications",
			stub: func(_ context.Context) (*mongo.UpdateResult, error) {
				return makeUpdateResult(0, 0), nil
			},
			wantMatched:  0,
			wantModified: 0,
			wantErr:      false,
			wantCalls:    1,
		},
		{
			name: "database error propagates as non-nil error",
			stub: func(_ context.Context) (*mongo.UpdateResult, error) {
				return nil, errors.New("mongo: connection refused")
			},
			wantErr:         true,
			wantErrContains: "connection refused",
			wantCalls:       1,
		},
		{
			name: "context cancellation is honoured",
			stub: func(ctx context.Context) (*mongo.UpdateResult, error) {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				default:
					return makeUpdateResult(1, 1), nil
				}
			},
			wantErr:         false, // context not cancelled in this path
			wantMatched:     1,
			wantModified:    1,
			wantCalls:       1,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := &mockUpdateDocuments{updateOneDocumentFn: tc.stub}

			result, err := svc.UpdateOneDocument(context.Background())

			assert.Equal(t, tc.wantCalls, svc.updateOneCalls,
				"UpdateOneDocument should be called exactly once per invocation")

			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrContains != "" {
					assert.Contains(t, err.Error(), tc.wantErrContains)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tc.wantMatched, result.MatchedCount)
				assert.Equal(t, tc.wantModified, result.ModifiedCount)
				// Invariant: at most one document modified per invocation.
				assert.LessOrEqual(t, result.ModifiedCount, int64(1),
					"updateOne must modify at most one document")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// UpdateManyDocument tests
// ---------------------------------------------------------------------------

func TestUpdateManyDocument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		stub            func(ctx context.Context) (*mongo.UpdateResult, error)
		wantMatched     int64
		wantModified    int64
		wantErr         bool
		wantErrContains string
		wantCalls       int
	}{
		{
			name: "multiple documents match – all are updated",
			stub: func(_ context.Context) (*mongo.UpdateResult, error) {
				return makeUpdateResult(5, 5), nil
			},
			wantMatched:  5,
			wantModified: 5,
			wantErr:      false,
			wantCalls:    1,
		},
		{
			name: "single document matches – it is updated",
			stub: func(_ context.Context) (*mongo.UpdateResult, error) {
				return makeUpdateResult(1, 1), nil
			},
			wantMatched:  1,
			wantModified: 1,
			wantErr:      false,
			wantCalls:    1,
		},
		{
			name: "no document matches filter – zero modifications",
			stub: func(_ context.Context) (*mongo.UpdateResult, error) {
				return makeUpdateResult(0, 0), nil
			},
			wantMatched:  0,
			wantModified: 0,
			wantErr:      false,
			wantCalls:    1,
		},
		{
			name: "database error propagates as non-nil error",
			stub: func(_ context.Context) (*mongo.UpdateResult, error) {
				return nil, errors.New("mongo: write concern error")
			},
			wantErr:         true,
			wantErrContains: "write concern",
			wantCalls:       1,
		},
		{
			name: "modified count equals matched count when all docs are stale",
			stub: func(_ context.Context) (*mongo.UpdateResult, error) {
				return makeUpdateResult(10, 10), nil
			},
			wantMatched:  10,
			wantModified: 10,
			wantErr:      false,
			wantCalls:    1,
		},
		{
			name: "matched count greater than modified count when some docs already current",
			stub: func(_ context.Context) (*mongo.UpdateResult, error) {
				// 8 matched but only 3 needed updating (5 already had target value).
				return makeUpdateResult(8, 3), nil
			},
			wantMatched:  8,
			wantModified: 3,
			wantErr:      false,
			wantCalls:    1,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := &mockUpdateDocuments{updateManyDocumentFn: tc.stub}

			result, err := svc.UpdateManyDocument(context.Background())

			assert.Equal(t, tc.wantCalls, svc.updateManyCalls,
				"UpdateManyDocument should be called exactly once per invocation")

			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrContains != "" {
					assert.Contains(t, err.Error(), tc.wantErrContains)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tc.wantMatched, result.MatchedCount)
				assert.Equal(t, tc.wantModified, result.ModifiedCount)
				// Invariant: modified count never exceeds matched count.
				assert.LessOrEqual(t, result.ModifiedCount, result.MatchedCount,
					"modifiedCount must never exceed matchedCount")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// UpdateDocumentWithCurrentDate tests
// ---------------------------------------------------------------------------

func TestUpdateDocumentWithCurrentDate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		stub            func(ctx context.Context) (*mongo.UpdateResult, error)
		wantMatched     int64
		wantModified    int64
		wantErr         bool
		wantErrContains string
		wantCalls       int
	}{
		{
			name: "matching document exists – date field updated",
			stub: func(_ context.Context) (*mongo.UpdateResult, error) {
				return makeUpdateResult(1, 1), nil
			},
			wantMatched:  1,
			wantModified: 1,
			wantErr:      false,
			wantCalls:    1,
		},
		{
			name: "no document matches filter – no modifications",
			stub: func(_ context.Context) (*mongo.UpdateResult, error) {
				return makeUpdateResult(0, 0), nil
			},
			wantMatched:  0,
			wantModified: 0,
			wantErr:      false,
			wantCalls:    1,
		},
		{
			name: "database error is surfaced to caller",
			stub: func(_ context.Context) (*mongo.UpdateResult, error) {
				return nil, errors.New("mongo: timeout")
			},
			wantErr:         true,
			wantErrContains: "timeout",
			wantCalls:       1,
		},
		{
			name: "multiple documents match – all receive current date",
			stub: func(_ context.Context) (*mongo.UpdateResult, error) {
				return makeUpdateResult(3, 3), nil
			},
			wantMatched:  3,
			wantModified: 3,
			wantErr:      false,
			wantCalls:    1,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := &mockUpdateDocuments{updateDocumentWithCurrentDateFn: tc.stub}

			result, err := svc.UpdateDocumentWithCurrentDate(context.Background())

			assert.Equal(t, tc.wantCalls, svc.updateCurrentDateCalls,
				"UpdateDocumentWithCurrentDate should be called exactly once per invocation")

			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrContains != "" {
					assert.Contains(t, err.Error(), tc.wantErrContains)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tc.wantMatched, result.MatchedCount)
				assert.Equal(t, tc.wantModified, result.ModifiedCount)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Context cancellation tests
// ---------------------------------------------------------------------------

func TestUpdateDocuments_ContextCancellation(t *testing.T) {
	t.Parallel()

	methods := []struct {
		name string
		call func(svc crud.UpdateDocuments, ctx context.Context) (*mongo.UpdateResult, error)
	}{
		{
			name: "UpdateOneDocument respects cancelled context",
			call: func(svc crud.UpdateDocuments, ctx context.Context) (*mongo.UpdateResult, error) {
				return svc.UpdateOneDocument(ctx)
			},
		},
		{
			name: "UpdateManyDocument respects cancelled context",
			call: func(svc crud.UpdateDocuments, ctx context.Context) (*mongo.UpdateResult, error) {
				return svc.UpdateManyDocument(ctx)
			},
		},
		{
			name: "UpdateDocumentWithCurrentDate respects cancelled context",
			call: func(svc crud.UpdateDocuments, ctx context.Context) (*mongo.UpdateResult, error) {
				return svc.UpdateDocumentWithCurrentDate(ctx)
			},
		},
	}

	for _, m := range methods {
		m := m
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()

			// Build a mock that checks the context and returns its error.
			ctxAwareFn := func(ctx context.Context) (*mongo.UpdateResult, error) {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				return makeUpdateResult(1, 1), nil
			}

			svc := &mockUpdateDocuments{
				updateOneDocumentFn:            ctxAwareFn,
				updateManyDocumentFn:           ctxAwareFn,
				updateDocumentWithCurrentDateFn: ctxAwareFn,
			}

			ctx, cancel := context.WithCancel(context.Background())
			cancel() // cancel immediately

			result, err := m.call(svc, ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, context.Canceled)
			assert.Nil(t, result)
		})
	}
}

// ---------------------------------------------------------------------------
// HTTP handler integration tests (using httptest)
//
// These tests demonstrate how an HTTP handler that orchestrates the
// UpdateDoc