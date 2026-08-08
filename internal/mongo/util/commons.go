// Package util provides shared constants and lifecycle contracts for the
// MongoDB-related components of the application.
//
// MIGRATION_NOTE: The Java source (com.mongo.utils.Commons) used the
// "constant interface" anti-pattern — an interface that bundled together
// unrelated public/static/final constants AND two lifecycle method
// signatures (loadMethods / finalized). Go has no notion of interface-hosted
// constants, and mixing constants with a behavioral contract is not
// idiomatic. We therefore split the single Java interface into:
//
//   1. Package-level constants (INVALIDMsg, MongoPropertiesPath) — the shared
//      strings, transcribed verbatim.
//   2. A Lifecycle interface capturing the two behavioral methods, adapted to
//      idiomatic Go signatures (context + error returns).
//
// MIGRATION_NOTE: MongoPropertiesPath is retained for reference/parity with
// the original file path, but the canonical way to obtain configuration in
// this codebase is conf.LoadMongoConfig (see internal/conf/mongo.properties.go),
// which loads typed config via viper rather than reading a raw .properties
// file. Prefer that over reading the path directly.
package util

import "context"

const (
	// INVALIDMsg is the user-facing message emitted when an unrecognized
	// operation argument is supplied. The valid operations are:
	// 1 - Read, 2 - Write, 3 - Update, 4 - Delete.
	//
	// MIGRATION_NOTE: In the Java code this string was surfaced to the user
	// (the "userMsg indirection" — historically via System.out). In Go,
	// prefer returning this as an error value to the caller rather than
	// printing it directly; the presentation layer (or a zerolog logger from
	// internal/log4j.xml.go) decides how to emit it.
	INVALIDMsg = "Invalid Argument(s) \n1 - Read / 2 - Write / 3 - Update / 4 - Delete"

	// MongoPropertiesPath mirrors the legacy location of the Mongo
	// properties file. Retained for parity; prefer conf.LoadMongoConfig.
	MongoPropertiesPath = "./conf/mongo.properties"
)

// Lifecycle describes the initialization and teardown contract that
// MongoDB-related components implement. It replaces the loadMethods /
// finalized methods declared on the original Java Commons interface.
//
// MIGRATION_NOTE: The Java methods returned void and took no arguments.
// Idiomatic Go threads a context.Context through any operation that may
// perform I/O (such as opening a Mongo connection) and returns an error
// instead of relying on side effects or unchecked failures.
type Lifecycle interface {
	// LoadMethods performs component initialization (e.g. establishing the
	// MongoDB connection and preparing collections). It corresponds to the
	// Java loadMethods() method.
	LoadMethods(ctx context.Context) error

	// Finalized performs component teardown (e.g. closing the MongoDB
	// client). It corresponds to the Java finalized() method.
	Finalized(ctx context.Context) error
}
