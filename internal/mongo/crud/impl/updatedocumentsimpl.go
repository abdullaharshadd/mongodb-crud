// Package impl provides concrete implementations of the CRUD contracts
// defined in the parent crud package for MongoDB.
//
// MIGRATION_NOTE: The Java source (com.mongo.crud.impl.UpdateDocumentsImpl)
// was the implementation half of the UpdateDocuments interface (already
// migrated to internal/mongo/crud/updatedocuments.go). The Go interface lives
// in the crud package while this concrete type lives here in the impl
// sub-package, mirroring the layout already established for
// InsertDocumentsImpl and QueryDocumentsImpl.
//
// Java-to-Go adaptation decisions:
//
//	Java pattern                          Go equivalent / decision
//	-----------------------------         -----------------------------
//	class ...Impl implements Iface        struct UpdateDocumentsImpl
//	constructor acquires connection       NewUpdateDocumentsImpl returns error
//	static log4j Logger                   injected *zerolog.Logger
//	void methods swallow MongoException   methods return error
//	Filters.eq / Updates.combine/set      bson.M / bson.D driver builders
//	Updates.currentDate("lastModified")   $currentDate server-side operator
//
// MIGRATION_NOTE: The original Java updateXxx methods returned void and logged
// exceptions internally. Idiomatic Go surfaces errors to the caller, so every
// operation here returns an error. Logging is retained for parity with the
// source, but the error is also propagated. LoadMethods stops on the first
// error, unlike the Java version which ran all steps regardless (each Java
// method caught its own exception). This is a deliberate improvement; if the
// original best-effort behavior is required, collect errors instead.
//
// MIGRATION_NOTE: currentDate("lastModified") MUST remain a server-side
// $currentDate operator. It is expressed here as bson.M{"$currentDate": ...}
// rather than a client-side time.Now(), so Mongo stamps the server time.
package impl

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"migrated-app/internal/mongo/crud"
	"migrated-app/internal/mongo/util"
)

// UpdateDocumentsImpl is the MongoDB-backed implementation of the
// crud.UpdateDocuments contract. It demonstrates updateOne, updateMany and an
// update that stamps a server-side lastModified date via $currentDate.
type UpdateDocumentsImpl struct {
	conn   *util.MongoConnection
	client *mongo.Client
	logger *zerolog.Logger
}

// Compile-time assertion that UpdateDocumentsImpl satisfies the interface.
var _ crud.UpdateDocuments = (*UpdateDocumentsImpl)(nil)

// NewUpdateDocumentsImpl constructs an UpdateDocumentsImpl, acquiring a Mongo
// connection through the shared util.MongoConnection helper.
//
// MIGRATION_NOTE: The Java constructor did resource acquisition implicitly and
// could not report failure; the Go constructor returns an error so a failed
// connection is visible to the caller.
func NewUpdateDocumentsImpl(ctx context.Context, conn *util.MongoConnection, logger *zerolog.Logger) (*UpdateDocumentsImpl, error) {
	if conn == nil {
		return nil, fmt.Errorf("mongo connection must not be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger must not be nil")
	}

	client, err := conn.Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire mongo client: %w", err)
	}

	return &UpdateDocumentsImpl{
		conn:   conn,
		client: client,
		logger: logger,
	}, nil
}

// collection resolves the sample collection from the configured database.
func (u *UpdateDocumentsImpl) collection() *mongo.Collection {
	db := u.client.Database(u.conn.Database())
	return db.Collection(u.conn.SampleCollection())
}

// LoadMethods invokes every update operation in sequence, stopping at the
// first error, and then finalizes the connection.
//
// MIGRATION_NOTE: The Java loadMethods ignored per-step failures because each
// method swallowed its own exception. Here we propagate errors; Finalized is
// always attempted (deferred) to mirror the Java lifecycle guarantee.
func (u *UpdateDocumentsImpl) LoadMethods(ctx context.Context) (err error) {
	defer func() {
		if cerr := u.Finalized(ctx); cerr != nil && err == nil {
			err = cerr
		}
	}()

	if err = u.UpdateOneDocument(ctx); err != nil {
		return err
	}
	if err = u.UpdateManyDocument(ctx); err != nil {
		return err
	}
	if err = u.UpdateDocumentWithCurrentDate(ctx); err != nil {
		return err
	}
	return nil
}

// UpdateOneDocument updates only the first document matching the filter,
// setting age and gender.
func (u *UpdateDocumentsImpl) UpdateOneDocument(ctx context.Context) error {
	filter := bson.M{"name": "Sundar"}
	update := bson.M{"$set": bson.M{"age": 23, "gender": "Male"}}

	result, err := u.collection().UpdateOne(ctx, filter, update)
	if err != nil {
		u.logger.Error().Err(err).Msg("Exception occurred while update single Document")
		return fmt.Errorf("update one document: %w", err)
	}

	u.logger.Info().
		Int64("matchedCount", result.MatchedCount).
		Int64("modifiedCount", result.ModifiedCount).
		Msg("UpdateOne completed")
	return nil
}

// UpdateManyDocument updates every document matching the filter, setting age
// and gender.
func (u *UpdateDocumentsImpl) UpdateManyDocument(ctx context.Context) error {
	filter := bson.M{"name": "Sundar"}
	update := bson.M{"$set": bson.M{"age": 23, "gender": "Male"}}

	result, err := u.collection().UpdateMany(ctx, filter, update)
	if err != nil {
		u.logger.Error().Err(err).Msg("Exception occurred while update Many Document")
		return fmt.Errorf("update many documents: %w", err)
	}

	u.logger.Info().
		Int64("matchedCount", result.MatchedCount).
		Int64("modifiedCount", result.ModifiedCount).
		Msg("UpdateMany completed")
	return nil
}

// UpdateDocumentWithCurrentDate updates the first matching document, setting
// age and gender and stamping a server-side lastModified date.
//
// MIGRATION_NOTE: $currentDate is a server-side operator — Mongo generates the
// timestamp, not the client. We intentionally do NOT use time.Now() here.
func (u *UpdateDocumentsImpl) UpdateDocumentWithCurrentDate(ctx context.Context) error {
	filter := bson.M{"name": "Sundar"}
	update := bson.M{
		"$set":         bson.M{"age": 23, "gender": "Male"},
		"$currentDate": bson.M{"lastModified": true},
	}

	result, err := u.collection().UpdateOne(ctx, filter, update)
	if err != nil {
		u.logger.Error().Err(err).Msg("Exception occurred while update Document with Date")
		return fmt.Errorf("update document with current date: %w", err)
	}

	u.logger.Info().
		Int64("matchedCount", result.MatchedCount).
		Int64("modifiedCount", result.ModifiedCount).
		Msg("Update with date completed")
	return nil
}

// Finalized closes the Mongo client. As the Java comment noted, the driver's
// connection pool normally manages lifecycle, but we close explicitly for
// safety and deterministic shutdown.
func (u *UpdateDocumentsImpl) Finalized(ctx context.Context) error {
	if err := u.conn.Close(ctx); err != nil {
		return fmt.Errorf("close mongo client: %w", err)
	}
	return nil
}
