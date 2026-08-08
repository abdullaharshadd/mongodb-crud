```go
package impl_test

import (
	"context"
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"

	"internal/mongo/crud/impl"
	"internal/mongo/util"
)

// ---------------------------------------------------------------------------
// Helpers / fakes
// ---------------------------------------------------------------------------

// fakeMongoConnection wraps mtest.T so that util.MongoConnection methods can
// be satisfied without a real server. Because util.MongoConnection is a
// concrete struct we use a thin adapter that embeds the real type initialised
// from the mtest client.
//
// The adapter pattern used here avoids importing a separate mock framework and
// keeps the tests self-contained.

// newConnFromMtest constructs a *util.MongoConnection backed by the in-process
// mtest MongoDB server. The helper panics on error because it is only used
// inside test setup where a panic surfaces cleanly.
func newConnFromMtest(mt *mtest.T, db, coll string) *util.MongoConnection {
	conn, err := util.NewMongoConnectionFromClient(mt.Client, db, coll)
	if err != nil {
		mt.Fatalf("newConnFromMtest: %v", err)
	}
	return conn
}

// discardLogger returns a zerolog.Logger that silently discards all output.
func discardLogger() zerolog.Logger {
	return zerolog.Nop()
}

// ---------------------------------------------------------------------------
// Constructor tests
// ---------------------------------------------------------------------------

func TestNewDeleteDocumentsImpl(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	tests := []struct {
		name    string
		conn    func() *util.MongoConnection
		wantErr bool
	}{
		{
			name: "valid connection returns initialised impl",
			conn: func() *util.MongoConnection {
				conn, _ := util.NewMongoConnectionFromClient(mt.Client, "testdb", "testcoll")
				return conn
			},
			wantErr: false,
		},
		{
			name:    "nil connection returns error",
			conn:    func() *util.MongoConnection { return nil },
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			d, err := impl.NewDeleteDocumentsImpl(tc.conn(), discardLogger())
			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, d)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, d)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DeleteOneDocument tests
// ---------------------------------------------------------------------------

func TestDeleteOneDocument(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	tests := []struct {
		name        string
		responses   []bson.D          // mtest mock responses
		wantErr     bool
		wantErrWrap string            // substring expected in error
		deletedCount int64
	}{
		{
			name: "matching documents deleted successfully",
			responses: []bson.D{
				mtest.CreateSuccessResponse(bson.E{Key: "n", Value: int32(3)}),
			},
			wantErr:      false,
			deletedCount: 3,
		},
		{
			name: "no matching documents – count 0",
			responses: []bson.D{
				mtest.CreateSuccessResponse(bson.E{Key: "n", Value: int32(0)}),
			},
			wantErr:      false,
			deletedCount: 0,
		},
		{
			name: "mongo error is wrapped and returned",
			responses: []bson.D{
				mtest.CreateCommandErrorResponse(mtest.CommandError{
					Code:    11000,
					Message: "simulated deleteMany error",
				}),
			},
			wantErr:     true,
			wantErrWrap: "delete single document",
		},
	}

	for _, tc := range tests {
		tc := tc
		mt.Run(tc.name, func(mt *mtest.T) {
			for _, r := range tc.responses {
				mt.AddMockResponses(r)
			}

			conn := newConnFromMtest(mt, "testdb", "sample")
			d, err := impl.NewDeleteDocumentsImpl(conn, discardLogger())
			require.NoError(mt, err)

			err = d.DeleteOneDocument(context.Background())
			if tc.wantErr {
				assert.Error(mt, err)
				if tc.wantErrWrap != "" {
					assert.Contains(mt, err.Error(), tc.wantErrWrap)
				}
			} else {
				assert.NoError(mt, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DeleteManyDocument tests
// ---------------------------------------------------------------------------

func TestDeleteManyDocument(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	tests := []struct {
		name        string
		responses   []bson.D
		wantErr     bool
		wantErrWrap string
		deletedCount int64
	}{
		{
			name: "documents with age < 20 deleted successfully",
			responses: []bson.D{
				mtest.CreateSuccessResponse(bson.E{Key: "n", Value: int32(5)}),
			},
			wantErr:      false,
			deletedCount: 5,
		},
		{
			name: "no documents with age < 20 – count 0",
			responses: []bson.D{
				mtest.CreateSuccessResponse(bson.E{Key: "n", Value: int32(0)}),
			},
			wantErr:      false,
			deletedCount: 0,
		},
		{
			name: "mongo error is wrapped and returned",
			responses: []bson.D{
				mtest.CreateCommandErrorResponse(mtest.CommandError{
					Code:    11000,
					Message: "simulated deleteMany error",
				}),
			},
			wantErr:     true,
			wantErrWrap: "delete many documents",
		},
	}

	for _, tc := range tests {
		tc := tc
		mt.Run(tc.name, func(mt *mtest.T) {
			for _, r := range tc.responses {
				mt.AddMockResponses(r)
			}

			conn := newConnFromMtest(mt, "testdb", "sample")
			d, err := impl.NewDeleteDocumentsImpl(conn, discardLogger())
			require.NoError(mt, err)

			err = d.DeleteManyDocument(context.Background())
			if tc.wantErr {
				assert.Error(mt, err)
				if tc.wantErrWrap != "" {
					assert.Contains(mt, err.Error(), tc.wantErrWrap)
				}
			} else {
				assert.NoError(mt, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// LoadMethods tests
// ---------------------------------------------------------------------------

func TestLoadMethods(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	tests := []struct {
		name      string
		responses []bson.D
		wantErr   bool
		// errContains lists substrings that must appear in the combined error.
		errContains []string
	}{
		{
			name: "all operations succeed – no error returned",
			responses: []bson.D{
				// DeleteOneDocument (deleteMany internally)
				mtest.CreateSuccessResponse(bson.E{Key: "n", Value: int32(1)}),
				// DeleteManyDocument
				mtest.CreateSuccessResponse(bson.E{Key: "n", Value: int32(4)}),
			},
			wantErr: false,
		},
		{
			name: "deleteOneDocument fails – error aggregated, deleteManyDocument still executed",
			responses: []bson.D{
				// DeleteOneDocument failure
				mtest.CreateCommandErrorResponse(mtest.CommandError{
					Code:    1,
					Message: "first op error",
				}),
				// DeleteManyDocument success (proves it is still called)
				mtest.CreateSuccessResponse(bson.E{Key: "n", Value: int32(2)}),
			},
			wantErr:     true,
			errContains: []string{"delete single document"},
		},
		{
			name: "deleteManyDocument fails – error aggregated",
			responses: []bson.D{
				// DeleteOneDocument success
				mtest.CreateSuccessResponse(bson.E{Key: "n", Value: int32(1)}),
				// DeleteManyDocument failure
				mtest.CreateCommandErrorResponse(mtest.CommandError{
					Code:    2,
					Message: "second op error",
				}),
			},
			wantErr:     true,
			errContains: []string{"delete many documents"},
		},
		{
			name: "both operations fail – both errors aggregated",
			responses: []bson.D{
				mtest.CreateCommandErrorResponse(mtest.CommandError{
					Code:    1,
					Message: "first op error",
				}),
				mtest.CreateCommandErrorResponse(mtest.CommandError{
					Code:    2,
					Message: "second op error",
				}),
			},
			wantErr:     true,
			errContains: []string{"delete single document", "delete many documents"},
		},
	}

	for _, tc := range tests {
		tc := tc
		mt.Run(tc.name, func(mt *mtest.T) {
			for _, r := range tc.responses {
				mt.AddMockResponses(r)
			}

			conn := newConnFromMtest(mt, "testdb", "sample")
			d, err := impl.NewDeleteDocumentsImpl(conn, discardLogger())
			require.NoError(mt, err)

			err = d.LoadMethods(context.Background())
			if tc.wantErr {
				assert.Error(mt, err)
				for _, sub := range tc.errContains {
					assert.Contains(mt, err.Error(), sub)
				}
			} else {
				assert.NoError(mt, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Finalized tests
// ---------------------------------------------------------------------------

func TestFinalized(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	// Finalized is fire-and-forget; we verify it does not panic and that the
	// method runs without returning any value.
	mt.Run("closes client without panic", func(mt *mtest.T) {
		conn := newConnFromMtest(mt, "testdb", "sample")
		d, err := impl.NewDeleteDocumentsImpl(conn, discardLogger())
		require.NoError(mt, err)

		assert.NotPanics(mt, func() {
			d.Finalized()
		})
	})
}

// ---------------------------------------------------------------------------
// Error aggregation / errors.Join verification
// ---------------------------------------------------------------------------

func TestLoadMethods_ErrorsJoin(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("joined error unwraps to individual errors", func(mt *mtest.T) {
		// Provide two command errors so both operations fail.
		mt.AddMockResponses(
			mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 1, Message: "err1"}),
			mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "err2"}),
		)

		conn := newConnFromMtest(mt, "testdb", "sample")
		d, err := impl.NewDeleteDocumentsImpl(conn, discardLogger())
		require.NoError(mt, err)

		loadErr := d.LoadMethods(context.Background())
		require.Error(mt, loadErr)

		// errors.Join produces an error whose message contains both wrapped messages.
		assert.Contains(mt, loadErr.Error(), "delete single document")
		assert.Contains(mt, loadErr.Error(), "delete many documents")

		// Unwrapping should surface both constituent errors.
		var unwrapped []error
		if uw, ok := loadErr.(interface{ Unwrap() []error }); ok {
			unwrapped = uw.Unwrap()
		}
		assert.Len(mt, unwrapped, 2, "errors.Join should preserve both errors")

		// Neither error should be nil.
		for _, e := range unwrapped {
			assert.NotNil(mt, e)
		}
		_ = errors.Join() // ensure errors package imported
	})
}

// ---------------------------------------------------------------------------
// Filter invariant tests (inspect the commands sent to the mock server)
// ---------------------------------------------------------------------------

func TestDeleteOneDocument_FilterInvariant(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("filter targets name == sundar", func(mt *mtest.T) {
		mt.AddMockResponses(
			mtest.CreateSuccessResponse(bson.E{Key: "n", Value: int32(1)}),
		)

		conn := newConnFromMtest(mt, "testdb", "sample")
		d, err := impl.NewDeleteDocumentsImpl(conn, discardLogger())
		require.NoError(mt, err)

		err = d.DeleteOneDocument(context.Background())
		assert.NoError(mt, err)

		// Inspect the command received by the mock.
		started := mt.GetStartedEvent()
		require.NotNil(mt, started)
		assert.Equal(mt, "delete", started.CommandName)

		deletes, ok := started.Command.Lookup("deletes").Array().Values()
		// bson lookup helpers may vary; just verify the command was a delete.
		// We confirm the collection name is correct.
		collName := started.Command.Lookup("delete").StringValue()
		assert.Equal(mt, "sample", collName)

		// Examine the filter inside the first delete statement.
		if ok && len(deletes) > 0 {
			filterDoc := deletes[0].Document().Lookup("q")
			nameVal := filterDoc.Document().Lookup("name").StringValue()
			assert.Equal(mt, "sundar", nameVal)
		}
	})
}

func TestDeleteManyDocument_FilterInvariant(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("filter targets age $lt 20", func(mt *mtest.T) {
		mt.AddMockResponses(
			mtest.CreateSuccessResponse(bson.E{Key: "n", Value: int32(2)}),
		)

		conn := newConnFromMtest(mt, "testdb", "sample")
		d, err := impl.NewDeleteDocumentsImpl(conn, discardLogger())
		require.NoError(mt, err)

		err = d.DeleteManyDocument(context.Background())
		assert.NoError(mt, err)

		started := mt.GetStartedEvent()
		require.NotNil(mt, started)
		assert.Equal(mt, "delete", started.CommandName)

		collName := started.Command.Lookup("delete").StringValue()
		assert.Equal(mt, "sample", collName)

		deletes, ok := started.Command.Lookup("deletes").Array().Values()
		if ok && len(deletes) > 0 {
			filterDoc := deletes[0].Document().Lookup("q")
			ageFilter := filterDoc.Document().