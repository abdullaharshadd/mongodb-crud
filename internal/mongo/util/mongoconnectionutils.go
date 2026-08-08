// Package util provides shared constants, lifecycle contracts, and MongoDB
// connection helpers for the MongoDB-related components of the application.
//
// MIGRATION_NOTE: This file is the Go equivalent of the Java
// com.mongo.utils.MongoConnectionUtils class. Several Java-specific patterns
// were deliberately dropped in favor of idiomatic Go:
//
//   Java pattern                          Go equivalent / decision
//   -----------------------------         -----------------------------
//   static { } block loading a            explicit LoadMongoConfig() call
//     .properties file                    (see internal/conf/mongo.properties.go)
//   per-instance lazy singleton           *mongo.Client is created once via
//     (mongoClient == null check)         Connect and owned by the caller
//   closeMongoClient(client) per-op       caller owns the client and uses
//                                         defer client.Disconnect(ctx) in main
//   getter/setter boilerplate             exported struct fields
//   host+port MongoClient(host,port)      official driver connects via a URI
//     legacy driver                       (mongo.Connect + options.ApplyURI)
//
// The legacy driver connected with a bare host/port and never used a context.
// The official Go driver (go.mongodb.org/mongo-driver) requires a context for
// all I/O and connects via a connection URI. We therefore rely on
// conf.MongoConfig.URI() to build the URI rather than mirroring the old
// host/port constructor.
package util

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"migrated-app/internal/conf"
)

// Default database and collection names, preserved from the Java source's
// getProperty defaults ("sundar" / "sample").
const (
	// DefaultDatabase is the fallback database name used when the loaded
	// configuration does not specify one.
	DefaultDatabase = "sundar"
	// DefaultCollection is the fallback collection name used when the loaded
	// configuration does not specify one.
	DefaultCollection = "sample"
)

// MongoConnection wraps an established MongoDB client together with the
// database and collection names selected from configuration.
//
// MIGRATION_NOTE: the Java class stored dataBase/sampleCollection as mutable
// fields with getters/setters. In Go we expose them as exported fields on a
// value returned from a constructor. The *mongo.Client is owned by whoever
// constructed it (typically main), which is responsible for calling
// Disconnect during graceful shutdown — there is no per-operation close.
type MongoConnection struct {
	// Client is the connected MongoDB client. Ownership (and the
	// responsibility to Disconnect) belongs to the caller.
	Client *mongo.Client
	// Database is the name of the database to use.
	Database string
	// Collection is the name of the sample collection to use.
	Collection string

	logger zerolog.Logger
}

// NewMongoConnection establishes a MongoDB client connection using the
// supplied configuration and returns a *MongoConnection.
//
// The returned connection's Client is verified with a Ping before being
// returned. The caller owns the client and must call Close (or
// Client.Disconnect) during shutdown.
//
// MIGRATION_NOTE: replaces the Java getMongoConnection() lazy-singleton. The
// lazy null-check is unnecessary in Go: construct exactly one client at
// startup and pass it where needed. Any error is returned rather than merely
// logged and swallowed as the Java code did.
func NewMongoConnection(ctx context.Context, cfg conf.MongoConfig, logger zerolog.Logger) (*MongoConnection, error) {
	uri := cfg.URI()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		logger.Error().Err(err).Msg("exception occurred while getting Mongo connection")
		return nil, fmt.Errorf("connecting to mongo: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		logger.Error().Err(err).Msg("exception occurred while pinging Mongo")
		// Best-effort cleanup of the half-open client.
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("pinging mongo: %w", err)
	}

	database := cfg.Database
	if database == "" {
		database = DefaultDatabase
	}
	collection := cfg.SampleCollection
	if collection == "" {
		collection = DefaultCollection
	}

	return &MongoConnection{
		Client:     client,
		Database:   database,
		Collection: collection,
		logger:     logger,
	}, nil
}

// Close disconnects the underlying MongoDB client.
//
// MIGRATION_NOTE: replaces the Java closeMongoClient(MongoClient) method.
// Idiomatically the caller invokes this via defer during graceful shutdown
// rather than passing a client instance back in. A nil connection or nil
// client is treated as a no-op, mirroring the original null-check.
func (c *MongoConnection) Close(ctx context.Context) error {
	if c == nil || c.Client == nil {
		return nil
	}
	if err := c.Client.Disconnect(ctx); err != nil {
		c.logger.Error().Err(err).Msg("exception occurred while closing MongoClient")
		return fmt.Errorf("disconnecting mongo: %w", err)
	}
	return nil
}