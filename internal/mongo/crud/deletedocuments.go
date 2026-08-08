// Package crud defines the contracts and implementations for MongoDB CRUD
// operations used by the application.
//
// MIGRATION_NOTE: The Java source (com.mongo.crud.DeleteDocuments) was an
// interface that extended com.mongo.utils.Commons and declared two delete
// operations (deleteOneDocument, deleteManyDocument). In Go we model this as an
// interface that embeds the already-migrated util.Lifecycle contract (the Go
// equivalent of the Commons lifecycle methods) and adds the two delete
// behaviors.
//
// Several Java-isms were deliberately adapted to idiomatic Go:
//
//	Java pattern                          Go equivalent / decision
//	-----------------------------         -----------------------------
//	extends Commons                       embeds util.Lifecycle
//	void deleteOneDocument()              DeleteOne(ctx) (int64, error)
//	void deleteManyDocument()             DeleteMany(ctx) (int64, error)
//
// The original methods returned void and swallowed errors internally. Idiomatic
// Go surfaces I/O failures to the caller, so every method returns an error and
// takes a context.Context as its first parameter for cancellation and deadline
// propagation. Each method also returns the number of documents deleted, which
// is the natural result of a MongoDB DeleteOne/DeleteMany call
// (mongo.DeleteResult.DeletedCount) and is far more useful than discarding it.
package crud

import (
	"context"

	"github.com/example/app/internal/mongo/util"
)

// DeleteDocuments is the contract for MongoDB document deletion operations.
//
// It embeds util.Lifecycle (the Go equivalent of the Java Commons base
// interface) so that any implementation also participates in the standard
// connection lifecycle. Implementations are expected to talk to MongoDB using
// the official driver's idioms (bson.M filters, collection.DeleteOne /
// collection.DeleteMany) rather than any SQL dialect.
type DeleteDocuments interface {
	util.Lifecycle

	// DeleteOne removes a single document matching the implementation's
	// configured filter. It returns the number of documents deleted (0 or 1)
	// and any error encountered while communicating with MongoDB.
	DeleteOne(ctx context.Context) (int64, error)

	// DeleteMany removes all documents matching the implementation's
	// configured filter. It returns the number of documents deleted and any
	// error encountered while communicating with MongoDB.
	DeleteMany(ctx context.Context) (int64, error)
}
