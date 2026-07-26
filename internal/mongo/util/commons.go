// Package util provides shared constants and lifecycle contracts for the
// MongoDB utility implementations.
//
// MIGRATION_NOTE: The source was a Java "constant interface" (an anti-pattern
// where an interface exists only to expose `public static final` fields plus
// two lifecycle methods, loadMethods() and finalized()). Go has no notion of
// interface constants, so the two shared strings are expressed as ordinary
// package-level constants. The lifecycle contract (loadMethods/finalized) is
// preserved as a small Go interface, but each method now returns an error so
// implementations can report failures instead of relying on side effects and
// panics — the idiomatic Go replacement for Java's void lifecycle hooks.
//
// NOTE: The MongoProperties path constant is retained for reference only.
// Connection configuration is now handled by internal/conf (MongoConfig /
// LoadMongoConfig), which sources values from environment variables via viper
// rather than a fixed ./conf/mongo.properties file. Prefer conf.LoadMongoConfig
// over reading the path below directly.
package util

import "context"

const (
	// InvalidMsg is the error message shown when an operation is invoked with
	// an unrecognized action selector. Migrated from Commons.INVALID_MSG.
	InvalidMsg = "Invalid Argument(s) \n1 - Read / 2 - Write / 3 - Update / 4 - Delete"

	// MongoProperties is the legacy relative path to the MongoDB properties
	// file used by the original Java application. Migrated from
	// Commons.MONGO_PROPERTIES.
	//
	// MIGRATION_NOTE: This path is preserved for historical/documentation
	// purposes only. New code should use internal/conf.LoadMongoConfig, which
	// reads configuration from the environment.
	MongoProperties = "./conf/mongo.properties"
)

// Commons is the lifecycle contract implemented by MongoDB utility types.
//
// MIGRATION_NOTE: The Java interface declared void loadMethods() and
// void finalized(). Both now accept a context.Context (for cancellation and
// deadlines during I/O) and return an error so callers can handle failures
// explicitly rather than discovering them via unchecked exceptions.
type Commons interface {
	// LoadMethods performs the setup/initialization step for an implementation.
	// It corresponds to the Java loadMethods() hook.
	LoadMethods(ctx context.Context) error

	// Finalized performs teardown/cleanup for an implementation. It corresponds
	// to the Java finalized() hook. Implementations should release any acquired
	// resources (e.g. Mongo client connections) here.
	Finalized(ctx context.Context) error
}
