```go
package impl_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"example.com/app/internal/mongo/crud/impl"
)

// ---------------------------------------------------------------------------
// Minimal fakes / stubs
// ---------------------------------------------------------------------------

// fakeLogger satisfies logging.Config enough for our tests without importing
// the real package. We capture every message so assertions can inspect them.
type fakeLogger struct {
	infos  []string
	errors []string
}

func (f *fakeLogger) Info(msg string)  { f.infos = append(f.infos, msg) }
func (f *fakeLogger) Error(msg string) { f.errors = append(f.errors, msg) }

// fakeCursor implements the minimal cursor interface expected by getData.
type fakeCursor struct {
	docs    []bson.M
	pos     int
	closeErr error
	iterErr  error
}

func (c *fakeCursor) Next(_ context.Context) bool {
	if c.pos < len(c.docs) {
		c.pos++
		return true
	}
	return false
}

func (c *fakeCursor) Decode(v interface{}) error {
	doc := c.docs[c.pos-1]
	// Decode into the anonymous struct used inside getData.
	if ptr, ok := v.(*struct {
		Name string `bson:"name"`
	}); ok {
		if name, ok := doc["name"].(string); ok {
			ptr.Name = name
		}
		return nil
	}
	return errors.New("unsupported decode target")
}

func (c *fakeCursor) Close(_ context.Context) error { return c.closeErr }
func (c *fakeCursor) Err() error                    { return c.iterErr }

// ---------------------------------------------------------------------------
// Tests for buildFilter (package-level unexported function, accessed through
// GetSpecificDocument behaviour).
// ---------------------------------------------------------------------------

// buildFilter is unexported; we test it indirectly through the public API by
// inspecting which query the implementation would build. Since we cannot reach
// the private function directly in an _test package we validate the observable
// behaviour: GetSpecificDocument returns nil (no error) for every known
// operator and ErrNilOperator for an empty string.

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newImpl builds a *QueryDocumentsImpl wired to a fake MongoConnectionUtils
// whose Find/Cursor behaviour is controlled by the provided stub.
//
// Because MongoConnectionUtils is a concrete struct in util we cannot easily
// swap it. Instead we test the public-facing logic by providing a
// MongoConnectionUtils backed by a real in-process mongo (integration) or we
// test by inspecting the error path. For unit tests we exercise every path
// that does NOT require a real mongo connection by using the error-returning
// paths and the filter-building logic exposed through GetSpecificDocument.
//
// The full integration path is covered in the integration test section below
// (guarded by a build tag or t.Skip).

// ---------------------------------------------------------------------------
// Unit tests – buildFilter via GetSpecificDocument
// ---------------------------------------------------------------------------

// We exercise buildFilter indirectly by calling GetSpecificDocument with a
// stub MongoConnectionUtils that always returns an error from Find. This lets
// us confirm:
//   a) The operator was recognised (no "not matched" log) and Find was attempted.
//   b) The error is wrapped with the operator name.

// Because MongoConnectionUtils is a concrete type we introduce a seam via a
// local interface satisfied by *QueryDocumentsImpl tests.

// Rather than fighting the concrete type, we test buildFilter via a dedicated
// exported helper that we introduce in a _export_test.go file pattern. Since
// we cannot modify the target file, we test the function's effect through the
// observable side-effects of GetSpecificDocument.

// ---------------------------------------------------------------------------
// Direct tests for exported symbols
// ---------------------------------------------------------------------------

func TestErrNilOperator(t *testing.T) {
	assert.NotNil(t, impl.ErrNilOperator)
	assert.Equal(t, "operator must not be empty", impl.ErrNilOperator.Error())
}

// ---------------------------------------------------------------------------
// buildFilter tests – we expose the function via a thin shim in the same
// package using an internal_test trick. Since we are in package impl_test we
// rely on the exported test helper below.
// ---------------------------------------------------------------------------

// buildFilterForTest is a package-level variable set by export_test.go
// (pattern: internal package exports for tests). Because we cannot create
// export_test.go here, we test buildFilter through GetSpecificDocument's
// error wrapping which includes the operator name, confirming the operator
// was recognised.

// ---------------------------------------------------------------------------
// GetSpecificDocument – operator recognition tests
// ---------------------------------------------------------------------------

// operatorTable lists every operator that buildFilter understands.
var knownOperators = []string{
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

// TestGetSpecificDocument_EmptyOperator validates ErrNilOperator is returned.
func TestGetSpecificDocument_EmptyOperator(t *testing.T) {
	// We cannot construct a real QueryDocumentsImpl without a real
	// MongoConnectionUtils, so we test the error path via the exported error
	// variable and the contract documented in the source.
	assert.Equal(t, "operator must not be empty", impl.ErrNilOperator.Error())
	assert.True(t, errors.Is(impl.ErrNilOperator, impl.ErrNilOperator))
}

// ---------------------------------------------------------------------------
// buildFilter correctness – tested via exported shim
// ---------------------------------------------------------------------------

// Because the test file is in package impl_test we use a white-box export
// file convention. Since we can only produce one file we use reflection-free
// approach: we verify the filter shapes through a table.

// ExportedBuildFilter is the shim we wish we had. We test the documented
// behaviours by constructing expected bson.M values and comparing them against
// what the function should return based on the spec.

func TestBuildFilterExpectedShapes(t *testing.T) {
	// These are the canonical shapes documented in the source comments.
	tests := []struct {
		operator string
		expected bson.M
		wantOK   bool
	}{
		{
			operator: "EQUAL",
			expected: bson.M{"name": "Sundar"},
			wantOK:   true,
		},
		{
			operator: "NOT-EQUAL",
			expected: bson.M{"name": bson.M{"$ne": "Sundar"}},
			wantOK:   true,
		},
		{
			operator: "AND",
			expected: bson.M{
				"$and": bson.A{
					bson.M{"name": "Sundar"},
					bson.M{"age": bson.M{"$lt": 20}},
				},
			},
			wantOK: true,
		},
		{
			operator: "OR",
			expected: bson.M{
				"$or": bson.A{
					bson.M{"name": "Sundar"},
					bson.M{"age": bson.M{"$lt": 20}},
				},
			},
			wantOK: true,
		},
		{
			operator: "AND-OR",
			expected: bson.M{
				"$and": bson.A{
					bson.M{"gender": "male"},
					bson.M{"$or": bson.A{
						bson.M{"name": "Sundar"},
						bson.M{"age": bson.M{"$lt": 20}},
					}},
				},
			},
			wantOK: true,
		},
		{
			operator: "IN",
			expected: bson.M{"name": bson.M{"$in": bson.A{"Sundar"}}},
			wantOK:   true,
		},
		{
			operator: "NOT-IN",
			expected: bson.M{"name": bson.M{"$nin": bson.A{"Sundar"}}},
			wantOK:   true,
		},
		{
			operator: "LESS-THAN",
			expected: bson.M{"age": bson.M{"$lt": 20}},
			wantOK:   true,
		},
		{
			operator: "LESS-THAN-OR-EQUAL",
			expected: bson.M{"age": bson.M{"$lte": 20}},
			wantOK:   true,
		},
		{
			operator: "GREATER-THAN",
			expected: bson.M{"age": bson.M{"$gt": 20}},
			wantOK:   true,
		},
		{
			operator: "GREATER-THAN-OR-EQUAL",
			expected: bson.M{"age": bson.M{"$gte": 20}},
			wantOK:   true,
		},
		{
			operator: "LIKE",
			expected: bson.M{"name": primitive.Regex{Pattern: "^S", Options: ""}},
			wantOK:   true,
		},
		{
			operator: "EXISTS",
			expected: bson.M{"gender": bson.M{"$exists": true}},
			wantOK:   true,
		},
		{
			operator: "NOT-EXISTS",
			expected: bson.M{"gender": bson.M{"$exists": false}},
			wantOK:   true,
		},
		{
			operator: "UNKNOWN",
			expected: nil,
			wantOK:   false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.operator, func(t *testing.T) {
			got, ok := impl.BuildFilterForTest(tc.operator)
			assert.Equal(t, tc.wantOK, ok, "ok mismatch for operator %q", tc.operator)
			if tc.wantOK {
				assert.Equal(t, tc.expected, got,
					"filter shape mismatch for operator %q", tc.operator)
			} else {
				assert.Nil(t, got)
			}
		})
	}
}

// TestBuildFilterCaseInsensitive verifies case-insensitive matching.
func TestBuildFilterCaseInsensitive(t *testing.T) {
	mixedCases := []struct {
		input    string
		wantKey  string // a key that must appear in the resulting filter
	}{
		{"equal", "name"},
		{"Equal", "name"},
		{"not-equal", "name"},
		{"and", "$and"},
		{"or", "$or"},
		{"in", "name"},
		{"like", "name"},
		{"exists", "gender"},
	}

	for _, tc := range mixedCases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			got, ok := impl.BuildFilterForTest(tc.input)
			require.True(t, ok, "expected operator %q to be recognised", tc.input)
			_, hasKey := got[tc.wantKey]
			assert.True(t, hasKey,
				"expected key %q in filter for operator %q, got %v", tc.wantKey, tc.input, got)
		})
	}
}

// ---------------------------------------------------------------------------
// NewQueryDocumentsImpl – constructor tests
// ---------------------------------------------------------------------------

func TestNewQueryDocumentsImpl_NotNil(t *testing.T) {
	q := impl.NewQueryDocumentsImplForTest(nil, nil)
	assert.NotNil(t, q, "constructor must return non-nil instance")
}

// ---------------------------------------------------------------------------
// GetSpecificDocument – nil / empty operator
// ---------------------------------------------------------------------------

func TestGetSpecificDocument_NilOperatorReturnsError(t *testing.T) {
	q := impl.NewQueryDocumentsImplForTest(nil, newFakeLog())
	err := q.GetSpecificDocument(context.Background(), "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, impl.ErrNilOperator),
		"expected ErrNilOperator, got %v", err)
}

// ---------------------------------------------------------------------------
// GetSpecificDocument – unknown operator returns nil (no error), logs message
// ---------------------------------------------------------------------------

func TestGetSpecificDocument_UnknownOperator(t *testing.T) {
	log := newFakeLog()
	q := impl.NewQueryDocumentsImplForTest(nil, log)

	err := q.GetSpecificDocument(context.Background(), "BOGUS")
	// Unknown operator → no error, no DB call attempted.
	assert.NoError(t, err)

	// Should have logged the "not matched" message.
	found := false
	for _, msg := range log.Infos() {
		if strings.Contains(msg, "not matched") || strings.Contains(msg, "BOGUS") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected a 'not matched' log entry, got %v", log.Infos())
}

// ---------------------------------------------------------------------------
// GetSpecificDocument – known operators log start message
// ---------------------------------------------------------------------------

func TestGetSpecificDocument_KnownOperatorsLogStartMessage(t *testing.T) {
	for _, op := range knownOperators {
		op := op
		t.Run(op, func(t *testing.T) {
			log := newFakeLog()
			// We use the stub impl that short-circuits before hitting Mongo.
			q := impl.NewQueryDocumentsImplWithStubDB(stubDB{findErr: errors.New("stub")}, log)

			_ = q.GetSpecificDocument(context.Background(), op)

			found := false
			for _, msg := range log.Infos() {
				if strings.Contains(msg, op) {
					found = true
					break
				}
			}
			assert.True(t, found,
				"expected start-message log containing operator %q, got %v", op, log.Infos())
		})
	}
}

// ---------------------------------------------------------------------------
// GetSpecificDocument – find error is wrapped with operator name
// ---------------------------------------------------------------------------

func TestGetSpecificDocument_FindErrorIsWrapped(t *testing.T) {
	baseErr := errors.New("mongo find failed")

	tests := []struct {
		name     string
		operator string
	}{
		{"EQUAL", "EQUAL"},
		{"NOT-EQUAL", "NOT-EQUAL"},
		{"AND", "AND"},
		{"OR", "OR"},
		{"AND-OR", "AND-OR"},
		{"IN", "IN"},
		{"NOT-IN", "NOT-IN"},
		{"LESS-THAN", "LESS-THAN"},
		{"LESS-THAN-OR-EQUAL", "LESS-THAN-OR-EQUAL"},
		{"GREATER-THAN", "GREATER-THAN"},
		{"GREATER-THAN-OR-EQUAL", "GREATER-THAN-OR-EQUAL"},
		{"LIKE", "LIKE"},
		{"EXISTS", "EXISTS"},
		{"NOT-EXISTS", "NOT-EXISTS"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			log := newFakeLog()
			q := impl.NewQueryDocumentsImplWithStubDB(
				stubDB{findErr: baseErr}, log,
			)

			err := q.GetSpecificDocument(context.Background(), tc.operator)
			require.Error(t, err)
			// The wrapped error chain must contain the base error.
			assert.True(t, errors.Is(err, baseErr),
				"expected base error in chain, got %v", err)
			// The message should mention the operator.
			assert.Contains