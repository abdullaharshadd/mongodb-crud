// Package impl provides concrete implementations of the CRUD contracts
// defined in the parent crud package for MongoDB.
//
// MIGRATION_NOTE: The Java source (com.mongo.crud.impl.DeleteDocumentsImpl)
// was the implementation half of the DeleteDocuments interface (already
// migrated to internal/mongo/crud/deletedocuments.go). The Go interface lives
// in the crud package while this concrete type lives here in the impl
// sub-package, mirroring the layout already established for
// InsertDocumentsImpl, QueryDocumentsImpl and UpdateDocumentsImpl.
//
// Java-to-Go adaptation decisions:
//
//	Java pattern                          Go equivalent / decision
//	-----------------------------         -----------------------------
//	class ...Impl implements Iface        struct DeleteDocumentsImpl
//	constructor acquires connection       NewDeleteDocumentsImpl returns error
//	void methods, log-and-swallow         methods return error; caller may
//	                                      aggregate with errors.Join
//	log4j Logger                          zerolog logger from internal/log4j.xml.go
//	legacy driver Filters.eq/lt           bson.M / bson.D filters (official driver)
//	deleteMany (both methods in source)   preserved: both use DeleteMany
package impl

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/rs/zerolog"

	"internal/mongo/util"
)

// DeleteDocumentsImpl is the concrete implementation of the
// crud.DeleteDocuments interface. It performs delete operations against a
// sample MongoDB collection using hardcoded filter queries, mirroring the
// behaviour of the original Java DeleteDocumentsImpl class.
type DeleteDocumentsImpl struct {
	conn *util.MongoConnection
	log  zerolog.Logger
}

// NewDeleteDocumentsImpl constructs a DeleteDocumentsImpl.
//
// MIGRATION_NOTE: The Java constructor created its own MongoConnectionUtils
// and eagerly opened a client. In Go we inject the already-constructed
// *util.MongoConnection so callers control connection lifecycle and the type
// stays testable. The logger is injected for the same reason.
func NewDeleteDocumentsImpl(conn *util.MongoConnection, log zerolog.Logger) (*DeleteDocumentsImpl, error) {
	if conn == nil {
		return nil, errors.New("mongo connection must not be nil")
	}
	return &DeleteDocumentsImpl{conn: conn, log: log}, nil
}

// LoadMethods invokes all deletion operations in sequence, mirroring the
// Java loadMethods orchestration.
//
// MIGRATION_NOTE: The Java version swallowed exceptions inside each method and
// returned void. Here each step returns an error; we log-and-continue and
// aggregate the failures with errors.Join so the caller sees everything that
// went wrong while still attempting every operation.
func (d *DeleteDocumentsImpl) LoadMethods(ctx context.Context) error {
	var errs []error

	if err := d.DeleteOneDocument(ctx); err != nil {
		d.log.Error().Err(err).Msg("delete one document failed")
		errs = append(errs, err)
	}
	if err := d.DeleteManyDocument(ctx); err != nil {
		d.log.Error().Err(err).Msg("delete many documents failed")
		errs = append(errs, err)
	}

	d.Finalized()

	return errors.Join(errs...)
}

// DeleteOneDocument deletes documents matching {"name": "sundar"}.
//
// MIGRATION_NOTE: Despite the Java method name ("deleteOneDocument") and its
// javadoc claiming "the first matched", the Java code actually called
// collection.deleteMany(query). Business logic is preserved exactly: this
// performs a DeleteMany, not a DeleteOne.
func (d *DeleteDocumentsImpl) DeleteOneDocument(ctx context.Context) error {
	collection := d.sampleCollection()
	filter := bson.M{"name": "sundar"}

	result, err := collection.DeleteMany(ctx, filter)
	if err != nil {
		return fmt.Errorf("delete single document: %w", err)
	}

	d.log.Info().
		Int64("deletedCount", result.DeletedCount).
		Msg("Single Document deleted successfully")
	return nil
}

// DeleteManyDocument deletes all documents matching {"age": {"$lt": 20}}.
func (d *DeleteDocumentsImpl) DeleteManyDocument(ctx context.Context) error {
	collection := d.sampleCollection()
	filter := bson.M{"age": bson.M{"$lt": 20}}

	result, err := collection.DeleteMany(ctx, filter)
	if err != nil {
		return fmt.Errorf("delete many documents: %w", err)
	}

	d.log.Info().
		Int64("deletedCount", result.DeletedCount).
		Msg("Document(s) deleted successfully")
	return nil
}

// Finalized closes the underlying MongoDB client.
//
// MIGRATION_NOTE: As noted in the Java source, closing is not strictly
// required (the driver/cluster manages connections), but we preserve the
// explicit close for parity. Any close error is logged rather than returned
// to keep the method's fire-and-forget semantics.
func (d *DeleteDocumentsImpl) Finalized() {
	if err := d.conn.Close(context.Background()); err != nil {
		d.log.Error().Err(err).Msg("failed to close mongo client")
	}
}

// sampleCollection returns the sample collection handle used by all delete
// operations, centralizing the database/collection selection that the Java
// code repeated in each method.
func (d *DeleteDocumentsImpl) sampleCollection() *mongo.Collection {
	return d.conn.Client().
		Database(d.conn.DataBase()).
		Collection(d.conn.SampleCollection())
}
