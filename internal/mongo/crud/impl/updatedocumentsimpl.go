// Package impl provides concrete implementations of the MongoDB CRUD
// operation contracts defined in the parent crud package.
//
// MIGRATION_NOTE: The Java source (UpdateDocumentsImpl) implemented the
// UpdateDocuments interface and demonstrated document-update techniques
// against a configured MongoDB collection: updateOne, updateMany, and an
// update that stamps a $currentDate field. The migration makes the following
// idiomatic changes over the source shape:
//
//  1. Every method now takes a context.Context as its first parameter, since
//     update operations perform cancellable network I/O against MongoDB.
//
//  2. Every method now returns an error instead of swallowing exceptions and
//     merely logging them (the Java code caught MongoException and logged).
//     Callers decide how to react.
//
//  3. Filters and update documents use the driver's bson.M / bson.D idioms
//     rather than the Java Filters/Updates static-builder DSL. The
//     $currentDate case is expressed directly as a bson operator.
//
//  4. Connection management is delegated to MongoConnectionUtils, obtained
//     via the constructor, mirroring the pattern established by the already
//     migrated InsertDocumentsImpl / QueryDocumentsImpl.
package impl

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/example/project/internal/logging"
	"github.com/example/project/internal/mongo/util"
)

// UpdateDocumentsImpl is the concrete implementation of the
// crud.UpdateDocuments interface. It performs update operations against the
// configured sample collection using an underlying MongoConnectionUtils.
type UpdateDocumentsImpl struct {
	mongo *util.MongoConnectionUtils
	log   *logging.Logger
}

// NewUpdateDocumentsImpl constructs an UpdateDocumentsImpl backed by the
// supplied MongoConnectionUtils and logger.
//
// MIGRATION_NOTE: The Java no-arg constructor created its own
// MongoConnectionUtils and eagerly opened a client. Here the collaborators are
// injected (constructor injection) so the type is testable and does not create
// hidden global state. Callers are expected to have connected the utils
// (via Connect) before invoking the update methods.
func NewUpdateDocumentsImpl(m *util.MongoConnectionUtils, log *logging.Logger) *UpdateDocumentsImpl {
	return &UpdateDocumentsImpl{mongo: m, log: log}
}

// collection resolves the configured sample collection from the underlying
// connection utilities, returning an error if the collection is unavailable.
func (u *UpdateDocumentsImpl) collection() (*mongo.Collection, error) {
	coll := u.mongo.SampleCollection()
	if coll == nil {
		return nil, fmt.Errorf("update documents: sample collection is not configured")
	}
	return coll, nil
}

// LoadMethods invokes each update operation in sequence, mirroring the Java
// loadMethods orchestration. It stops and returns on the first error.
func (u *UpdateDocumentsImpl) LoadMethods(ctx context.Context) error {
	if err := u.UpdateOneDocument(ctx); err != nil {
		return fmt.Errorf("update one document: %w", err)
	}
	if err := u.UpdateManyDocument(ctx); err != nil {
		return fmt.Errorf("update many documents: %w", err)
	}
	if err := u.UpdateDocumentWithCurrentDate(ctx); err != nil {
		return fmt.Errorf("update document with current date: %w", err)
	}
	return u.Finalized(ctx)
}

// UpdateOneDocument updates the first document whose "name" equals "Sundar",
// setting the "age" and "gender" fields. It logs the acknowledgement status
// and number of modified records.
func (u *UpdateDocumentsImpl) UpdateOneDocument(ctx context.Context) error {
	coll, err := u.collection()
	if err != nil {
		return err
	}

	filter := bson.M{"name": "Sundar"}
	update := bson.M{"$set": bson.M{"age": 23, "gender": "Male"}}

	result, err := coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("update single document: %w", err)
	}

	u.log.Info(fmt.Sprintf("UpdateOne acknowledged (matched %d)", result.MatchedCount))
	u.log.Info(fmt.Sprintf("No of Record Modified : %d", result.ModifiedCount))
	return nil
}

// UpdateManyDocument updates every document whose "name" equals "Sundar",
// setting the "age" and "gender" fields. It logs the acknowledgement status
// and number of modified records.
func (u *UpdateDocumentsImpl) UpdateManyDocument(ctx context.Context) error {
	coll, err := u.collection()
	if err != nil {
		return err
	}

	filter := bson.M{"name": "Sundar"}
	update := bson.M{"$set": bson.M{"age": 23, "gender": "Male"}}

	result, err := coll.UpdateMany(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("update many documents: %w", err)
	}

	u.log.Info(fmt.Sprintf("UpdateMany acknowledged (matched %d)", result.MatchedCount))
	u.log.Info(fmt.Sprintf("No of Record Modified : %d", result.ModifiedCount))
	return nil
}

// UpdateDocumentWithCurrentDate updates the first document whose "name" equals
// "Sundar", setting the "age" and "gender" fields and stamping the
// "lastModified" field with the server's current date.
//
// MIGRATION_NOTE: The Java Updates.currentDate(...) helper maps to the Mongo
// $currentDate update operator, applied directly here alongside $set.
func (u *UpdateDocumentsImpl) UpdateDocumentWithCurrentDate(ctx context.Context) error {
	coll, err := u.collection()
	if err != nil {
		return err
	}

	filter := bson.M{"name": "Sundar"}
	update := bson.M{
		"$set":         bson.M{"age": 23, "gender": "Male"},
		"$currentDate": bson.M{"lastModified": true},
	}

	result, err := coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("update document with current date: %w", err)
	}

	u.log.Info(fmt.Sprintf("Update with date acknowledged (matched %d)", result.MatchedCount))
	u.log.Info(fmt.Sprintf("No of Record Modified : %d", result.ModifiedCount))
	return nil
}

// Finalized closes the underlying MongoDB client.
//
// MIGRATION_NOTE: The Java finalized() method delegated to
// closeMongoClient(client); here it delegates to the migrated Close method on
// MongoConnectionUtils, which returns an error for the caller to handle.
func (u *UpdateDocumentsImpl) Finalized(ctx context.Context) error {
	if err := u.mongo.Close(ctx); err != nil {
		return fmt.Errorf("close mongo client: %w", err)
	}
	return nil
}
