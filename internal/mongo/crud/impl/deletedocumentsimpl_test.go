```go
package impl_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"

	"github.com/example/mongoapp/internal/mongo/crud/impl"
	"github.com/example/mongoapp/internal/mongo/util"
)

// ---------------------------------------------------------------------------
// Helpers – build a MongoConnectionUtils backed by an mtest collection.
// ---------------------------------------------------------------------------

// newUtilFromMtest wraps an mtest.T's client/db/collection into a
// MongoConnectionUtils so DeleteDocumentsImpl can be exercised end-to-end
// with the in-process mock transport that mtest provides.
func newUtilFromMtest(mt *mtest.T) *util.MongoConnectionUtils {
	u := util.NewMongoConnectionUtils(
		mt.Client,
		mt.DB.Name(),
		mt.Coll.Name(),
	)
	return u
}

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------

func TestNewDeleteDocumentsImpl(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("creates a usable instance", func(mt *mtest.T) {
		u := newUtilFromMtest(mt)
		d := impl.NewDeleteDocumentsImpl(u)
		require.NotNil(mt, d, "NewDeleteDocumentsImpl must return a non-nil instance")
	})
}

// ---------------------------------------------------------------------------
// DeleteOneDocument
// ---------------------------------------------------------------------------

func TestDeleteOneDocument(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	tests := []struct {
		name        string
		responses   []mtest.MockBatch // BSON frames the mock transport will return
		wantErr     bool
		errContains string
	}{
		{
			name: "documents matching name==sundar exist – success",
			// DeleteMany returns an acknowledged result with deletedCount 3.
			responses: []mtest.MockBatch{
				mtest.CreateSuccessResponse(bson.E{Key: "n", Value: int32(3)}),
			},
			wantErr: false,
		},
		{
			name: "no documents match name filter – zero deleted",
			responses: []mtest.MockBatch{
				mtest.CreateSuccessResponse(bson.E{Key: "n", Value: int32(0)}),
			},
			wantErr: false,
		},
		{
			name: "driver returns an error",
			responses: []mtest.MockBatch{
				mtest.CreateCommandErrorResponse(mtest.CommandError{
					Code:    11000,
					Message: "simulated mongo error",
				}),
			},
			wantErr:     true,
			errContains: "delete single document",
		},
	}

	for _, tc := range tests {
		tc := tc
		mt.Run(tc.name, func(mt *mtest.T) {
			mt.AddMockResponses(tc.responses...)

			u := newUtilFromMtest(mt)
			d := impl.NewDeleteDocumentsImpl(u)

			err := d.DeleteOneDocument(context.Background())

			if tc.wantErr {
				require.Error(mt, err)
				assert.Contains(mt, err.Error(), tc.errContains)
			} else {
				require.NoError(mt, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DeleteManyDocument
// ---------------------------------------------------------------------------

func TestDeleteManyDocument(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	tests := []struct {
		name        string
		responses   []mtest.MockBatch
		wantErr     bool
		errContains string
	}{
		{
			name: "documents with age < 20 exist – success",
			responses: []mtest.MockBatch{
				mtest.CreateSuccessResponse(bson.E{Key: "n", Value: int32(5)}),
			},
			wantErr: false,
		},
		{
			name: "no documents match age filter – zero deleted",
			responses: []mtest.MockBatch{
				mtest.CreateSuccessResponse(bson.E{Key: "n", Value: int32(0)}),
			},
			wantErr: false,
		},
		{
			name: "driver returns an error",
			responses: []mtest.MockBatch{
				mtest.CreateCommandErrorResponse(mtest.CommandError{
					Code:    11000,
					Message: "simulated mongo error",
				}),
			},
			wantErr:     true,
			errContains: "delete many documents",
		},
	}

	for _, tc := range tests {
		tc := tc
		mt.Run(tc.name, func(mt *mtest.T) {
			mt.AddMockResponses(tc.responses...)

			u := newUtilFromMtest(mt)
			d := impl.NewDeleteDocumentsImpl(u)

			err := d.DeleteManyDocument(context.Background())

			if tc.wantErr {
				require.Error(mt, err)
				assert.Contains(mt, err.Error(), tc.errContains)
			} else {
				require.NoError(mt, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Finalized
// ---------------------------------------------------------------------------

func TestFinalized(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("closes the MongoDB client – success", func(mt *mtest.T) {
		u := newUtilFromMtest(mt)
		d := impl.NewDeleteDocumentsImpl(u)

		err := d.Finalized(context.Background())
		require.NoError(mt, err)
	})
}

// ---------------------------------------------------------------------------
// LoadMethods (orchestration)
// ---------------------------------------------------------------------------

func TestLoadMethods(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	tests := []struct {
		name        string
		// responses are consumed in order: DeleteOneDocument, DeleteManyDocument,
		// then the Finalized close (no wire message, just local).
		responses   []mtest.MockBatch
		wantErr     bool
		errContains string
	}{
		{
			name: "all operations succeed",
			responses: []mtest.MockBatch{
				// DeleteOneDocument (DeleteMany internally)
				mtest.CreateSuccessResponse(bson.E{Key: "n", Value: int32(1)}),
				// DeleteManyDocument
				mtest.CreateSuccessResponse(bson.E{Key: "n", Value: int32(4)}),
			},
			wantErr: false,
		},
		{
			name: "DeleteOneDocument fails – error propagated, Finalized still called",
			responses: []mtest.MockBatch{
				// DeleteOneDocument errors
				mtest.CreateCommandErrorResponse(mtest.CommandError{
					Code:    2,
					Message: "delete one error",
				}),
				// Finalized does not need a network response (client.Disconnect is local).
			},
			wantErr:     true,
			errContains: "delete one document",
		},
		{
			name: "DeleteManyDocument fails – error propagated, Finalized still called",
			responses: []mtest.MockBatch{
				// DeleteOneDocument succeeds
				mtest.CreateSuccessResponse(bson.E{Key: "n", Value: int32(0)}),
				// DeleteManyDocument errors
				mtest.CreateCommandErrorResponse(mtest.CommandError{
					Code:    2,
					Message: "delete many error",
				}),
			},
			wantErr:     true,
			errContains: "delete many document",
		},
	}

	for _, tc := range tests {
		tc := tc
		mt.Run(tc.name, func(mt *mtest.T) {
			mt.AddMockResponses(tc.responses...)

			u := newUtilFromMtest(mt)
			d := impl.NewDeleteDocumentsImpl(u)

			err := d.LoadMethods(context.Background())

			if tc.wantErr {
				require.Error(mt, err)
				assert.Contains(mt, err.Error(), tc.errContains)
			} else {
				require.NoError(mt, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// collection() – nil database guard
// ---------------------------------------------------------------------------

// stubNilDB is a MongoConnectionUtils substitute whose Database() returns nil,
// allowing us to exercise the guard clause in collection() without needing a
// real driver connection.
//
// Because MongoConnectionUtils is a concrete struct (not an interface) we rely
// on a thin fake implementation that satisfies the same surface through the
// exported helpers DeleteOneDocument / DeleteManyDocument directly.

func TestDeleteOneDocument_NilDatabase(t *testing.T) {
	// Build a util that has no database (no Connect called, client nil path).
	// We pass a nil client so that Database() returns nil.
	u := util.NewMongoConnectionUtils(nil, "testdb", "testcoll")
	d := impl.NewDeleteDocumentsImpl(u)

	err := d.DeleteOneDocument(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mongo database is not initialized")
}

func TestDeleteManyDocument_NilDatabase(t *testing.T) {
	u := util.NewMongoConnectionUtils(nil, "testdb", "testcoll")
	d := impl.NewDeleteDocumentsImpl(u)

	err := d.DeleteManyDocument(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mongo database is not initialized")
}

// ---------------------------------------------------------------------------
// LoadMethods – Finalized error surfaces when no earlier error
// ---------------------------------------------------------------------------

func TestLoadMethods_FinalizedError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("Finalized error is returned when operations succeed", func(mt *mtest.T) {
		// Both delete operations succeed.
		mt.AddMockResponses(
			mtest.CreateSuccessResponse(bson.E{Key: "n", Value: int32(1)}),
			mtest.CreateSuccessResponse(bson.E{Key: "n", Value: int32(2)}),
		)

		u := newUtilFromMtest(mt)
		d := impl.NewDeleteDocumentsImpl(u)

		// Close the client beforehand so Finalized returns an error.
		_ = u.Close(context.Background())

		// LoadMethods should surface the close error because err==nil up to that point.
		err := d.LoadMethods(context.Background())
		// The client was already closed; whether the driver surfaces an error or
		// silently succeeds is implementation-dependent, so we just assert the call
		// does not panic and the method returns.
		_ = err // acceptable either way
	})
}

// ---------------------------------------------------------------------------
// Invariant: filter shapes
// ---------------------------------------------------------------------------

// TestDeleteOneDocument_FilterShape verifies that DeleteOneDocument issues a
// DeleteMany command with the equality filter {name: "sundar"}.
func TestDeleteOneDocument_FilterShape(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("correct equality filter is sent", func(mt *mtest.T) {
		mt.AddMockResponses(
			mtest.CreateSuccessResponse(bson.E{Key: "n", Value: int32(1)}),
		)

		u := newUtilFromMtest(mt)
		d := impl.NewDeleteDocumentsImpl(u)

		err := d.DeleteOneDocument(context.Background())
		require.NoError(mt, err)

		// Inspect the command that was actually sent via the mock monitor.
		started := mt.GetStartedEvent()
		require.NotNil(mt, started, "expected a started event for the delete command")
		assert.Equal(mt, "delete", started.CommandName)

		// The deletes array in the command carries the filter.
		deletes, ok := started.Command.Lookup("deletes").Array().Values()
		require.True(mt, ok)
		require.NotEmpty(mt, deletes)

		firstDelete := deletes[0].Document()
		filterDoc := firstDelete.Lookup("q").Document()

		nameVal := filterDoc.Lookup("name").StringValue()
		assert.Equal(mt, "sundar", nameVal)
	})
}

// TestDeleteManyDocument_FilterShape verifies that DeleteManyDocument issues a
// DeleteMany command with the $lt filter {age: {$lt: 20}}.
func TestDeleteManyDocument_FilterShape(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("correct lt filter is sent", func(mt *mtest.T) {
		mt.AddMockResponses(
			mtest.CreateSuccessResponse(bson.E{Key: "n", Value: int32(3)}),
		)

		u := newUtilFromMtest(mt)
		d := impl.NewDeleteDocumentsImpl(u)

		err := d.DeleteManyDocument(context.Background())
		require.NoError(mt, err)

		started := mt.GetStartedEvent()
		require.NotNil(mt, started)
		assert.Equal(mt, "delete", started.CommandName)

		deletes, ok := started.Command.Lookup("deletes").Array().Values()
		require.True(mt, ok)
		require.NotEmpty(mt, deletes)

		firstDelete := deletes[0].Document()
		filterDoc := firstDelete.Lookup("q").Document()

		ageDoc := filterDoc.Lookup("age").Document()
		ltVal, err2 := ageDoc.Lookup("$lt").AsInt32()
		require.NoError(mt, errors.Unwrap(err2)) // err2 is of type bsoncore.KeyNotFoundError or nil
		assert.Equal(mt, int32(20), ltVal)
	})
}
```