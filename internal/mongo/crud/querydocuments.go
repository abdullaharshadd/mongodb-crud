// Package crud defines the MongoDB CRUD operation contracts and their
// implementations for this application.
//
// MIGRATION_NOTE: The Java source (QueryDocuments) was a pure interface that
// extended the Commons "constant interface". It declared two no-argument /
// single-argument, void query methods:
//
//   - getAllDocuments()
//   - getSpecificDocument(String operator)
//
// In idiomatic Go these become methods on an interface, but with three
// important changes over the source shape:
//
//  1. Every method now takes a context.Context as its first parameter. Query
//     operations perform network I/O against MongoDB and must be cancellable
//     and carry deadlines/tracing.
//
//  2. Every method now returns a result plus an error rather than being
//     `void`. The Java methods relied on side effects (printing results) and
//     unchecked exceptions; Go reports failures explicitly and returns the
//     decoded documents to the caller. Documents are modelled as bson.M so
//     callers receive the raw decoded MongoDB documents without a
//     source-shaped SQL translation.
//
//  3. The interface embeds the already-migrated util.Commons lifecycle
//     contract (the Go equivalent of Java's `extends Commons`) instead of
//     inheriting a set of interface constants.
package crud

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"

	"migrated-app/internal/mongo/util"
)

// QueryDocuments is the contract for read operations against a MongoDB
// collection. It embeds util.Commons so that implementations also satisfy the
// shared lifecycle contract, mirroring the Java `extends Commons` relationship.
type QueryDocuments interface {
	util.Commons

	// GetAllDocuments retrieves every document in the target collection.
	//
	// MIGRATION_NOTE: The Java getAllDocuments() was void and printed results as
	// a side effect. Here the decoded documents are returned to the caller as a
	// slice of bson.M, and any I/O failure is reported via the error return.
	GetAllDocuments(ctx context.Context) ([]bson.M, error)

	// GetSpecificDocument retrieves the documents matching the supplied query
	// operator. The operator identifies which filter/query the implementation
	// should apply against the collection.
	//
	// MIGRATION_NOTE: The Java getSpecificDocument(String operator) was void.
	// Here it returns the matching decoded documents plus an error so callers
	// can handle "not found" and I/O failures explicitly.
	GetSpecificDocument(ctx context.Context, operator string) ([]bson.M, error)
}
