// Package crud defines the MongoDB CRUD operation contracts and their
// implementations for this application.
//
// MIGRATION_NOTE: The Java source (DeleteDocuments) was a pure interface that
// extended the Commons "constant interface". It declared two no-argument, void
// delete methods:
//
//   - deleteOneDocument()
//   - deleteManyDocument()
//
// In idiomatic Go these become methods on an interface, but with several
// important changes over the source shape:
//
//  1. Every method now takes a context.Context as its first parameter. Delete
//     operations perform network I/O against MongoDB and must be cancellable
//     and carry deadlines/tracing.
//
//  2. Every method now takes an explicit bson.M filter argument. The Java
//     methods took no arguments and presumably relied on a hard-coded filter
//     inside the (unmigrated) implementation. Idiomatic Go passes the filter
//     in explicitly so callers control which documents are targeted; the
//     MongoDB driver generates/uses the _id itself.
//
//  3. Every method now returns a *mongo.DeleteResult plus an error rather than
//     being `void`. The Java methods relied on side effects / unchecked
//     exceptions; Go reports failures explicitly and returns the deletion
//     outcome (DeletedCount) so callers can react.
//
// The Commons interface is embedded to preserve the Java "extends Commons"
// relationship, mirroring the InsertDocuments/QueryDocuments/UpdateDocuments
// migrations in this package.
package crud

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"migrated-app/internal/mongo/util"
)

// DeleteDocuments is the contract for MongoDB document deletion operations.
//
// It embeds util.Commons to preserve the Java "extends Commons" relationship,
// keeping the shared collection/connection accessors available to any
// implementation of this contract.
type DeleteDocuments interface {
	util.Commons

	// DeleteOneDocument deletes the single document matching the given filter.
	// It returns the deletion result (including DeletedCount) or an error if
	// the operation fails. Use an empty bson.M{} filter with care, as it will
	// match — and delete — the first document in the collection.
	DeleteOneDocument(ctx context.Context, filter bson.M) (*mongo.DeleteResult, error)

	// DeleteManyDocument deletes all documents matching the given filter. It
	// returns the deletion result (including DeletedCount) or an error if the
	// operation fails. Passing an empty bson.M{} filter will delete every
	// document in the collection.
	DeleteManyDocument(ctx context.Context, filter bson.M) (*mongo.DeleteResult, error)
}
