// Package internal documents the build and dependency configuration for
// this application. The original project defined its build via a Maven
// pom.xml. In Go there is no direct equivalent of a POM: dependency
// management is handled by go.mod/go.sum, builds by the `go` toolchain, and
// packaging by `go build`. This file therefore contains no runtime logic —
// it exists only to capture, in one place, the migration decisions implied
// by the original pom.xml so that a human reviewer can reconcile them with
// the Go module setup.
//
// MIGRATION_NOTE: pom.xml has no behavioral counterpart in Go. Everything it
// declared maps onto tooling/config that lives outside a .go source file:
//
//   Maven concept                       Go equivalent
//   -----------------------------       -------------------------------------
//   <groupId>/<artifactId>/<version>    module path + tag in go.mod
//   <packaging>jar</packaging>          `go build` produces a native binary
//   maven-jar-plugin (executable JAR)   `go build ./cmd/...` (no manifest)
//   classpathPrefix lib/                static linking; no external lib dir
//   maven-dependency-plugin copy-deps   vendoring: `go mod vendor` (optional)
//   maven-resources-plugin copy conf    embed via `//go:embed` or read at run
//   <mainClass>com.mongo.main.MongoTest a func main() in package main
//   maven.compiler.source/target 1.8    the `go` directive in go.mod
//
// None of the above belongs in a compiled .go file, so this file only exposes
// the version constants that DO carry semantic weight for the migration —
// specifically the MongoDB driver version, which affects BSON numeric
// encoding (see the CRITICAL note below).
package internal

// Dependency versions carried over from the original pom.xml <dependencies>
// block. These are informational: the actual Go dependencies are pinned in
// go.mod. They are recorded here because the driver version has behavioral
// consequences that must survive the migration.
const (
	// LegacyLog4jVersion is the log4j version used by the original Java
	// application (log4j:log4j:1.2.17). In Go this is replaced by a native
	// logger; no direct dependency is required.
	//
	// MIGRATION_NOTE: log4j 1.x has no Go equivalent. Prefer the standard
	// library's log/slog (Go 1.21+) or zerolog/zap. A log4j.properties/
	// log4j.xml config file, if present in the source tree, must be
	// re-expressed as Go logger configuration by hand.
	LegacyLog4jVersion = "1.2.17"

	// LegacyMongoJavaDriverVersion is the mongo-java-driver version the
	// original application was built against (org.mongodb:mongo-java-driver:
	// 3.4.2). This is replaced by go.mongodb.org/mongo-driver in Go.
	//
	// CRITICAL BSON ENCODING TRAP:
	// The Java mongo-java-driver 3.4.2 encoded a plain Java `int` as a BSON
	// Int32. The Go driver encodes a Go `int` as a BSON Int64 by default.
	// Any document field written by the old app as an Int32 (notably the
	// "age" field) will therefore be QUERIED/WRITTEN inconsistently if a Go
	// `int` is used naively:
	//
	//   - Reading old docs: a filter like bson.M{"age": 30} in Go marshals 30
	//     as Int64; MongoDB's numeric comparison still matches an Int32 30, so
	//     equality queries generally work.
	//   - Writing new docs: Go writes age as Int64, so a collection will end
	//     up with MIXED Int32/Int64 "age" values across old and new records.
	//
	// To preserve the original on-disk representation exactly, model age as
	// int32 (which the Go driver encodes as BSON Int32) in the struct that
	// maps to the document, e.g.:
	//
	//   type Sample struct {
	//       ID   primitive.ObjectID `bson:"_id,omitempty"`
	//       Name string             `bson:"name"`
	//       Age  int32              `bson:"age"`
	//   }
	//
	// Let MongoDB generate _id (omitempty on an ObjectID) rather than
	// assigning one client-side.
	LegacyMongoJavaDriverVersion = "3.4.2"
)

// MIGRATION_NOTE: The pom declared a standalone main class,
// com.mongo.main.MongoTest, packaged as an executable JAR with dependencies
// copied to a lib/ directory. In Go the entry point should live in
// cmd/mongodb/main.go (package main) and should:
//
//   1. Load configuration via conf.LoadMongoConfig (already migrated in
//      internal/conf/mongo.properties.go).
//   2. Build the connection string via the MongoConfig.URI method.
//   3. Connect with go.mongodb.org/mongo-driver/mongo using a
//      context.Context, honoring graceful shutdown on SIGINT/SIGTERM.
//
// That wiring is intentionally NOT placed in this file, because pom.xml is a
// build descriptor, not application logic. See internal/conf for the config
// surface (MongoConfig, URI, LoadMongoConfig).
