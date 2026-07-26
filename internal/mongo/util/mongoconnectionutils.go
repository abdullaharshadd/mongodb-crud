// Package util provides shared constants and lifecycle contracts for the
// MongoDB utility implementations, along with the connection helper that
// opens and closes MongoDB client connections.
//
// MIGRATION_NOTE: The Java source (MongoConnectionUtils) used a lazily cached
// *singleton* MongoClient held in an instance field, a static initializer to
// eagerly load a properties file, and mutable getter/setter accessors for the
// database and collection names. In idiomatic Go these concerns are expressed
// differently:
//
//   - The properties file loading is delegated to the already-migrated
//     internal/conf.LoadMongoConfig / MongoConfig.URI helpers rather than
//     re-parsing a *.properties file here.
//   - Instead of a package-level mutable singleton client, we expose a
//     MongoConnectionUtils value constructed via NewMongoConnectionUtils and
//     an explicit Connect(ctx) method. The caller owns the returned client's
//     lifetime and calls Close(ctx) for a graceful shutdown. This avoids the
//     hidden global state and non-thread-safe null-check caching of the
//     original.
//   - Every I/O operation takes a context.Context as its first parameter and
//     returns an error instead of logging-and-swallowing as the Java code did.
//   - The legacy MongoClient(host, port) constructor is replaced by the
//     modern driver's URI-based connection (mongo.Connect with
//     options.Client().ApplyURI(...)), matching the target MongoDB driver
//     idioms.
package util

import (
	"context"
	"fmt"
	"sync"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"migrated-app/internal/conf"
)

// Default database and collection names, preserved from the Java source's
// getProperty(...) fallback values ("sundar" / "sample").
const (
	// DefaultDatabase is the fallback database name used when the
	// configuration does not specify one.
	DefaultDatabase = "sundar"
	// DefaultSampleCollection is the fallback collection name used when the
	// configuration does not specify one.
	DefaultSampleCollection = "sample"
)

// MongoConnectionUtils manages the lifecycle of a MongoDB client connection.
//
// It replaces the Java class of the same name. The zero value is not usable;
// construct instances with NewMongoConnectionUtils.
type MongoConnectionUtils struct {
	cfg *conf.MongoConfig

	mu     sync.Mutex
	client *mongo.Client

	database         string
	sampleCollection string
}

// NewMongoConnectionUtils constructs a MongoConnectionUtils from the given
// configuration. If cfg is nil, the configuration is loaded via
// conf.LoadMongoConfig.
func NewMongoConnectionUtils(cfg *conf.MongoConfig) (*MongoConnectionUtils, error) {
	if cfg == nil {
		loaded, err := conf.LoadMongoConfig()
		if err != nil {
			return nil, fmt.Errorf("load mongo config: %w", err)
		}
		cfg = loaded
	}

	return &MongoConnectionUtils{
		cfg:              cfg,
		database:         DefaultDatabase,
		sampleCollection: DefaultSampleCollection,
	}, nil
}

// Database returns the configured database name (defaulting to
// DefaultDatabase). It replaces the Java getDataBase() accessor.
func (m *MongoConnectionUtils) Database() string {
	return m.database
}

// SetDatabase overrides the database name. It replaces the Java
// setDataBase(String) mutator.
func (m *MongoConnectionUtils) SetDatabase(database string) {
	m.database = database
}

// SampleCollection returns the configured sample collection name (defaulting
// to DefaultSampleCollection). It replaces the Java getSampleCollection()
// accessor.
func (m *MongoConnectionUtils) SampleCollection() string {
	return m.sampleCollection
}

// SetSampleCollection overrides the sample collection name. It replaces the
// Java setSampleCollection(String) mutator.
func (m *MongoConnectionUtils) SetSampleCollection(sampleCollection string) {
	m.sampleCollection = sampleCollection
}

// Connect returns a connected MongoDB client, creating and caching it on the
// first call. Subsequent calls return the cached client. It replaces the Java
// getMongoConnection() method.
//
// Unlike the Java original, connection failures are returned to the caller
// rather than logged and swallowed, and access to the cached client is
// synchronized so it is safe for concurrent use.
func (m *MongoConnectionUtils) Connect(ctx context.Context) (*mongo.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.client != nil {
		return m.client, nil
	}

	uri := m.cfg.URI()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("connect to mongo at %q: %w", uri, err)
	}

	// Verify the connection is actually usable before caching it. The legacy
	// driver established a connection eagerly in its constructor; the modern
	// driver connects lazily, so we ping to surface errors early.
	if err := client.Ping(ctx, nil); err != nil {
		// Best-effort cleanup of the half-open client.
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("ping mongo at %q: %w", uri, err)
	}

	m.client = client
	return m.client, nil
}

// Close disconnects the given MongoDB client. It replaces the Java
// closeMongoClient(MongoClient) method. A nil client is a no-op, matching the
// original null-check behaviour.
//
// If the client passed is the one cached by this instance, the cache is
// cleared so a later Connect call establishes a fresh connection.
func (m *MongoConnectionUtils) Close(ctx context.Context, client *mongo.Client) error {
	if client == nil {
		return nil
	}

	if err := client.Disconnect(ctx); err != nil {
		return fmt.Errorf("disconnect mongo client: %w", err)
	}

	m.mu.Lock()
	if m.client == client {
		m.client = nil
	}
	m.mu.Unlock()

	return nil
}
