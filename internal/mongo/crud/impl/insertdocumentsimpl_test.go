```go
package impl_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"

	"github.com/example/app/internal/conf"
	"github.com/example/app/internal/mongo/crud/impl"
	"github.com/example/app/internal/mongo/util"
)

// ---------------------------------------------------------------------------
// Minimal logger stub
// ---------------------------------------------------------------------------

type stubLogger struct {
	infos  []string
	errors []string
}

func (s *stubLogger) Info(msg string) {
	s.infos = append(s.infos, msg)
}

func (s *stubLogger) Error(msg string) {
	s.errors = append(s.errors, msg)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newCfg() *conf.MongoConfig {
	return &conf.MongoConfig{
		Database:         "testdb",
		SampleCollection: "testcol",
	}
}

// buildImpl wires a real mtest client into a MongoConnection stub and returns
// a ready InsertDocumentsImpl plus the logger so tests can inspect log calls.
func buildImpl(t *testing.T, mt *mtest.T) (*impl.InsertDocumentsImpl, *stubLogger) {
	t.Helper()
	log := &stubLogger{}
	conn := util.NewMongoConnectionFromClient(mt.Client) // see stub below
	cfg := &conf.MongoConfig{
		Database:         mt.DB.Name(),
		SampleCollection: "testcol",
	}
	ins, err := impl.NewInsertDocumentsImpl(conn, cfg, log)
	require.NoError(t, err)
	return ins, log
}

// ---------------------------------------------------------------------------
// Constructor tests
// ---------------------------------------------------------------------------

func TestNewInsertDocumentsImpl(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	mt.Run("nil connection returns error", func(mt *mtest.T) {
		cfg := newCfg()
		_, err := impl.NewInsertDocumentsImpl(nil, cfg, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connection must not be nil")
	})

	mt.Run("nil config returns error", func(mt *mtest.T) {
		conn := util.NewMongoConnectionFromClient(mt.Client)
		_, err := impl.NewInsertDocumentsImpl(conn, nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config must not be nil")
	})

	mt.Run("valid inputs return non-nil instance", func(mt *mtest.T) {
		conn := util.NewMongoConnectionFromClient(mt.Client)
		cfg := newCfg()
		ins, err := impl.NewInsertDocumentsImpl(conn, cfg, nil)
		assert.NoError(t, err)
		assert.NotNil(t, ins)
	})

	mt.Run("logger may be nil without panicking", func(mt *mtest.T) {
		conn := util.NewMongoConnectionFromClient(mt.Client)
		cfg := newCfg()
		ins, err := impl.NewInsertDocumentsImpl(conn, cfg, nil)
		assert.NoError(t, err)
		assert.NotNil(t, ins)
	})
}

// ---------------------------------------------------------------------------
// InsertUsingDocument tests
// ---------------------------------------------------------------------------

func TestInsertUsingDocument(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	tests := []struct {
		name        string
		responses   []bson.D
		wantErr     bool
		errContains string
		wantLog     bool
	}{
		{
			name:      "success inserts document and logs",
			responses: []bson.D{mtest.CreateSuccessResponse()},
			wantErr:   false,
			wantLog:   true,
		},
		{
			name: "mongo failure returns wrapped error",
			responses: []bson.D{
				mtest.CreateWriteErrorsResponse(mtest.WriteError{
					Index:   0,
					Code:    11000,
					Message: "duplicate key error",
				}),
			},
			wantErr:     true,
			errContains: "insert using Document",
			wantLog:     false,
		},
	}

	for _, tc := range tests {
		tc := tc
		mt.Run(tc.name, func(mt *mtest.T) {
			for _, r := range tc.responses {
				mt.AddMockResponses(r)
			}
			ins, log := buildImpl(t, mt)
			err := ins.InsertUsingDocument(context.Background())
			if tc.wantErr {
				assert.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
			if tc.wantLog {
				assert.NotEmpty(t, log.infos)
				found := false
				for _, msg := range log.infos {
					if msg == "Document Insert Successfully using Document Obj..." {
						found = true
					}
				}
				assert.True(t, found, "expected success log message not found")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// InsertUsingMap tests
// ---------------------------------------------------------------------------

func TestInsertUsingMap(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	tests := []struct {
		name        string
		responses   []bson.D
		wantErr     bool
		errContains string
		wantLog     bool
	}{
		{
			name:      "success inserts map document and logs",
			responses: []bson.D{mtest.CreateSuccessResponse()},
			wantErr:   false,
			wantLog:   true,
		},
		{
			name: "mongo failure returns wrapped error",
			responses: []bson.D{
				mtest.CreateWriteErrorsResponse(mtest.WriteError{
					Index:   0,
					Code:    11000,
					Message: "duplicate key error",
				}),
			},
			wantErr:     true,
			errContains: "insert using Map",
			wantLog:     false,
		},
	}

	for _, tc := range tests {
		tc := tc
		mt.Run(tc.name, func(mt *mtest.T) {
			for _, r := range tc.responses {
				mt.AddMockResponses(r)
			}
			ins, log := buildImpl(t, mt)
			err := ins.InsertUsingMap(context.Background())
			if tc.wantErr {
				assert.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
			if tc.wantLog {
				assert.NotEmpty(t, log.infos)
				successFound := false
				employDetailsFound := false
				for _, msg := range log.infos {
					if msg == "Document Insert Successfully using Map..." {
						successFound = true
					}
					if len(msg) > 0 && msg[:len("Employ Details")] == "Employ Details" {
						employDetailsFound = true
					}
				}
				assert.True(t, successFound, "expected success log message")
				assert.True(t, employDetailsFound, "expected employee details log message")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// InsertSingleDocument tests
// ---------------------------------------------------------------------------

func TestInsertSingleDocument(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	tests := []struct {
		name        string
		responses   []bson.D
		wantErr     bool
		errContains string
		wantLog     bool
	}{
		{
			name:      "success inserts canvas document and logs",
			responses: []bson.D{mtest.CreateSuccessResponse()},
			wantErr:   false,
			wantLog:   true,
		},
		{
			name: "mongo failure returns wrapped error",
			responses: []bson.D{
				mtest.CreateWriteErrorsResponse(mtest.WriteError{
					Index:   0,
					Code:    11000,
					Message: "insert error",
				}),
			},
			wantErr:     true,
			errContains: "insert single document",
			wantLog:     false,
		},
	}

	for _, tc := range tests {
		tc := tc
		mt.Run(tc.name, func(mt *mtest.T) {
			for _, r := range tc.responses {
				mt.AddMockResponses(r)
			}
			ins, log := buildImpl(t, mt)
			err := ins.InsertSingleDocument(context.Background())
			if tc.wantErr {
				assert.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
			if tc.wantLog {
				assert.NotEmpty(t, log.infos)
				found := false
				for _, msg := range log.infos {
					if msg == "Single Document Insert Successfully..." {
						found = true
					}
				}
				assert.True(t, found, "expected success log message not found")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// InsertMultipleDocuments tests
// ---------------------------------------------------------------------------

func TestInsertMultipleDocuments(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	tests := []struct {
		name        string
		responses   []bson.D
		wantErr     bool
		errContains string
		wantLog     bool
	}{
		{
			name:      "success inserts three documents and logs",
			responses: []bson.D{mtest.CreateSuccessResponse()},
			wantErr:   false,
			wantLog:   true,
		},
		{
			name: "mongo failure returns wrapped error",
			responses: []bson.D{
				mtest.CreateWriteErrorsResponse(mtest.WriteError{
					Index:   0,
					Code:    11000,
					Message: "batch insert error",
				}),
			},
			wantErr:     true,
			errContains: "insert multiple documents",
			wantLog:     false,
		},
	}

	for _, tc := range tests {
		tc := tc
		mt.Run(tc.name, func(mt *mtest.T) {
			for _, r := range tc.responses {
				mt.AddMockResponses(r)
			}
			ins, log := buildImpl(t, mt)
			err := ins.InsertMultipleDocuments(context.Background())
			if tc.wantErr {
				assert.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
			if tc.wantLog {
				assert.NotEmpty(t, log.infos)
				found := false
				for _, msg := range log.infos {
					if msg == "Multiple Document Insert Successfully..." {
						found = true
					}
				}
				assert.True(t, found, "expected success log message not found")
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

	mt.Run("closes the mongo client without error", func(mt *mtest.T) {
		ins, _ := buildImpl(t, mt)
		err := ins.Finalized(context.Background())
		assert.NoError(t, err)
	})
}

// ---------------------------------------------------------------------------
// LoadMethods tests
// ---------------------------------------------------------------------------

func TestLoadMethods(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	mt.Run("calls all insert methods in order and then finalizes", func(mt *mtest.T) {
		// Provide one success response per insert call (4 total) plus the
		// implicit close (no response needed).
		mt.AddMockResponses(
			mtest.CreateSuccessResponse(), // InsertUsingDocument
			mtest.CreateSuccessResponse(), // InsertUsingMap
			mtest.CreateSuccessResponse(), // InsertSingleDocument
			mtest.CreateSuccessResponse(), // InsertMultipleDocuments
		)
		ins, log := buildImpl(t, mt)
		err := ins.LoadMethods(context.Background())
		assert.NoError(t, err)

		// All four success messages should appear in the log.
		expectedMessages := []string{
			"Document Insert Successfully using Document Obj...",
			"Document Insert Successfully using Map...",
			"Single Document Insert Successfully...",
			"Multiple Document Insert Successfully...",
		}
		for _, want := range expectedMessages {
			found := false
			for _, got := range log.infos {
				if got == want {
					found = true
					break
				}
			}
			assert.True(t, found, "missing log message: %s", want)
		}
	})

	mt.Run("stops at first insert error and returns wrapped error", func(mt *mtest.T) {
		// First call fails → LoadMethods should return immediately.
		mt.AddMockResponses(
			mtest.CreateWriteErrorsResponse(mtest.WriteError{
				Index:   0,
				Code:    11000,
				Message: "dup key",
			}),
		)
		ins, _ := buildImpl(t, mt)
		err := ins.LoadMethods(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "insert using Document")
	})

	mt.Run("stops at InsertUsingMap error", func(mt *mtest.T) {
		mt.AddMockResponses(
			mtest.CreateSuccessResponse(), // InsertUsingDocument OK
			mtest.CreateWriteErrorsResponse(mtest.WriteError{ // InsertUsingMap fails
				Index:   0,
				Code:    11000,
				Message: "dup key",
			}),
		)
		ins, _ := buildImpl(t, mt)
		err := ins.LoadMethods(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "insert using Map")
	})

	mt.Run("stops at InsertSingleDocument error", func(mt *mtest.T) {
		mt.AddMockResponses(
			mtest.CreateSuccessResponse(), // InsertUsingDocument OK
			mtest.CreateSuccessResponse(), // InsertUsingMap OK
			mtest.CreateWriteErrorsResponse(mtest.WriteError{ // InsertSingleDocument fails
				Index:   0,
				Code:    11000,
				Message: "dup key",
			}),
		)
		ins, _ := buildImpl(t, mt)
		err := ins.LoadMethods(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "insert single document")
	})

	mt.