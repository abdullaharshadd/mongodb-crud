```go
// Package impl_test contains table-driven tests for UpdateDocumentsImpl.
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
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"

	"migrated-app/internal/logging"
	"migrated-app/internal/mongo/util"
	"migrated-app/internal/mongo/crud/impl"
)

// ---------------------------------------------------------------------------
// Helpers / fakes
// ---------------------------------------------------------------------------

// fakeLogger captures log messages so we can assert them in tests.
type fakeLogger struct {
	lines []string
}

func newFakeLogger() *logging.Logger {
	// We need to return the real *logging.Logger type that the impl uses.
	// If logging.Logger is a struct with exported fields or a constructor, use
	// that. Here we assume logging.NewNopLogger() or similar exists; adjust
	// to your actual package API.
	return logging.NewLogger(logging.LevelInfo, &fakeWriter{})
}

type fakeWriter struct {
	buf strings.Builder
}

func (fw *fakeWriter) Write(p []byte) (int, error) {
	return fw.buf.Write(p)
}

// fakeMongoConnectionUtils wraps a *mtest.T's collection so that
// SampleCollection() returns the mtest collection and Close() can be
// controlled.
type fakeMongoConnectionUtils struct {
	coll        *mongo.Collection
	closeErr    error
	closeCalled bool
}



// ---------------------------------------------------------------------------
// Because util.MongoConnectionUtils is a concrete struct and not an interface
// in the migrated code, we need to look at how UpdateDocumentsImpl accesses
// it. It calls u.mongo.SampleCollection() and u.mongo.Close(ctx).
//
// Strategy: define a thin interface that both the real
// *util.MongoConnectionUtils and our fake satisfy, then change the struct
// field to hold the interface. If you cannot modify the production file,
// embed a local test-only interface and use a helper constructor.
//
// For the purposes of this test file we assume the impl package exposes a
// testable seam via NewUpdateDocumentsImplWithConn (or the standard
// constructor accepting the interface). Adjust the import path / constructor
// name as needed for your project layout.
// ---------------------------------------------------------------------------

// connProvider is the minimal interface extracted for testing.
type connProvider interface {
	SampleCollection() *mongo.Collection
	Close(ctx context.Context) error
}

// ---------------------------------------------------------------------------
// We use the mtest package to get a real *mongo.Collection backed by a mock
// transport so we can inject command responses without a live server.
// ---------------------------------------------------------------------------

func TestNewUpdateDocumentsImpl(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	mt.Run("constructor returns non-nil impl", func(mt *mtest.T) {
		fake := &fakeMongoConnectionUtils{coll: mt.Coll}
		log := logging.NewNoopLogger()

		// We call the real constructor; the returned value must not be nil.
		got := impl.NewUpdateDocumentsImpl(
			// util.MongoConnectionUtils is accepted; we supply a nil here and
			// rely on the fact that the test never triggers a nil-deref through
			// our fake seam. Adjust if your code uses a real *util.MongoConnectionUtils.
			(*util.MongoConnectionUtils)(nil),
			log,
		)
		// The impl should be non-nil regardless.
		assert.NotNil(t, got)
		_ = fake // suppress unused warning
	})
}

// ---------------------------------------------------------------------------
// TestUpdateOneDocument
// ---------------------------------------------------------------------------

func TestUpdateOneDocument(t *testing.T) {
	opts := mtest.NewOptions().ClientType(mtest.Mock)
	mt := mtest.New(t, opts)
	defer mt.Close()

	tests := []struct {
		name         string
		responses    []bson.D   // mtest command responses to queue
		wantErr      bool
		wantErrMsg   string
		nilColl      bool // simulate nil collection
	}{
		{
			name: "success – one document matched and modified",
			responses: []bson.D{
				mtest.CreateSuccessResponse(
					bson.E{Key: "n", Value: int32(1)},
					bson.E{Key: "nModified", Value: int32(1)},
				),
			},
			wantErr: false,
		},
		{
			name: "success – no document matches (modified count 0)",
			responses: []bson.D{
				mtest.CreateSuccessResponse(
					bson.E{Key: "n", Value: int32(0)},
					bson.E{Key: "nModified", Value: int32(0)},
				),
			},
			wantErr: false,
		},
		{
			name:       "error – nil collection",
			nilColl:    true,
			wantErr:    true,
			wantErrMsg: "sample collection is not configured",
		},
		{
			name: "error – mongo command error",
			responses: []bson.D{
				mtest.CreateCommandErrorResponse(mtest.CommandError{
					Code:    11000,
					Message: "simulated update error",
				}),
			},
			wantErr:    true,
			wantErrMsg: "update single document",
		},
	}

	for _, tc := range tests {
		tc := tc
		mt.Run(tc.name, func(mt *mtest.T) {
			for _, r := range tc.responses {
				mt.AddMockResponses(r)
			}

			var coll *mongo.Collection
			if !tc.nilColl {
				coll = mt.Coll
			}

			fake := &fakeMongoConnectionUtils{coll: coll}
			log := logging.NewNoopLogger()
			u := newTestableImpl(fake, log)

			err := u.UpdateOneDocument(context.Background())
			if tc.wantErr {
				require.Error(t, err)
				if tc.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tc.wantErrMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestUpdateManyDocument
// ---------------------------------------------------------------------------

func TestUpdateManyDocument(t *testing.T) {
	opts := mtest.NewOptions().ClientType(mtest.Mock)
	mt := mtest.New(t, opts)
	defer mt.Close()

	tests := []struct {
		name       string
		responses  []bson.D
		wantErr    bool
		wantErrMsg string
		nilColl    bool
	}{
		{
			name: "success – multiple documents matched and modified",
			responses: []bson.D{
				mtest.CreateSuccessResponse(
					bson.E{Key: "n", Value: int32(3)},
					bson.E{Key: "nModified", Value: int32(3)},
				),
			},
			wantErr: false,
		},
		{
			name: "success – no documents match",
			responses: []bson.D{
				mtest.CreateSuccessResponse(
					bson.E{Key: "n", Value: int32(0)},
					bson.E{Key: "nModified", Value: int32(0)},
				),
			},
			wantErr: false,
		},
		{
			name:       "error – nil collection",
			nilColl:    true,
			wantErr:    true,
			wantErrMsg: "sample collection is not configured",
		},
		{
			name: "error – mongo command error",
			responses: []bson.D{
				mtest.CreateCommandErrorResponse(mtest.CommandError{
					Code:    2,
					Message: "simulated updateMany error",
				}),
			},
			wantErr:    true,
			wantErrMsg: "update many documents",
		},
	}

	for _, tc := range tests {
		tc := tc
		mt.Run(tc.name, func(mt *mtest.T) {
			for _, r := range tc.responses {
				mt.AddMockResponses(r)
			}

			var coll *mongo.Collection
			if !tc.nilColl {
				coll = mt.Coll
			}

			fake := &fakeMongoConnectionUtils{coll: coll}
			log := logging.NewNoopLogger()
			u := newTestableImpl(fake, log)

			err := u.UpdateManyDocument(context.Background())
			if tc.wantErr {
				require.Error(t, err)
				if tc.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tc.wantErrMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestUpdateDocumentWithCurrentDate
// ---------------------------------------------------------------------------

func TestUpdateDocumentWithCurrentDate(t *testing.T) {
	opts := mtest.NewOptions().ClientType(mtest.Mock)
	mt := mtest.New(t, opts)
	defer mt.Close()

	tests := []struct {
		name       string
		responses  []bson.D
		wantErr    bool
		wantErrMsg string
		nilColl    bool
	}{
		{
			name: "success – document matched and lastModified stamped",
			responses: []bson.D{
				mtest.CreateSuccessResponse(
					bson.E{Key: "n", Value: int32(1)},
					bson.E{Key: "nModified", Value: int32(1)},
				),
			},
			wantErr: false,
		},
		{
			name: "success – no document matches",
			responses: []bson.D{
				mtest.CreateSuccessResponse(
					bson.E{Key: "n", Value: int32(0)},
					bson.E{Key: "nModified", Value: int32(0)},
				),
			},
			wantErr: false,
		},
		{
			name:       "error – nil collection",
			nilColl:    true,
			wantErr:    true,
			wantErrMsg: "sample collection is not configured",
		},
		{
			name: "error – mongo command error",
			responses: []bson.D{
				mtest.CreateCommandErrorResponse(mtest.CommandError{
					Code:    2,
					Message: "simulated currentDate error",
				}),
			},
			wantErr:    true,
			wantErrMsg: "update document with current date",
		},
	}

	for _, tc := range tests {
		tc := tc
		mt.Run(tc.name, func(mt *mtest.T) {
			for _, r := range tc.responses {
				mt.AddMockResponses(r)
			}

			var coll *mongo.Collection
			if !tc.nilColl {
				coll = mt.Coll
			}

			fake := &fakeMongoConnectionUtils{coll: coll}
			log := logging.NewNoopLogger()
			u := newTestableImpl(fake, log)

			err := u.UpdateDocumentWithCurrentDate(context.Background())
			if tc.wantErr {
				require.Error(t, err)
				if tc.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tc.wantErrMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestFinalized
// ---------------------------------------------------------------------------


// ---------------------------------------------------------------------------
// TestLoadMethods – orchestration order and error propagation
// ---------------------------------------------------------------------------

