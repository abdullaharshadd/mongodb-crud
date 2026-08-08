```go
package impl_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"

	"example.com/app/internal/mongo/crud/impl"
	"example.com/app/internal/mongo/util"
)

// ---------------------------------------------------------------------------
// Helpers / fakes
// ---------------------------------------------------------------------------

// nopLogger returns a zerolog logger that discards all output.
func nopLogger() *zerolog.Logger {
	l := zerolog.Nop()
	return &l
}

// fakeConn wraps a *mtest.T-supplied client behind the util.MongoConnection
// surface that UpdateDocumentsImpl needs. Because util.MongoConnection is a
// concrete struct we cannot embed a fake client directly; instead we use
// mtest which provides a real *mongo.Client pointed at an in-process Mongo
// mock.
//
// The tests therefore drive the impl via mtest, which is the idiomatic way to
// unit-test mongo-driver code without a live server.

// ---------------------------------------------------------------------------
// Constructor tests
// ---------------------------------------------------------------------------

func TestNewUpdateDocumentsImpl_NilConn(t *testing.T) {
	_, err := impl.NewUpdateDocumentsImpl(context.Background(), nil, nopLogger())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestNewUpdateDocumentsImpl_NilLogger(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	mt.Run("nil logger", func(mt *mtest.T) {
		conn := util.NewMongoConnectionFromClient(mt.Client, mt.DB.Name(), "sample")
		_, err := impl.NewUpdateDocumentsImpl(context.Background(), conn, nil)
		assert.Error(mt, err)
		assert.Contains(mt, err.Error(), "nil")
	})
}

func TestNewUpdateDocumentsImpl_ClientAcquisitionFailure(t *testing.T) {
	// Pass a MongoConnection whose Client() method will fail.
	conn := util.NewMongoConnectionBroken() // returns error on Client()
	_, err := impl.NewUpdateDocumentsImpl(context.Background(), conn, nopLogger())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "acquire mongo client")
}

func TestNewUpdateDocumentsImpl_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	mt.Run("success", func(mt *mtest.T) {
		conn := util.NewMongoConnectionFromClient(mt.Client, mt.DB.Name(), "sample")
		impl_, err := impl.NewUpdateDocumentsImpl(context.Background(), conn, nopLogger())
		require.NoError(mt, err)
		assert.NotNil(mt, impl_)
	})
}

// ---------------------------------------------------------------------------
// UpdateOneDocument tests
// ---------------------------------------------------------------------------

func TestUpdateOneDocument(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	tests := []struct {
		name        string
		responses   []bson.D
		wantErr     bool
		errContains string
	}{
		{
			name: "matching document exists – one modified",
			responses: []bson.D{
				mtest.CreateSuccessResponse(
					bson.E{Key: "n", Value: 1},
					bson.E{Key: "nModified", Value: 1},
				),
			},
			wantErr: false,
		},
		{
			name: "no document matches – zero modified",
			responses: []bson.D{
				mtest.CreateSuccessResponse(
					bson.E{Key: "n", Value: 0},
					bson.E{Key: "nModified", Value: 0},
				),
			},
			wantErr: false,
		},
		{
			name: "mongodb error during update",
			responses: []bson.D{
				mtest.CreateCommandErrorResponse(mtest.CommandError{
					Code:    11000,
					Message: "forced update error",
				}),
			},
			wantErr:     true,
			errContains: "update one document",
		},
	}

	for _, tc := range tests {
		tc := tc
		mt.Run(tc.name, func(mt *mtest.T) {
			for _, r := range tc.responses {
				mt.AddMockResponses(r)
			}

			conn := util.NewMongoConnectionFromClient(mt.Client, mt.DB.Name(), mt.Coll.Name())
			sut, err := impl.NewUpdateDocumentsImpl(context.Background(), conn, nopLogger())
			require.NoError(mt, err)

			err = sut.UpdateOneDocument(context.Background())
			if tc.wantErr {
				assert.Error(mt, err)
				assert.Contains(mt, err.Error(), tc.errContains)
			} else {
				assert.NoError(mt, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// UpdateManyDocument tests
// ---------------------------------------------------------------------------

func TestUpdateManyDocument(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	tests := []struct {
		name        string
		responses   []bson.D
		wantErr     bool
		errContains string
	}{
		{
			name: "multiple documents match – all modified",
			responses: []bson.D{
				mtest.CreateSuccessResponse(
					bson.E{Key: "n", Value: 3},
					bson.E{Key: "nModified", Value: 3},
				),
			},
			wantErr: false,
		},
		{
			name: "no document matches – zero modified",
			responses: []bson.D{
				mtest.CreateSuccessResponse(
					bson.E{Key: "n", Value: 0},
					bson.E{Key: "nModified", Value: 0},
				),
			},
			wantErr: false,
		},
		{
			name: "mongodb error during updateMany",
			responses: []bson.D{
				mtest.CreateCommandErrorResponse(mtest.CommandError{
					Code:    11000,
					Message: "forced updateMany error",
				}),
			},
			wantErr:     true,
			errContains: "update many documents",
		},
	}

	for _, tc := range tests {
		tc := tc
		mt.Run(tc.name, func(mt *mtest.T) {
			for _, r := range tc.responses {
				mt.AddMockResponses(r)
			}

			conn := util.NewMongoConnectionFromClient(mt.Client, mt.DB.Name(), mt.Coll.Name())
			sut, err := impl.NewUpdateDocumentsImpl(context.Background(), conn, nopLogger())
			require.NoError(mt, err)

			err = sut.UpdateManyDocument(context.Background())
			if tc.wantErr {
				assert.Error(mt, err)
				assert.Contains(mt, err.Error(), tc.errContains)
			} else {
				assert.NoError(mt, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// UpdateDocumentWithCurrentDate tests
// ---------------------------------------------------------------------------

func TestUpdateDocumentWithCurrentDate(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	tests := []struct {
		name        string
		responses   []bson.D
		wantErr     bool
		errContains string
	}{
		{
			name: "matching document exists – updated with lastModified",
			responses: []bson.D{
				mtest.CreateSuccessResponse(
					bson.E{Key: "n", Value: 1},
					bson.E{Key: "nModified", Value: 1},
				),
			},
			wantErr: false,
		},
		{
			name: "no document matches – zero modified",
			responses: []bson.D{
				mtest.CreateSuccessResponse(
					bson.E{Key: "n", Value: 0},
					bson.E{Key: "nModified", Value: 0},
				),
			},
			wantErr: false,
		},
		{
			name: "mongodb error during update with date",
			responses: []bson.D{
				mtest.CreateCommandErrorResponse(mtest.CommandError{
					Code:    11000,
					Message: "forced currentDate error",
				}),
			},
			wantErr:     true,
			errContains: "update document with current date",
		},
	}

	for _, tc := range tests {
		tc := tc
		mt.Run(tc.name, func(mt *mtest.T) {
			for _, r := range tc.responses {
				mt.AddMockResponses(r)
			}

			conn := util.NewMongoConnectionFromClient(mt.Client, mt.DB.Name(), mt.Coll.Name())
			sut, err := impl.NewUpdateDocumentsImpl(context.Background(), conn, nopLogger())
			require.NoError(mt, err)

			err = sut.UpdateDocumentWithCurrentDate(context.Background())
			if tc.wantErr {
				assert.Error(mt, err)
				assert.Contains(mt, err.Error(), tc.errContains)
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
	defer mt.Close()

	mt.Run("closes client successfully", func(mt *mtest.T) {
		conn := util.NewMongoConnectionFromClient(mt.Client, mt.DB.Name(), mt.Coll.Name())
		sut, err := impl.NewUpdateDocumentsImpl(context.Background(), conn, nopLogger())
		require.NoError(mt, err)

		err = sut.Finalized(context.Background())
		assert.NoError(mt, err)
	})
}

func TestFinalized_CloseError(t *testing.T) {
	conn := util.NewMongoConnectionWithCloseError(errors.New("close failed"))
	// We need a valid client to construct; use a mock one.
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	mt.Run("close error is propagated", func(mt *mtest.T) {
		conn2 := util.NewMongoConnectionFromClientWithCloseError(
			mt.Client, mt.DB.Name(), mt.Coll.Name(), errors.New("close failed"),
		)
		sut, err := impl.NewUpdateDocumentsImpl(context.Background(), conn2, nopLogger())
		require.NoError(mt, err)

		err = sut.Finalized(context.Background())
		assert.Error(mt, err)
		assert.Contains(mt, err.Error(), "close mongo client")
	})

	// suppress unused variable warning
	_ = conn
}

// ---------------------------------------------------------------------------
// LoadMethods tests
// ---------------------------------------------------------------------------

func TestLoadMethods(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	// success response helper
	okResp := func() bson.D {
		return mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 1},
			bson.E{Key: "nModified", Value: 1},
		)
	}

	tests := []struct {
		name      string
		responses []bson.D
		wantErr   bool
		errHint   string
	}{
		{
			name: "all operations succeed",
			// UpdateOne, UpdateMany, UpdateDocumentWithCurrentDate each need one response.
			// Finalized (Close) does not need a mock response.
			responses: []bson.D{okResp(), okResp(), okResp()},
			wantErr:   false,
		},
		{
			name: "updateOne fails – sequence stops",
			responses: []bson.D{
				mtest.CreateCommandErrorResponse(mtest.CommandError{
					Code:    2,
					Message: "updateOne failure",
				}),
				// remaining responses are never consumed
				okResp(),
				okResp(),
			},
			wantErr: true,
			errHint: "update one document",
		},
		{
			name: "updateMany fails after updateOne succeeds",
			responses: []bson.D{
				okResp(), // updateOne ok
				mtest.CreateCommandErrorResponse(mtest.CommandError{
					Code:    2,
					Message: "updateMany failure",
				}),
				okResp(), // never consumed
			},
			wantErr: true,
			errHint: "update many documents",
		},
		{
			name: "updateWithCurrentDate fails after previous two succeed",
			responses: []bson.D{
				okResp(), // updateOne ok
				okResp(), // updateMany ok
				mtest.CreateCommandErrorResponse(mtest.CommandError{
					Code:    2,
					Message: "currentDate failure",
				}),
			},
			wantErr: true,
			errHint: "update document with current date",
		},
	}

	for _, tc := range tests {
		tc := tc
		mt.Run(tc.name, func(mt *mtest.T) {
			for _, r := range tc.responses {
				mt.AddMockResponses(r)
			}

			conn := util.NewMongoConnectionFromClient(mt.Client, mt.DB.Name(), mt.Coll.Name())
			sut, err := impl.NewUpdateDocumentsImpl(context.Background(), conn, nopLogger())
			require.NoError(mt, err)

			err = sut.LoadMethods(context.Background())
			if tc.wantErr {
				assert.Error(mt, err)
				if tc.errHint != "" {
					assert.Contains(mt, err.Error(), tc.errHint)
				}
			} else {
				assert.NoError(mt, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// LoadMethods ordering invariant
// ---------------------------------------------------------------------------

// callRecorder is a thin wrapper that records which high-level operations were
// dispatched. Because the impl methods are concrete (not behind an interface)
// we verify ordering indirectly through the mock response queue: the mock
// server returns responses in FIFO order, so if the impl calls operations in
// the wrong order the wrong response will be consumed by the wrong call and
// the test assertion will catch the mismatch via error presence/absence.
//
// A separate ordering test seeds the mock with responses that will only
// succeed when consumed in the right sequence.
func TestLoadMethods_OrderInvariant(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	mt.Run("operations invoked in correct order", func(mt *mtest.T) {
		// Queue: first two are success, third is a command error so we can
		// detect if the third call is UpdateDocumentWithCurrentDate (correct)
		// vs something else.
		mt.AddMockResponses(
			mtest.CreateSuccessResponse(bson.E{Key: "n", Value: 1}, bson.E{Key: "nModified", Value: 1}),