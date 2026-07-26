// Package impl provides concrete implementations of the MongoDB CRUD
// operation contracts defined in the parent crud package.
//
// MIGRATION_NOTE: The Java source (InsertDocumentsImpl) implemented the
// InsertDocuments interface and demonstrated several document-insertion
// techniques against a configured MongoDB collection. The migration makes the
// following idiomatic changes over the source shape:
//
//  1. Every method now takes a context.Context as its first parameter, since
//     insert operations perform cancellable network I/O against MongoDB.
//
//  2. Every method now returns an error instead of swallowing exceptions and
//     logging them (the Java code caught MongoException/ClassCastException and
//     merely logged). Callers decide how to react.
//
//  3. Documents are expressed with the official Go driver's bson.M / bson.D
//     rather than the Java driver's Document builder. We let Mongo generate
//     the _id where the Java code did not set one; the "insert using map"
//     case preserved an explicit numeric _id, so we keep that behaviour.
//
//  4. The Java no-arg constructor acquired a MongoClient as a side effect.
//     Here we use a NewInsertDocumentsImpl constructor that takes the already
//     migrated *util.MongoConnectionUtils, following dependency-injection
//     idioms rather than acquiring resources implicitly.
//
//  5. finalized() -> Close(), delegating to MongoConnectionUtils.Close.
package impl

import (
	"context"
	"fmt"
	"math/rand"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"migrated-app/internal/mongo/crud"
	"migrated-app/internal/mongo/util"
)

// InsertDocumentsImpl is the concrete implementation of the
// crud.InsertDocuments contract. It performs document inserts against the
// MongoDB collection configured in the supplied connection utilities.
type InsertDocumentsImpl struct {
	mongo *util.MongoConnectionUtils
}

// compile-time assertion that InsertDocumentsImpl satisfies the contract.
var _ crud.InsertDocuments = (*InsertDocumentsImpl)(nil)

// NewInsertDocumentsImpl constructs an InsertDocumentsImpl using the supplied
// MongoConnectionUtils.
//
// MIGRATION_NOTE: The Java constructor created a new MongoConnectionUtils and
// eagerly opened a client. Here the connection utilities are injected so the
// caller controls the lifecycle. The caller is expected to have already called
// Connect on the utils (or this type's methods will surface the resulting
// error from the driver).
func NewInsertDocumentsImpl(mongoUtils *util.MongoConnectionUtils) *InsertDocumentsImpl {
	return &InsertDocumentsImpl{mongo: mongoUtils}
}

// collection resolves the configured sample collection from the connection
// utilities. It centralises the db.getDatabase(...).getCollection(...) logic
// repeated in every Java method.
func (i *InsertDocumentsImpl) collection() (*mongo.Collection, error) {
	client := i.mongo.Connect()
	if client == nil {
		return nil, fmt.Errorf("insert documents: mongo client is not connected")
	}
	db := client.Database(i.mongo.Database())
	return db.Collection(i.mongo.SampleCollection()), nil
}

// LoadMethods invokes all of the insert demonstrations in sequence, mirroring
// the Java loadMethods(). It stops and returns on the first error rather than
// silently continuing.
func (i *InsertDocumentsImpl) LoadMethods(ctx context.Context) error {
	if err := i.InsertUsingDocument(ctx); err != nil {
		return fmt.Errorf("insert using document: %w", err)
	}
	if err := i.InsertUsingMap(ctx); err != nil {
		return fmt.Errorf("insert using map: %w", err)
	}
	if err := i.InsertSingleDocument(ctx); err != nil {
		return fmt.Errorf("insert single document: %w", err)
	}
	if err := i.InsertMultipleDocuments(ctx); err != nil {
		return fmt.Errorf("insert multiple documents: %w", err)
	}
	return i.Close(ctx)
}

// InsertUsingDocument inserts a single document built as a bson.D, mirroring
// the Java Document-object insert. Mongo generates the _id.
func (i *InsertDocumentsImpl) InsertUsingDocument(ctx context.Context) error {
	collection, err := i.collection()
	if err != nil {
		return err
	}
	doc := bson.D{
		{Key: "name", Value: "Sivaraman"},
		{Key: "age", Value: 23},
		{Key: "gender", Value: "male"},
	}
	if _, err := collection.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("inserting value using Document: %w", err)
	}
	return nil
}

// InsertUsingMap inserts a single document built from a map, mirroring the
// Java Map-based insert. It preserves the explicit random numeric _id set by
// the Java code.
func (i *InsertDocumentsImpl) InsertUsingMap(ctx context.Context) error {
	collection, err := i.collection()
	if err != nil {
		return err
	}
	// MIGRATION_NOTE: The Java code set an explicit _id via new Random().nextInt(999).
	// We preserve that behaviour rather than letting Mongo auto-generate the _id,
	// since it was an intentional part of the demonstration.
	emp := bson.M{
		"_id":         rand.Intn(999),
		"name":        "Vel",
		"age":         25,
		"desicnation": "Java Developer",
		"gender":      "Male",
		"salary":      "10000",
	}
	if _, err := collection.InsertOne(ctx, emp); err != nil {
		return fmt.Errorf("inserting value using Map: %w", err)
	}
	return nil
}

// InsertSingleDocument inserts a single nested document, mirroring the Java
// "canvas" insert with an embedded size sub-document.
func (i *InsertDocumentsImpl) InsertSingleDocument(ctx context.Context) error {
	collection, err := i.collection()
	if err != nil {
		return err
	}
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
	if _, err := collection.InsertOne(ctx, canvas); err != nil {
		return fmt.Errorf("inserting single document: %w", err)
	}
	return nil
}

// InsertMultipleDocuments inserts several documents at once, mirroring the
// Java insertMany with journal, mat and mousePad documents.
func (i *InsertDocumentsImpl) InsertMultipleDocuments(ctx context.Context) error {
	collection, err := i.collection()
	if err != nil {
		return err
	}
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
	docs := []interface{}{journal, mat, mousePad}
	if _, err := collection.InsertMany(ctx, docs); err != nil {
		return fmt.Errorf("inserting multiple documents: %w", err)
	}
	return nil
}

// Close releases the underlying MongoDB client via the connection utilities.
//
// MIGRATION_NOTE: This corresponds to the Java finalized() method, which
// delegated to MongoConnectionUtils.closeMongoClient (migrated to Close).
func (i *InsertDocumentsImpl) Close(ctx context.Context) error {
	return i.mongo.Close(ctx)
}
