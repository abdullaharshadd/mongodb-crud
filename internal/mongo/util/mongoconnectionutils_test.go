```go
package util

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.mongodb.org/mongo-driver/mongo/options"

	"migrated-app/internal/conf"
)

// ---------------------------------------------------------------------------
// Helpers / fakes
// ---------------------------------------------------------------------------

// newTestConfig returns a *conf.MongoConfig pointing at the mtest MongoDB URI
// (or a local standalone) so tests that need a real driver round-trip can use it.
func newTestConfig(t *testing.T) *conf.MongoConfig {
	t.Helper()
	cfg, err := conf.NewMongoConfig("mongodb://localhost:27017")
	if err != nil {
		t.Skipf("cannot build test MongoConfig: %v", err)
	}
	return cfg
}

// newTestUtils builds a MongoConnectionUtils with an explicit config so no
// file loading is needed.
func newTestUtils(t *testing.T, cfg *conf.MongoConfig) *MongoConnectionUtils {
	t.Helper()
	u, err := NewMongoConnectionUtils(cfg)
	require.NoError(t, err)
	return u
}

// ---------------------------------------------------------------------------
// Tests for NewMongoConnectionUtils
// ---------------------------------------------------------------------------

func TestNewMongoConnectionUtils_NilConfig_LoadsFromFile(t *testing.T) {
	// We cannot guarantee a properties file exists in the test environment,
	// so we only assert that either a valid struct is returned or an error
	// that wraps "load mongo config".
	u, err := NewMongoConnectionUtils(nil)
	if err != nil {
		assert.Contains(t, err.Error(), "load mongo config")
		return
	}
	require.NotNil(t, u)
	assert.Equal(t, DefaultDatabase, u.Database())
	assert.Equal(t, DefaultSampleCollection, u.SampleCollection())
}


// ---------------------------------------------------------------------------
// Tests for Database / SetDatabase
// ---------------------------------------------------------------------------

func TestDatabase_DefaultValue(t *testing.T) {
	u := newTestUtils(t, newTestConfig(t))
	assert.Equal(t, DefaultDatabase, u.Database())
}

func TestSetDatabase_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		setValue string
		want     string
	}{
		{
			name:     "set non-empty string",
			setValue: "mydb",
			want:     "mydb",
		},
		{
			name:     "set empty string",
			setValue: "",
			want:     "",
		},
		{
			name:     "overwrite previous value",
			setValue: "another",
			want:     "another",
		},
		{
			name:     "unicode value",
			setValue: "データベース",
			want:     "データベース",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			u := newTestUtils(t, newTestConfig(t))
			u.SetDatabase(tc.setValue)
			assert.Equal(t, tc.want, u.Database(),
				"Database() should return the value passed to SetDatabase()")
		})
	}
}


// ---------------------------------------------------------------------------
// Tests for SampleCollection / SetSampleCollection
// ---------------------------------------------------------------------------

func TestSampleCollection_DefaultValue(t *testing.T) {
	u := newTestUtils(t, newTestConfig(t))
	assert.Equal(t, DefaultSampleCollection, u.SampleCollection())
}

func TestSetSampleCollection_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		setValue string
		want     string
	}{
		{
			name:     "set non-empty string",
			setValue: "myCollection",
			want:     "myCollection",
		},
		{
			name:     "set empty string",
			setValue: "",
			want:     "",
		},
		{
			name:     "overwrite previous value",
			setValue: "yetAnother",
			want:     "yetAnother",
		},
		{
			name:     "unicode value",
			setValue: "コレクション",
			want:     "コレクション",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			u := newTestUtils(t, newTestConfig(t))
			u.SetSampleCollection(tc.setValue)
			assert.Equal(t, tc.want, u.SampleCollection(),
				"SampleCollection() should return the value passed to SetSampleCollection()")
		})
	}
}


// ---------------------------------------------------------------------------
// Tests for Connect using mtest (mock MongoDB)
// ---------------------------------------------------------------------------

func TestConnect_ReturnsClient_OnSuccess(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("first connect creates and caches client", func(mt *mtest.T) {
		cfg, err := conf.NewMongoConfig(mt.Client.Database("admin").Client().(*mongo.Client).NumberSessionsInProgress)
		// mtest already provides a connected client; we simulate by injecting
		// a config that targets the mock server.
		_ = cfg
		_ = err

		// Because mtest's internal URI is not directly accessible via a simple
		// conf.NewMongoConfig call without package-internal details, we test
		// Connect indirectly by constructing the client ourselves and asserting
		// the cache behaviour through the mutex/field inspection.
		//
		// The canonical path: inject the mock client directly into the struct.
		u := &MongoConnectionUtils{
			cfg:              mustNewMongoConfig(mt.T, "mongodb://localhost:27017"),
			database:         DefaultDatabase,
			sampleCollection: DefaultSampleCollection,
		}

		// Pre-inject a mock client to simulate a successful Connect cache hit.
		mockClient := mt.Client
		u.mu.Lock()
		u.client = mockClient
		u.mu.Unlock()

		ctx := context.Background()
		got, err := u.Connect(ctx)
		require.NoError(mt.T, err)
		assert.Same(mt.T, mockClient, got,
			"second call should return the cached client")
	})
}





// ---------------------------------------------------------------------------
// Tests for Close
// ---------------------------------------------------------------------------

func TestClose_NilClient_IsNoop(t *testing.T) {
	u := newTestUtils(t, newTestConfig(t))
	err := u.Close(context.Background(), nil)
	assert.NoError(t, err, "Close(nil) must be a no-op and return nil")
}



// ---------------------------------------------------------------------------
// Tests for constant values
// ---------------------------------------------------------------------------

func TestDefaultConstants(t *testing.T) {
	assert.Equal(t, "sundar", DefaultDatabase,
		"DefaultDatabase must match the Java fallback 'sundar'")
	assert.Equal(t, "sample", DefaultSampleCollection,
		"DefaultSampleCollection must match the Java fallback 'sample'")
}

// ---------------------------------------------------------------------------
// Integration-style: full lifecycle Connect → Close
// ---------------------------------------------------------------------------

func TestConnectClose_FullLifecycle(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("connect then close then reconnect", func(mt *mtest.T) {
		u := &MongoConnectionUtils{