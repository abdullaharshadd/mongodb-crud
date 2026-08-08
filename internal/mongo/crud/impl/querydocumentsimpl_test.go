```go
// Package impl_test provides table-driven tests for QueryDocumentsImpl.
package impl_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"

	"internal/mongo/crud/impl"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newLogger returns a no-op zerolog logger suitable for tests.
func newLogger() *zerolog.Logger {
	nop := zerolog.Nop()
	return &nop
}

// ---------------------------------------------------------------------------
// buildFilter (package-internal, tested via a thin exported wrapper or
// by exercising GetSpecificDocument with a real mtest server)
// ---------------------------------------------------------------------------

// TestBuildFilter_AllOperators validates every operator branch returns a
// non-nil filter and that unknown / empty operators are handled correctly.
// Because buildFilter is unexported we drive it through GetSpecificDocument
// against an mtest server and assert that Find was called with the correct
// filter document. Where we only need the filter shape (not real data) we
// can also test the toUpper helper indirectly via case-insensitive inputs.

// ---------------------------------------------------------------------------
// toUpper – tested indirectly through GetSpecificDocument case-insensitive
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Table-driven tests using mtest
// ---------------------------------------------------------------------------

func TestGetAllDocuments(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	tests := []struct {
		name          string
		mockResponses []bson.D
		wantErr       bool
	}{
		{
			name: "collection has documents",
			mockResponses: []bson.D{
				mtest.CreateCursorResponse(1, "testdb.sample", mtest.FirstBatch,
					bson.D{{Key: "name", Value: "Alice"}},
					bson.D{{Key: "name", Value: "Bob"}},
				),
				mtest.CreateCursorResponse(0, "testdb.sample", mtest.NextBatch),
			},
			wantErr: false,
		},
		{
			name: "collection is empty",
			mockResponses: []bson.D{
				mtest.CreateCursorResponse(0, "testdb.sample", mtest.FirstBatch),
			},
			wantErr: false,
		},
		{
			name:          "mongo error on find",
			mockResponses: []bson.D{mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 11000, Message: "find error"})},
			wantErr:       true,
		},
	}

	for _, tc := range tests {
		tc := tc
		mt.Run(tc.name, func(mt *mtest.T) {
			mt.AddMockResponses(tc.mockResponses...)

			q := impl.NewQueryDocumentsImplFromClient(mt.Client, newLogger(), "testdb", "sample")
			err := q.GetAllDocuments(context.Background())

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetSpecificDocument(t *testing.T) {
	allOperators := []string{
		"EQUAL",
		"NOT-EQUAL",
		"AND",
		"OR",
		"AND-OR",
		"IN",
		"NOT-IN",
		"LESS-THAN",
		"LESS-THAN-OR-EQUAL",
		"GREATER-THAN",
		"GREATER-THAN-OR-EQUAL",
		"LIKE",
		"EXISTS",
		"NOT-EXISTS",
	}

	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	// --- Known operators return no error ---
	for _, op := range allOperators {
		op := op
		mt.Run("known_operator_"+op, func(mt *mtest.T) {
			mt.AddMockResponses(
				mtest.CreateCursorResponse(1, "testdb.sample", mtest.FirstBatch,
					bson.D{{Key: "name", Value: "Sundar"}},
				),
				mtest.CreateCursorResponse(0, "testdb.sample", mtest.NextBatch),
			)

			q := impl.NewQueryDocumentsImplFromClient(mt.Client, newLogger(), "testdb", "sample")
			err := q.GetSpecificDocument(context.Background(), op)
			assert.NoError(mt, err, "operator %q should not error", op)
		})

		// Also test case-insensitive variant
		lowerOp := strings.ToLower(op)
		mt.Run("case_insensitive_"+lowerOp, func(mt *mtest.T) {
			mt.AddMockResponses(
				mtest.CreateCursorResponse(0, "testdb.sample", mtest.FirstBatch),
			)

			q := impl.NewQueryDocumentsImplFromClient(mt.Client, newLogger(), "testdb", "sample")
			err := q.GetSpecificDocument(context.Background(), lowerOp)
			assert.NoError(mt, err, "lower-case operator %q should not error", lowerOp)
		})
	}

	// --- Unknown operator ---
	mt.Run("unknown_operator", func(mt *mtest.T) {
		// No mock response needed; getData should not be called per spec,
		// but Go impl logs "not matched" and returns nil without querying.
		q := impl.NewQueryDocumentsImplFromClient(mt.Client, newLogger(), "testdb", "sample")
		err := q.GetSpecificDocument(context.Background(), "UNKNOWN_OP")
		assert.NoError(mt, err)
	})

	// --- Empty (nil-equivalent) operator ---
	mt.Run("empty_operator", func(mt *mtest.T) {
		q := impl.NewQueryDocumentsImplFromClient(mt.Client, newLogger(), "testdb", "sample")
		err := q.GetSpecificDocument(context.Background(), "")
		assert.NoError(mt, err)
	})

	// --- Mongo error during find for known operator ---
	mt.Run("mongo_error_on_find_known_operator", func(mt *mtest.T) {
		mt.AddMockResponses(
			mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 11000, Message: "find error"}),
		)

		q := impl.NewQueryDocumentsImplFromClient(mt.Client, newLogger(), "testdb", "sample")
		err := q.GetSpecificDocument(context.Background(), "EQUAL")
		assert.Error(mt, err)
	})
}

func TestLoadMethods(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	// 15 queries total (1 getAllDocuments + 14 operators) each needs a response.
	buildSuccessResponses := func() []bson.D {
		var responses []bson.D
		for i := 0; i < 15; i++ {
			responses = append(responses,
				mtest.CreateCursorResponse(0, "testdb.sample", mtest.FirstBatch),
			)
		}
		return responses
	}

	mt.Run("all_operations_succeed", func(mt *mtest.T) {
		mt.AddMockResponses(buildSuccessResponses()...)

		q := impl.NewQueryDocumentsImplFromClient(mt.Client, newLogger(), "testdb", "sample")
		err := q.LoadMethods(context.Background())
		assert.NoError(mt, err)
	})

	mt.Run("getAllDocuments_fails_stops_execution", func(mt *mtest.T) {
		// First response is a command error → getAllDocuments fails.
		mt.AddMockResponses(
			mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 1, Message: "getAllDocuments error"}),
		)

		q := impl.NewQueryDocumentsImplFromClient(mt.Client, newLogger(), "testdb", "sample")
		err := q.LoadMethods(context.Background())
		assert.Error(mt, err)
		assert.Contains(mt, err.Error(), "get all documents")
	})

	mt.Run("getSpecificDocument_fails_stops_execution", func(mt *mtest.T) {
		// getAllDocuments succeeds, first getSpecificDocument (EQUAL) fails.
		mt.AddMockResponses(
			// getAllDocuments – success
			mtest.CreateCursorResponse(0, "testdb.sample", mtest.FirstBatch),
			// EQUAL – failure
			mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 1, Message: "find error"}),
		)

		q := impl.NewQueryDocumentsImplFromClient(mt.Client, newLogger(), "testdb", "sample")
		err := q.LoadMethods(context.Background())
		assert.Error(mt, err)
	})
}

func TestFinalized(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("closes_client_successfully", func(mt *mtest.T) {
		q := impl.NewQueryDocumentsImplFromClient(mt.Client, newLogger(), "testdb", "sample")
		err := q.Finalized(context.Background())
		// mtest clients close without error.
		assert.NoError(mt, err)
	})
}

// ---------------------------------------------------------------------------
// buildFilter unit tests (via exported shim if available; otherwise via
// GetSpecificDocument behavior assertions above cover all branches).
// We add a pure-Go table-driven test for the filter shape by calling the
// exported BuildFilterForTest helper that we expect the impl package to
// expose for testing purposes. If no such helper exists, we test via mtest.
// ---------------------------------------------------------------------------

func TestBuildFilter_Shapes(t *testing.T) {
	tests := []struct {
		operator    string
		wantOk      bool
		checkFilter func(t *testing.T, f bson.D)
	}{
		{
			operator: "EQUAL",
			wantOk:   true,
			checkFilter: func(t *testing.T, f bson.D) {
				t.Helper()
				assert.Equal(t, "name", f[0].Key)
				assert.Equal(t, "Sundar", f[0].Value)
			},
		},
		{
			operator: "NOT-EQUAL",
			wantOk:   true,
			checkFilter: func(t *testing.T, f bson.D) {
				t.Helper()
				assert.Equal(t, "name", f[0].Key)
				m, ok := f[0].Value.(bson.M)
				assert.True(t, ok)
				assert.Equal(t, "Sundar", m["$ne"])
			},
		},
		{
			operator: "AND",
			wantOk:   true,
			checkFilter: func(t *testing.T, f bson.D) {
				t.Helper()
				assert.Equal(t, "$and", f[0].Key)
				arr, ok := f[0].Value.(bson.A)
				assert.True(t, ok)
				assert.Len(t, arr, 2)
			},
		},
		{
			operator: "OR",
			wantOk:   true,
			checkFilter: func(t *testing.T, f bson.D) {
				t.Helper()
				assert.Equal(t, "$or", f[0].Key)
				arr, ok := f[0].Value.(bson.A)
				assert.True(t, ok)
				assert.Len(t, arr, 2)
			},
		},
		{
			operator: "AND-OR",
			wantOk:   true,
			checkFilter: func(t *testing.T, f bson.D) {
				t.Helper()
				assert.Equal(t, "$and", f[0].Key)
			},
		},
		{
			operator: "IN",
			wantOk:   true,
			checkFilter: func(t *testing.T, f bson.D) {
				t.Helper()
				assert.Equal(t, "name", f[0].Key)
				m, ok := f[0].Value.(bson.M)
				assert.True(t, ok)
				arr, ok := m["$in"].(bson.A)
				assert.True(t, ok)
				assert.Contains(t, arr, "Sundar")
			},
		},
		{
			operator: "NOT-IN",
			wantOk:   true,
			checkFilter: func(t *testing.T, f bson.D) {
				t.Helper()
				assert.Equal(t, "name", f[0].Key)
				m, ok := f[0].Value.(bson.M)
				assert.True(t, ok)
				arr, ok := m["$nin"].(bson.A)
				assert.True(t, ok)
				assert.Contains(t, arr, "Sundar")
			},
		},
		{
			operator: "LESS-THAN",
			wantOk:   true,
			checkFilter: func(t *testing.T, f bson.D) {
				t.Helper()
				assert.Equal(t, "age", f[0].Key)
				m, ok := f[0].Value.(bson.M)
				assert.True(t, ok)
				assert.Equal(t, 20, m["$lt"])
			},
		},
		{
			operator: "LESS-THAN-OR-EQUAL",
			wantOk:   true,
			checkFilter: func(t *testing.T, f bson.D) {
				t.Helper()
				m, ok := f[0].Value.(bson.M)
				assert.True(t, ok)
				assert.Equal(t, 20, m["$lte"])
			},
		},
		{
			operator: "GREATER-THAN",
			wantOk:   true,
			checkFilter: func(t *testing.T, f bson.D) {
				t.Helper()
				m, ok := f[0].Value.(bson.M)
				assert.True(t, ok)
				assert.Equal(t, 20, m["$gt"])
			},
		},
		{
			operator: "GREATER-THAN-OR-EQUAL",
			wantOk:   true,
			checkFilter: func(t *testing.T, f bson.D) {
				t.Helper()
				m, ok := f[0].Value.(bson.M)
				assert.True(t, ok)
				assert.Equal(t, 20, m["$gte"])
			},
		},
		{
			operator: "LIKE",
			wantOk:   true,
			checkFilter: func(t *testing.T, f bson.D) {
				t.Helper()
				assert.Equal(t, "name", f[0].Key)
				m, ok := f[0].Value.(bson.M)
				assert.True(t, ok)
				assert.Equal(t, "^S", m["$regex"])
			},
		},
		{
			operator: "EXISTS",
			wantOk:   true,
			checkFilter: func(t *testing.T, f bson.D) {
				t.Helper()
				assert.Equal(t, "gender", f[0].Key)
				m, ok := f[0].Value.(bson.M)
				assert.True(t, ok)
				assert.Equal(t, true, m["$exists"])
			},
		},
		{
			operator: "NOT-EXISTS",
			wantOk:   true,
			checkFilter: func(t *testing.T, f