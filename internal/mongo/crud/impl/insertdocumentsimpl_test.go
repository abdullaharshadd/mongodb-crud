```go
// Package impl_test provides table-driven tests for InsertDocumentsImpl.
package impl_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"

	"github.com/example/project/internal/mongo/crud/impl"
	"github.com/example/project/internal/mongo/util"
)

// ---------------------------------------------------------------------------
// Helpers / fakes
// ---------------------------------------------------------------------------

// fakeMongoConnectionUtils is a test-double for *util.MongoConnectionUtils that
// lets each test inject the client, database name, collection name, and a
// Close error.
type fakeMongoConnectionUtils struct {
	client         *mongo.Client
	database       string
	sampleColl     string
	closeErr       error
	connectCalled  int
	closeCalled    int
}

func (f *fakeMongoConnectionUtils) Connect() *mongo.Client {
	f.connectCalled++
	return f.client
}

func (f *fakeMongoConnectionUtils) Database() string {
	return f.database
}

func (f *fakeMongoConnectionUtils) SampleCollection() string {
	return f.sampleColl
}

func (f *fakeMongoConnectionUtils) Close(_ context.Context) error {
	f.closeCalled++
	return f.closeErr
}

// We need InsertDocumentsImpl to accept an interface so we can inject fakes.
// Because the production code accepts *util.MongoConnectionUtils directly, we
// define a matching interface here and use a thin adapter to construct the impl.
//
// NOTE: If the production constructor were changed to accept an interface the
// fake could be passed directly. Until then, we use mtest to spin up a real
// (in-process) Mongo topology for integration-style tests, and additionally
// test the constructor / error paths with a hand-rolled fake via the exported
// field (accessible because the test is in the same module).

// ---------------------------------------------------------------------------
// mtest-based integration tests
// ---------------------------------------------------------------------------

// newImpl is a convenience factory that wires up an InsertDocumentsImpl
// against the mtest-provided mongo.Client.
func newImpl(t *testing.T, mt *mtest.T) *impl.InsertDocumentsImpl {
	t.Helper()
	utils := util.NewMongoConnectionUtils(
		mt.Client,
		mt.DB.Name(),
		mt.Coll.Name(),
	)
	return impl.NewInsertDocumentsImpl(utils)
}

// successResult returns a bson.D that the mtest framework interprets as a
// successful single-insert acknowledgement.
func successInsertOneResult() bson.D {
	return mtest.CreateSuccessResponse(
		bson.E{Key: "n", Value: 1},
	)
}

// successInsertManyResult returns a mocked insertMany success response.
func successInsertManyResult() bson.D {
	return mtest.CreateSuccessResponse(
		bson.E{Key: "n", Value: 3},
	)
}

func commandErrorResponse(code int32, msg string) bson.D {
	return mtest.CreateCommandErrorResponse(mtest.CommandError{
		Code:    code,
		Message: msg,
	})
}

// ---------------------------------------------------------------------------
// Constructor tests
// ---------------------------------------------------------------------------

func TestNewInsertDocumentsImpl(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("creates a fully initialized object", func(mt *mtest.T) {
		utils := util.NewMongoConnectionUtils(mt.Client, "testdb", "testcoll")
		require.NotNil(mt, utils)

		i := impl.NewInsertDocumentsImpl(utils)
		require.NotNil(mt, i, "NewInsertDocumentsImpl must return a non-nil pointer")
	})
}

// ---------------------------------------------------------------------------
// InsertUsingDocument
// ---------------------------------------------------------------------------

func TestInsertUsingDocument(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	tests := []struct {
		name        string
		responses   []bson.D
		wantErr     bool
		errContains string
	}{
		{
			name:      "success – one document inserted",
			responses: []bson.D{successInsertOneResult()},
			wantErr:   false,
		},
		{
			name:        "mongo error – insert fails",
			responses:   []bson.D{commandErrorResponse(11000, "duplicate key")},
			wantErr:     true,
			errContains: "inserting value using Document",
		},
	}

	for _, tc := range tests {
		tc := tc
		mt.Run(tc.name, func(mt *mtest.T) {
			for _, r := range tc.responses {
				mt.AddMockResponses(r)
			}

			i := newImpl(t, mt)
			err := i.InsertUsingDocument(context.Background())

			if tc.wantErr {
				require.Error(mt, err)
				assert.Contains(mt, err.Error(), tc.errContains)
			} else {
				assert.NoError(mt, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// InsertUsingMap
// ---------------------------------------------------------------------------

func TestInsertUsingMap(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	tests := []struct {
		name        string
		responses   []bson.D
		wantErr     bool
		errContains string
	}{
		{
			name:      "success – one document inserted with explicit _id",
			responses: []bson.D{successInsertOneResult()},
			wantErr:   false,
		},
		{
			name:        "mongo error – duplicate _id",
			responses:   []bson.D{commandErrorResponse(11000, "duplicate key error")},
			wantErr:     true,
			errContains: "inserting value using Map",
		},
	}

	for _, tc := range tests {
		tc := tc
		mt.Run(tc.name, func(mt *mtest.T) {
			for _, r := range tc.responses {
				mt.AddMockResponses(r)
			}

			i := newImpl(t, mt)
			err := i.InsertUsingMap(context.Background())

			if tc.wantErr {
				require.Error(mt, err)
				assert.Contains(mt, err.Error(), tc.errContains)
			} else {
				assert.NoError(mt, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// InsertSingleDocument
// ---------------------------------------------------------------------------

func TestInsertSingleDocument(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	tests := []struct {
		name        string
		responses   []bson.D
		wantErr     bool
		errContains string
	}{
		{
			name:      "success – canvas document inserted",
			responses: []bson.D{successInsertOneResult()},
			wantErr:   false,
		},
		{
			name:        "mongo error – write concern failure",
			responses:   []bson.D{commandErrorResponse(64, "write concern error")},
			wantErr:     true,
			errContains: "inserting single document",
		},
	}

	for _, tc := range tests {
		tc := tc
		mt.Run(tc.name, func(mt *mtest.T) {
			for _, r := range tc.responses {
				mt.AddMockResponses(r)
			}

			i := newImpl(t, mt)
			err := i.InsertSingleDocument(context.Background())

			if tc.wantErr {
				require.Error(mt, err)
				assert.Contains(mt, err.Error(), tc.errContains)
			} else {
				assert.NoError(mt, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// InsertMultipleDocuments
// ---------------------------------------------------------------------------

func TestInsertMultipleDocuments(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	tests := []struct {
		name        string
		responses   []bson.D
		wantErr     bool
		errContains string
	}{
		{
			name:      "success – three documents inserted in one batch",
			responses: []bson.D{successInsertManyResult()},
			wantErr:   false,
		},
		{
			name:        "mongo error – batch write fails",
			responses:   []bson.D{commandErrorResponse(11000, "bulk write error")},
			wantErr:     true,
			errContains: "inserting multiple documents",
		},
	}

	for _, tc := range tests {
		tc := tc
		mt.Run(tc.name, func(mt *mtest.T) {
			for _, r := range tc.responses {
				mt.AddMockResponses(r)
			}

			i := newImpl(t, mt)
			err := i.InsertMultipleDocuments(context.Background())

			if tc.wantErr {
				require.Error(mt, err)
				assert.Contains(mt, err.Error(), tc.errContains)
			} else {
				assert.NoError(mt, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Close (finalized)
// ---------------------------------------------------------------------------

func TestClose(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("close delegates to connection utility", func(mt *mtest.T) {
		i := newImpl(t, mt)
		// The real client disconnect is a no-op against a mock topology.
		err := i.Close(context.Background())
		assert.NoError(mt, err)
	})
}

// ---------------------------------------------------------------------------
// LoadMethods – orchestration
// ---------------------------------------------------------------------------

func TestLoadMethods(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	tests := []struct {
		name        string
		// responses are consumed in order by the mock transport.
		responses   []bson.D
		wantErr     bool
		errContains string
	}{
		{
			name: "success – all four inserts then close",
			responses: []bson.D{
				successInsertOneResult(),  // InsertUsingDocument
				successInsertOneResult(),  // InsertUsingMap
				successInsertOneResult(),  // InsertSingleDocument
				successInsertManyResult(), // InsertMultipleDocuments
			},
			wantErr: false,
		},
		{
			name: "error on InsertUsingDocument aborts sequence",
			responses: []bson.D{
				commandErrorResponse(2, "bad value"), // InsertUsingDocument fails
			},
			wantErr:     true,
			errContains: "insert using document",
		},
		{
			name: "error on InsertUsingMap aborts sequence",
			responses: []bson.D{
				successInsertOneResult(),             // InsertUsingDocument succeeds
				commandErrorResponse(11000, "dup"),   // InsertUsingMap fails
			},
			wantErr:     true,
			errContains: "insert using map",
		},
		{
			name: "error on InsertSingleDocument aborts sequence",
			responses: []bson.D{
				successInsertOneResult(),             // InsertUsingDocument
				successInsertOneResult(),             // InsertUsingMap
				commandErrorResponse(2, "bad value"), // InsertSingleDocument fails
			},
			wantErr:     true,
			errContains: "insert single document",
		},
		{
			name: "error on InsertMultipleDocuments aborts sequence",
			responses: []bson.D{
				successInsertOneResult(),              // InsertUsingDocument
				successInsertOneResult(),              // InsertUsingMap
				successInsertOneResult(),              // InsertSingleDocument
				commandErrorResponse(2, "batch err"),  // InsertMultipleDocuments fails
			},
			wantErr:     true,
			errContains: "insert multiple documents",
		},
	}

	for _, tc := range tests {
		tc := tc
		mt.Run(tc.name, func(mt *mtest.T) {
			for _, r := range tc.responses {
				mt.AddMockResponses(r)
			}

			i := newImpl(t, mt)
			err := i.LoadMethods(context.Background())

			if tc.wantErr {
				require.Error(mt, err)
				assert.Contains(mt, err.Error(), tc.errContains)
			} else {
				assert.NoError(mt, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Nil-client / connection-failure path
// ---------------------------------------------------------------------------

// nilClientUtils is a minimal stand-in whose Connect() always returns nil,
// simulating a connection that was never established.
type nilClientUtils struct{}

func (n *nilClientUtils) Connect() *mongo.Client    { return nil }
func (n *nilClientUtils) Database() string          { return "testdb" }
func (n *nilClientUtils) SampleCollection() string  { return "testcoll" }
func (n *nilClientUtils) Close(_ context.Context) error { return nil }

// Because the production code accepts *util.MongoConnectionUtils (a concrete
// type), we cannot inject nilClientUtils directly without changing the
// production signature.  Instead we verify the nil-client guard by ensuring
// that a zero-valued (nil) client inside a real MongoConnectionUtils surfaces
// the expected error.  We do this by building a MongoConnectionUtils with a
// nil client via the constructor that accepts a *mongo.Client.
func TestCollection_NilClient(t *testing.T) {
	tests := []struct {
		name   string
		method func(*impl.InsertDocumentsImpl) error
	}{
		{
			name:   "InsertUsingDocument returns error when client is nil",
			method: func(i *impl.InsertDocumentsImpl) error { return i.InsertUsingDocument(context.Background()) },
		},
		{
			name:   "InsertUsingMap returns error when client is nil",
			method: func(i *impl.InsertDocumentsImpl) error { return i.InsertUsingMap(context.Background()) },
		},
		{
			name:   "InsertSingleDocument returns error when client is nil",
			method: func(i *impl.InsertDocumentsImpl) error { return i.InsertSingleDocument(context.Background()) },
		},
		{
			name:   "InsertMultipleDocuments returns error when client is nil",
			method: func(i *impl.InsertDocumentsImpl) error { return i.InsertMultipleDocuments(context.Background()) },
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Construct MongoConnectionUtils with a nil *mongo.Client.
			utils := util.NewMongoConnectionUtils(nil, "testdb", "testcoll")
			i := impl.NewInsertDocumentsImpl(utils)

			err := tc.method(i)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "mongo client is not connected")
		})
	}
}

// ---------------------------------------------------------------------------
// Close error propagation
// ---------------------------------------------------------------------------

func TestClose_ErrorPropagation(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	// We cannot inject a custom Close error through the current production API
	// (which accepts *util.MongoConnectionUtils), so we verify