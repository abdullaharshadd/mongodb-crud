// Package crud defines the MongoDB CRUD operation contracts and their
// implementations for this application.
//
// MIGRATION_NOTE: The Java source (InsertDocuments) was a pure interface that
// extended the Commons "constant interface". It declared four no-argument,
// void insert methods:
//
//   - insertUsingDocument()
//   - insertUsingMap()
//   - insertSingleDocument()
//   - insertMultipleDocuments()
//
// In idiomatic Go these become methods on an interface, but with two important
// changes over the source shape:
//
//   1. Every method now returns an error. The Java methods were `void` and
//      relied on side effects / unchecked exceptions; Go reports failures
//      explicitly. Insert operations perform network I/O against MongoDB and
//      therefore must be able to fail.
//   2. Every method takes a context.Context as its first parameter so callers
//      can propagate cancellation and deadlines through the driver call — this
//      is mandatory for any MongoDB driver operation (InsertOne/InsertMany).
//
// The Java interface "extends Commons". The already-migrated
// internal/mongo/util package exposes Commons as a small lifecycle interface
// (LoadMethods/Finalized). Rather than embed a foreign-package lifecycle
// contract into this data-access interface (which would conflate concerns),
// the composition is expressed by embedding util.Commons, faithfully
// preserving the "extends Commons" relationship while keeping the insert
// behaviour in this package.
//
// There is NO SQL here: the target datastore is MongoDB. Implementations are
// expected to use the official Mongo Go driver idioms — bson.M / bson.D
// documents with collection.InsertOne / collection.InsertMany — and to let
// MongoDB generate the document _id.
package crud

import (
	"context"

	"migrated-app/internal/mongo/util"
)

// InsertDocuments is the contract for MongoDB document insertion operations.
//
// It mirrors the Java com.mongo.crud.InsertDocuments interface, which extended
// Commons. Here the lifecycle contract is preserved by embedding util.Commons,
// while each insert method returns an error and accepts a context.Context for
// cancellation and deadline propagation.
type InsertDocuments interface {
	// Commons carries the shared lifecycle contract (LoadMethods/Finalized)
	// inherited from the original Java `extends Commons`.
	util.Commons

	// InsertUsingDocument inserts a document constructed as a bson document
	// (e.g. bson.D/bson.M) via the Mongo driver's InsertOne. It returns an
	// error if the insert fails.
	InsertUsingDocument(ctx context.Context) error

	// InsertUsingMap inserts a document built from a Go map (translated to a
	// bson.M) via the Mongo driver's InsertOne. It returns an error if the
	// insert fails.
	InsertUsingMap(ctx context.Context) error

	// InsertSingleDocument inserts exactly one document via the Mongo driver's
	// InsertOne. It returns an error if the insert fails.
	InsertSingleDocument(ctx context.Context) error

	// InsertMultipleDocuments inserts several documents in one call via the
	// Mongo driver's InsertMany. It returns an error if the insert fails.
	InsertMultipleDocuments(ctx context.Context) error
}
