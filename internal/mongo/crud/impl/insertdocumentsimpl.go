// Package impl provides concrete implementations of the CRUD contracts
// defined in the parent crud package for MongoDB.
//
// MIGRATION_NOTE: The Java source (com.mongo.crud.impl.InsertDocumentsImpl)
// was the implementation half of the InsertDocuments interface (already
// migrated to internal/mongo/crud/insertdocuments.go). In Go we keep the
// interface in the crud package and place the concrete type here in a
// sub-package to avoid a name collision inside the crud package.
//
// Java-to-Go adaptation decisions:
//
//	Java pattern                          Go equivalent / decision
//	-----------------------------         -----------------------------
//	class ...Impl implements Iface        struct InsertDocumentsImpl + iface
//	constructor acquires resources        NewInsertDocumentsImpl returns error
//	log4j Logger                          internal.NewLogger (zerolog)
//	swallow MongoException, log only      return wrapped error to caller
//	org.bson.Document builder             bson.D / bson.M
//	collection.insertOne/insertMany       InsertOne / InsertMany (Go driver)
//	context-less driver calls             context.Context first parameter
//
MIGRATION_NOTE (transcription): the toy documents (name/age/gender for the
// Document/Map variants; item/qty/tags/size for the single/multiple variants)
// and the choice of InsertOne vs InsertMany were preserved exactly from the
// Java source. insertUsingMap and insertSingleDocument use InsertOne;
// insertMultipleDocuments uses InsertMany.
package impl

import (
	"context"
	"fmt"
	"math/rand"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/example/app/internal"
	"github.com/example/app/internal/conf"
	"github.com/example/app/internal/mongo/crud"
	"github.com/example/app/internal/mongo/util"
)

// InsertDocumentsImpl is the MongoDB-backed implementation of the
// crud.InsertDocuments interface. It demonstrates several ways of inserting
// documents into a collection.
//
// MIGRATION_NOTE: import paths above use a placeholder module path
// (github.com/example/app). Adjust them to match the real go.mod module path
// during integration.
type InsertDocumentsImpl struct {
	conn   *util.MongoConnection
	cfg    *conf.MongoConfig
	logger internal.Logger
}

// compile-time assertion that InsertDocumentsImpl satisfies the interface.
var _ crud.InsertDocuments = (*InsertDocumentsImpl)(nil)

// NewInsertDocumentsImpl constructs an InsertDocumentsImpl, acquiring a Mongo
// connection. Unlike the Java constructor (which swallowed failures), any
// setup error is returned to the caller.
//
// MIGRATION_NOTE: The Java constructor created its own MongoConnectionUtils
// and eagerly opened a client. Here we accept the already-migrated
// dependencies so the type is testable and composable.
func NewInsertDocumentsImpl(conn *util.MongoConnection, cfg *conf.MongoConfig, logger internal.Logger) (*InsertDocumentsImpl, error) {
	if conn == nil {
		return nil, fmt.Errorf("NewInsertDocumentsImpl: mongo connection must not be nil")
	}
	if cfg == nil {
		return nil, fmt.Errorf("NewInsertDocumentsImpl: mongo config must not be nil")
	}
	return &InsertDocumentsImpl{conn: conn, cfg: cfg, logger: logger}, nil
}

// collection returns the configured sample collection handle.
func (i *InsertDocumentsImpl) collection() *mongo.Collection {
	return i.conn.Client().Database(i.cfg.Database).Collection(i.cfg.SampleCollection)
}

// LoadMethods invokes all insert demonstrations in sequence. Unlike the Java
// version (which ignored failures of individual steps), it returns the first
// error encountered.
func (i *InsertDocumentsImpl) LoadMethods(ctx context.Context) error {
	if err := i.InsertUsingDocument(ctx); err != nil {
		return err
	}
	if err := i.InsertUsingMap(ctx); err != nil {
		return err
	}
	if err := i.InsertSingleDocument(ctx); err != nil {
		return err
	}
	if err := i.InsertMultipleDocuments(ctx); err != nil {
		return err
	}
	return i.Finalized(ctx)
}

// InsertUsingDocument inserts a single document built field-by-field, mirroring
// the Java Document-object variant.
func (i *InsertDocumentsImpl) InsertUsingDocument(ctx context.Context) error {
	doc := bson.D{
		{Key: "name", Value: "Sivaraman"},
		{Key: "age", Value: 23},
		{Key: "gender", Value: "male"},
	}
	if _, err := i.collection().InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("insert using Document: %w", err)
	}
	if i.logger != nil {
		i.logger.Info("Document Insert Successfully using Document Obj...")
	}
	return nil
}

