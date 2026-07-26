// Package impl provides concrete implementations of the MongoDB CRUD
// operation contracts defined in the parent crud package.
//
// MIGRATION_NOTE: The Java source (DeleteDocumentsImpl) implemented the
// DeleteDocuments interface and demonstrated document-deletion techniques
// against a configured MongoDB collection: a "delete one" (which in the Java
// source actually invoked deleteMany with an equality filter) and a
// "delete many" using a less-than filter. The migration makes the following
// idiomatic changes over the source shape:
//
//  1. Every method now takes a context.Context as its first parameter, since
//     delete operations perform cancellable network I/O against MongoDB.
//
//  2. Every method now returns an error instead of swallowing exceptions and
//     merely logging them (the Java code caught MongoException and logged).
//     Callers decide how to react.
//
//  3. Filters use bson.M rather than the driver's Filters.eq / Filters.lt
//     helpers.
//
//  4. The Java "deleteOneDocument" method used deleteMany under the hood; that
//     behavior is preserved exactly (DeleteMany with an equality filter),
//     though the method is named DeleteOneDocument to match the source.
package impl

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"migrated-app/internal/mongo/util"
)

// DeleteDocumentsImpl is the concrete implementation of the DeleteDocuments
// contract. It performs document-deletion operations against a configured
// MongoDB collection.
//
// MIGRATION_NOTE: In the Java source the constructor eagerly created a
// MongoConnectionUtils and opened a MongoClient. Here the connection utility
// is injected via NewDeleteDocumentsImpl so the type is testable and does not
// perform I/O during construction.
type DeleteDocumentsImpl struct {
	mongo *util.MongoConnectionUtils
}

// NewDeleteDocumentsImpl constructs a DeleteDocumentsImpl using the supplied
// MongoConnectionUtils. The utility is expected to have an established
// connection (via its Connect method) before the delete methods are called.
func NewDeleteDocumentsImpl(mongoUtils *util.MongoConnectionUtils) *DeleteDocumentsImpl {
	return &DeleteDocumentsImpl{mongo: mongoUtils}
}

// LoadMethods invokes all deletion methods in sequence. It returns the first
// error encountered, stopping the chain early.
//
// MIGRATION_NOTE: The Java loadMethods called finalized() unconditionally at
// the end. Here Finalized is invoked via defer-like sequencing so the client
// is always closed even if an earlier step fails.
func (d *DeleteDocumentsImpl) LoadMethods(ctx context.Context) (err error) {
	defer func() {
		if closeErr := d.Finalized(ctx); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	if err = d.DeleteOneDocument(ctx); err != nil {
		return fmt.Errorf("delete one document: %w", err)
	}
	if err = d.DeleteManyDocument(ctx); err != nil {
		return fmt.Errorf("delete many document: %w", err)
	}
	return nil
}

// collection resolves the configured sample collection from the connection
// utility.
func (d *DeleteDocumentsImpl) collection() (*mongo.Collection, error) {
	db := d.mongo.Database()
	if db == nil {
		return nil, fmt.Errorf("mongo database is not initialized; call Connect first")
	}
	return db.Collection(d.mongo.SampleCollection()), nil
}

// DeleteOneDocument deletes documents matching an equality filter on the
// "name" field.
//
// MIGRATION_NOTE: The Java source named this deleteOneDocument but invoked
// deleteMany(eq("name", "sundar")). That exact behavior — DeleteMany with an
// equality filter — is preserved here.
func (d *DeleteDocumentsImpl) DeleteOneDocument(ctx context.Context) error {
	collection, err := d.collection()
	if err != nil {
		return err
	}

	filter := bson.M{"name": "sundar"}
	result, err := collection.DeleteMany(ctx, filter)
	if err != nil {
		return fmt.Errorf("delete single document: %w", err)
	}
	return nil
	_ = result // DeletedCount available on result if a caller needs it.
}

// DeleteManyDocument deletes all documents matching a less-than filter on the
// "age" field (age < 20).
func (d *DeleteDocumentsImpl) DeleteManyDocument(ctx context.Context) error {
	collection, err := d.collection()
	if err != nil {
		return err
	}

	filter := bson.M{"age": bson.M{"$lt": 20}}
	_, err = collection.DeleteMany(ctx, filter)
	if err != nil {
		return fmt.Errorf("delete many documents: %w", err)
	}
	return nil
}

// Finalized closes the MongoDB client. As in the Java source, this is a safety
// measure — the driver manages connection pooling, but explicit closure is
// performed for cleanliness.
func (d *DeleteDocumentsImpl) Finalized(ctx context.Context) error {
	if err := d.mongo.Close(ctx); err != nil {
		return fmt.Errorf("close mongo client: %w", err)
	}
	return nil
}
