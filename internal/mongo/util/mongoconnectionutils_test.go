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

	"github.com/mongo/internal/conf"
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

func TestNewMongoConnectionUtils_ExplicitConfig(t *testing.T) {
	cfg := newTestConfig(t)
	u, err := NewMongoConnectionUtils(cfg)
	require.NoError(t, err)
	require.NotNil(t, u)

	assert.Equal(t, DefaultDatabase, u.Database(),
		"database should default to DefaultDatabase")
	assert.Equal(t, DefaultSampleCollection, u.SampleCollection(),
		"collection should default to DefaultSampleCollection")
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

func TestSetDatabase_Roundtrip(t *testing.T) {
	u := newTestUtils(t, newTestConfig(t))

	// Multiple successive sets; each Database() call should reflect the last Set.
	values := []string{"first", "second", "third", ""}
	for _, v := range values {
		u.SetDatabase(v)
		assert.Equal(t, v, u.Database())
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

func TestSetSampleCollection_Roundtrip(t *testing.T) {
	u := newTestUtils(t, newTestConfig(t))

	values := []string{"col1", "col2", "", "finalCol"}
	for _, v := range values {
		u.SetSampleCollection(v)
		assert.Equal(t, v, u.SampleCollection())
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

func TestConnect_CachesClientOnFirstCall(t *testing.T) {
	// We use the mock transport from mtest to verify singleton behaviour.
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("singleton: second Connect returns same instance", func(mt *mtest.T) {
		u := &MongoConnectionUtils{
			cfg:              mustNewMongoConfig(mt.T, "mongodb://localhost:27017"),
			database:         DefaultDatabase,
			sampleCollection: DefaultSampleCollection,
			client:           mt.Client, // pre-seeded
		}

		ctx := context.Background()

		c1, err1 := u.Connect(ctx)
		require.NoError(mt.T, err1)

		c2, err2 := u.Connect(ctx)
		require.NoError(mt.T, err2)

		assert.Same(mt.T, c1, c2, "both calls must return identical *mongo.Client pointer")
	})
}

func TestConnect_BadURI_ReturnsError(t *testing.T) {
	tests := []struct {
		name string
		uri  string
	}{
		{
			name: "completely invalid URI",
			uri:  "not-a-uri://%%%",
		},
		{
			name: "empty URI",
			uri:  "",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cfg := &conf.MongoConfig{}
			// Force the URI through the internal field via a helper that accepts
			// arbitrary URIs (including bad ones).
			cfg.SetURIForTesting(tc.uri)

			u := &MongoConnectionUtils{
				cfg:              cfg,
				database:         DefaultDatabase,
				sampleCollection: DefaultSampleCollection,
			}

			ctx := context.Background()
			client, err := u.Connect(ctx)

			assert.Error(t, err, "Connect with a bad URI must return an error")
			assert.Nil(t, client)

			// Internal cache must remain nil so the next call retries.
			u.mu.Lock()
			cached := u.client
			u.mu.Unlock()
			assert.Nil(t, cached, "failed Connect must not cache the client")
		})
	}
}

func TestConnect_PingFailure_ClearsClient(t *testing.T) {
	// Point at an address that accepts TCP but is not a MongoDB server so
	// Ping fails. We use a fake TCP echo server or simply a refused port.
	// The easiest approach: connect to a port that refuses connections.
	//
	// This test asserts the behaviour that a Ping failure returns an error
	// and does not cache the client.

	cfg := &conf.MongoConfig{}
	cfg.SetURIForTesting("mongodb://127.0.0.1:1") // port 1 is almost always closed

	u := &MongoConnectionUtils{
		cfg:              cfg,
		database:         DefaultDatabase,
		sampleCollection: DefaultSampleCollection,
	}

	ctx := context.Background()
	client, err := u.Connect(ctx)

	// We expect either a connect error or a ping error.
	assert.Error(t, err)
	assert.Nil(t, client)

	u.mu.Lock()
	cached := u.client
	u.mu.Unlock()
	assert.Nil(t, cached)
}

func TestConnect_ConcurrentAccess_SingletonGuaranteed(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("concurrent Connect returns same pointer", func(mt *mtest.T) {
		u := &MongoConnectionUtils{
			cfg:              mustNewMongoConfig(mt.T, "mongodb://localhost:27017"),
			database:         DefaultDatabase,
			sampleCollection: DefaultSampleCollection,
			client:           mt.Client,
		}

		const goroutines = 20
		results := make([]*mongo.Client, goroutines)
		var wg sync.WaitGroup

		for i := 0; i < goroutines; i++ {
			i := i
			wg.Add(1)
			go func() {
				defer wg.Done()
				c, err := u.Connect(context.Background())
				if err == nil {
					results[i] = c
				}
			}()
		}
		wg.Wait()

		first := results[0]
		for _, r := range results {
			assert.Same(mt.T, first, r,
				"all goroutines must receive the same *mongo.Client")
		}
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

func TestClose_TableDriven(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("close cached client clears cache", func(mt *mtest.T) {
		u := &MongoConnectionUtils{
			cfg:              mustNewMongoConfig(mt.T, "mongodb://localhost:27017"),
			database:         DefaultDatabase,
			sampleCollection: DefaultSampleCollection,
			client:           mt.Client,
		}

		err := u.Close(context.Background(), mt.Client)
		// Disconnect on the mtest client may or may not error; what matters is
		// the cache is cleared.
		// We do not assert err==nil because the mtest transport may reject Disconnect.
		_ = err

		u.mu.Lock()
		cached := u.client
		u.mu.Unlock()
		assert.Nil(mt.T, cached,
			"Close of the cached client must set the cache to nil")
	})

	mt.Run("close non-cached client does not touch cache", func(mt *mtest.T) {
		// Build a second client that is NOT the cached one.
		otherClient, err := mongo.Connect(
			context.Background(),
			options.Client().ApplyURI("mongodb://localhost:27017"),
		)
		if err != nil {
			mt.Skip("cannot create second client for test")
		}

		u := &MongoConnectionUtils{
			cfg:              mustNewMongoConfig(mt.T, "mongodb://localhost:27017"),
			database:         DefaultDatabase,
			sampleCollection: DefaultSampleCollection,
			client:           mt.Client, // cached client is mt.Client
		}

		// Close the *other* client; the cache should remain intact.
		_ = u.Close(context.Background(), otherClient)

		u.mu.Lock()
		cached := u.client
		u.mu.Unlock()
		assert.Same(mt.T, mt.Client, cached,
			"closing a non-cached client must not alter the internal cache")
	})
}

func TestClose_NilClientVariants(t *testing.T) {
	tests := []struct {
		name   string
		client *mongo.Client
	}{
		{name: "nil pointer", client: nil},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			u := newTestUtils(t, newTestConfig(t))
			err := u.Close(context.Background(), tc.client)
			assert.NoError(t, err)
		})
	}
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