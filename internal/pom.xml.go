// Package internal documents the build and dependency configuration that
// was previously expressed by the Maven pom.xml of the source project.
//
// MIGRATION_NOTE: pom.xml is a Maven *build descriptor*, not application
// source code. It has no runtime business logic to translate — its
// responsibilities (declaring dependencies, compiler target, packaging an
// executable JAR, and copying resources) are fulfilled in the Go ecosystem
// by entirely different mechanisms:
//
//   - Dependencies      -> go.mod / go.sum (the Go module system)
//   - Compiler target   -> the `go` directive in go.mod + build tags
//   - Executable JAR    -> `go build ./cmd/server` produces a static binary
//   - lib/ classpath    -> not applicable; Go links dependencies statically
//   - Resource copying   -> Go embeds files via the `embed` package, or ships
//                          them alongside the binary
//
// There is therefore no idiomatic Go *code* that corresponds to pom.xml.
// The most useful, honest migration is to (a) record the equivalent module
// metadata here for human reference, and (b) map the two real dependencies
// to their Go counterparts so downstream files import the correct packages.
//
// The single dependency that matters at runtime is the MongoDB driver. The
// source pinned `org.mongodb:mongo-java-driver:3.4.2`, whose 3.4-era API is
// built around `MongoClientURI` / `MongoClient(MongoClientURI)`. The direct
// Go equivalent is go.mongodb.org/mongo-driver, whose
// `options.Client().ApplyURI(uri)` + `mongo.Connect(ctx, opts)` shape maps
// cleanly onto the connection-URI approach already produced by
// internal/conf.MongoConfig.URI().
package internal

// BuildMetadata captures the project coordinates that were declared in the
// original Maven pom.xml. It exists purely as documentation/reference and
// carries no runtime behaviour.
type BuildMetadata struct {
	// GroupID is the Maven groupId (com.mongo).
	GroupID string
	// ArtifactID is the Maven artifactId (MongoDB).
	ArtifactID string
	// Version is the project version (1.0.0).
	Version string
	// JavaSourceTarget records the original compiler source/target level (1.8).
	// In Go the equivalent is the `go` directive in go.mod.
	JavaSourceTarget string
}

// ProjectBuildMetadata returns the build coordinates migrated from pom.xml.
//
// This is informational only: it lets tooling or humans confirm the identity
// of the module the Go port replaces. It performs no I/O and cannot fail.
func ProjectBuildMetadata() BuildMetadata {
	return BuildMetadata{
		GroupID:          "com.mongo",
		ArtifactID:       "MongoDB",
		Version:          "1.0.0",
		JavaSourceTarget: "1.8",
	}
}

// GoModuleDependencies documents the mapping from the Maven dependencies
// declared in pom.xml to their idiomatic Go module equivalents. These are the
// import paths that must appear in go.mod for a functionally equivalent build.
//
// MIGRATION_NOTE:
//   - log4j:log4j:1.2.17            -> github.com/rs/zerolog (see cmd/server/main.go)
//   - org.mongodb:mongo-java-driver -> go.mongodb.org/mongo-driver/mongo
//
// The MongoDB driver version was 3.4.2, which uses the MongoClientURI API.
// The Go driver's options.Client().ApplyURI(...) is the correct equivalent
// and consumes the URI produced by internal/conf.MongoConfig.URI().
func GoModuleDependencies() map[string]string {
	return map[string]string{
		"log4j:log4j:1.2.17":                    "github.com/rs/zerolog",
		"org.mongodb:mongo-java-driver:3.4.2":   "go.mongodb.org/mongo-driver/mongo",
	}
}
