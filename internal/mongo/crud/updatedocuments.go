// Package crud defines the MongoDB CRUD operation contracts and their
// implementations for this application.
//
// MIGRATION_NOTE: The Java source (UpdateDocuments) was a pure interface that
// extended the Commons "constant interface". It declared three no-argument,
// void update methods:
//
//   - updateOneDocument()
//   - updateManyDocument()
//   - updateDocumentWithCurrentDate()
//
// In idiomatic Go these become methods on an interface, but with several
// important changes over the source shape:
//
//  1. Every method now takes a context.Context as its first parameter. Update
//     operations perform network I/O against MongoDB and must be cancellable
//     and carry deadlines/tracing.
//
//  2. Every method now returns a result describing the update outcome plus an
//     error rather than being `void`. The Java methods relied on side effects
//     and unchecked exceptions; Go reports failures explicitly. We surface the
//     MongoDB driver's *mongo.UpdateResult so callers can inspect matched and
//     modified counts.
//
//  3. The interface embeds util.Commons (the already-migrated shared contract)
//     to preserve the Java "extends Commons" relationship. In Go this is
//     expressed through interface embedding rather than inheritance.
//
// This file intentionally declares ONLY the UpdateDocuments interface. The
// concrete implementation (the Java UpdateDocumentsImp equivalent, if any)
// belongs in a separate file to avoid redeclaring types already present in
// this package.
package crud

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"

	"github.com/mongo/crud/internal/mongo/util"
)

// UpdateDocuments is the contract for MongoDB document update operations.
//
// It mirrors the Java com.mongo.crud.UpdateDocuments interface, which extended
// the shared Commons contract. Here that relationship is modelled by embedding
// util.Commons.
//
// Each method performs network I/O against MongoDB, so all methods accept a
// context.Context for cancellation/deadlines and return an error alongside the
// driver's update result.
type UpdateDocuments interface {
	util.Commons

	// UpdateOneDocument updates a single document matching the operation's
	// filter. It returns the driver's UpdateResult (matched/modified counts)
	// or a non-nil error if the update fails.
	UpdateOneDocument(ctx context.Context) (*mongo.UpdateResult, error)

	// UpdateManyDocument updates all documents matching the operation's
	// filter. It returns the driver's UpdateResult (matched/modified counts)
	// or a non-nil error if the update fails.
	UpdateManyDocument(ctx context.Context) (*mongo.UpdateResult, error)

	// UpdateDocumentWithCurrentDate updates a document, setting a field to the
	// current date via MongoDB's $currentDate operator. It returns the
	// driver's UpdateResult (matched/modified counts) or a non-nil error if
	// the update fails.
	UpdateDocumentWithCurrentDate(ctx context.Context) (*mongo.UpdateResult, error)
}