// InsertUsingMap inserts a single document built from a map, mirroring the Java
// Map variant.
//
// MIGRATION_NOTE: The Java code set an explicit random int "_id". Idiomatic
// Mongo usage lets the server generate the _id, but the source's business
// intent (an explicit random _id) is preserved here to keep behavior exact.
func (i *InsertDocumentsImpl) InsertUsingMap(ctx context.Context) error {
	emp := bson.M{
		"_id":         rand.Intn(999),
		"name":        "Vel",
		"age":         25,
		"desicnation": "Java Developer",
		"gender":      "Male",
		"salary":      "10000",
	}
	if i.logger != nil {
		i.logger.Info(fmt.Sprintf("Employ Details : %v", emp))
	}
	if _, err := i.collection().InsertOne(ctx, emp); err != nil {
		return fmt.Errorf("insert using Map: %w", err)
	}
	if i.logger != nil {
		i.logger.Info("Document Insert Successfully using Map...")
	}
	return nil
}

// InsertSingleDocument inserts a single nested document (the "canvas" item),
// mirroring the Java single-document variant.
func (i *InsertDocumentsImpl) InsertSingleDocument(ctx context.Context) error {
	canvas := bson.D{
		{Key: "item", Value: "canvas"},
		{Key: "qty", Value: 100},
		{Key: "tags", Value: bson.A{"cotton"}},
		{Key: "size", Value: bson.D{
			{Key: "h", Value: 28},
			{Key: "w", Value: 35.5},
			{Key: "uom", Value: "cm"},
		}},
	}
	if _, err := i.collection().InsertOne(ctx, canvas); err != nil {
		return fmt.Errorf("insert single document: %w", err)
	}
	if i.logger != nil {
		i.logger.Info("Single Document Insert Successfully...")
	}
	return nil
}

// InsertMultipleDocuments inserts several nested documents at once (journal,
// mat, mousePad), mirroring the Java multiple-document variant which used
// insertMany.
func (i *InsertDocumentsImpl) InsertMultipleDocuments(ctx context.Context) error {
	journal := bson.D{
		{Key: "item", Value: "journal"},
		{Key: "qty", Value: 25},
		{Key: "tags", Value: bson.A{"blank", "red"}},
		{Key: "size", Value: bson.D{
			{Key: "h", Value: 14},
			{Key: "w", Value: 21},
			{Key: "uom", Value: "cm"},
		}},
	}
	mat := bson.D{
		{Key: "item", Value: "mat"},
		{Key: "qty", Value: 85},
		{Key: "tags", Value: bson.A{"gray"}},
		{Key: "size", Value: bson.D{
			{Key: "h", Value: 27.9},
			{Key: "w", Value: 35.5},
			{Key: "uom", Value: "cm"},
		}},
	}
	mousePad := bson.D{
		{Key: "item", Value: "mousePad"},
		{Key: "qty", Value: 25},
		{Key: "tags", Value: bson.A{"gel", "blue"}},
		{Key: "size", Value: bson.D{
			{Key: "h", Value: 19},
			{Key: "w", Value: 22.85},
			{Key: "uom", Value: "cm"},
		}},
	}
	if _, err := i.collection().InsertMany(ctx, []interface{}{journal, mat, mousePad}); err != nil {
		return fmt.Errorf("insert multiple documents: %w", err)
	}
	if i.logger != nil {
		i.logger.Info("Multiple Document Insert Successfully...")
	}
	return nil
}

// Finalized closes the underlying Mongo client. As the Java comment noted, the
// driver/cluster generally manages connection teardown, but we close
// explicitly for safety.
func (i *InsertDocumentsImpl) Finalized(ctx context.Context) error {
	if err := i.conn.Close(ctx); err != nil {
		return fmt.Errorf("finalize (close mongo client): %w", err)
	}
	return nil
}
