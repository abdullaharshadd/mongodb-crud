// Package crud defines the contracts and implementations for MongoDB CRUD
// operations used by the application.
//
// MIGRATION_NOTE: The Java source (com.mongo.crud.UpdateDocuments) was an
// interface that extended com.mongo.utils.Commons and declared three update
// operations (updateOneDocument, updateManyDocument,
// updateDocumentWithCurrentDate). In Go we model this as an interface that
// embeds the already-migrated util.Lifecycle contract (the Go equivalent of
// the Commons lifecycle methods) and adds the three update behaviors.
//
// Several Java-isms were deliberately adapted to idiomatic Go:
//
//   Java pattern                          Go equivalent / decision
//   -----------------------------         -----------------------------
//   extends Commons                       embeds util.Lifecycle
//   void updateOneDocument()              UpdateOne(ctx) error
//   void updateManyDocument()             UpdateMany(ctx) error
//   void updateDocumentWithCurrentDate()  UpdateWithCurrentDate(ctx) error
//
// Each method takes a context.Context as its first parameter (every MongoDB
// operation performs I/O and must be cancellable), and returns an error so the
// caller can handle failures explicitly instead of relying on unchecked
// exceptions. The concrete implementation of this interface lives elsewhere;
// this file declares only the contract, mirroring the interface-only nature of
// the Java source.
package crud

import (
	"context"

	"github.com/mongo/crud/internal/mongo/util"
)

// UpdateDocuments is the contract for MongoDB document update operations.
//
// It embeds util.Lifecycle so that any implementation also satisfies the shared
// lifecycle contract (the Go equivalent of the Java Commons interface). The
// added methods cover updating a single document, updating many documents, and
// updating a document with the current date.
type UpdateDocuments interface {
	util.Lifecycle

	// UpdateOne updates a single matching document in the collection.
	// It returns an error if the update cannot be performed.
	UpdateOne(ctx context.Context) error

	// UpdateMany updates all matching documents in the collection.
	// It returns an error if the update cannot be performed.
	UpdateMany(ctx context.Context) error

	// UpdateWithCurrentDate updates a document, setting a field to the current
	// date (the Go equivalent of Mongo's $currentDate operator).
	// It returns an error if the update cannot be performed.
	UpdateWithCurrentDate(ctx context.Context) error
}
