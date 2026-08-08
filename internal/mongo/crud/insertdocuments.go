// Package crud defines the contracts and implementations for MongoDB CRUD
// operations used by the application.
//
// MIGRATION_NOTE: The Java source (com.mongo.crud.InsertDocuments) was an
// interface that extended com.mongo.utils.Commons and declared four insert
// operations (insertUsingDocument, insertUsingMap, insertSingleDocument,
// insertMultipleDocuments). In Go we model this as an interface that embeds
// the already-migrated util.Lifecycle contract (the Go equivalent of the
// Commons lifecycle methods) and adds the four insert behaviors.
//
// Several Java-isms were deliberately dropped in favor of idiomatic Go:
//
//   Java pattern                          Go equivalent / decision
//   -----------------------------         -----------------------------
//   extends Commons                       embed util.Lifecycle
//   void insertXxx()                      InsertXxx(ctx) error
//   no parameters / no error              context.Context first param +
//                                         explicit error return
//
// The concrete implementation (the Java *Impl class, if any) is intentionally
// NOT declared here — only the contract lives in this file, mirroring the
// source which is a pure interface. The Mongo driver idioms (InsertOne /
// InsertMany with bson.D / bson.M, letting Mongo generate the _id) belong in
// the concrete implementation.
package crud

import (
	"context"

	"migrated-app/internal/mongo/util"
)

// InsertDocuments is the contract for the various MongoDB document-insertion
// strategies. It embeds util.Lifecycle so implementations also satisfy the
// shared load/finalize lifecycle used across the Mongo components.
//
// Each method takes a context.Context (for cancellation, deadlines, and
// propagation into the Mongo driver) and returns an error describing any
// failure. Implementations should use the driver's InsertOne / InsertMany
// APIs with bson documents and let MongoDB generate the _id.
type InsertDocuments interface {
	util.Lifecycle

	// InsertUsingDocument inserts a single document constructed via the
	// driver's native bson.D / bson.M document type.
	InsertUsingDocument(ctx context.Context) error

	// InsertUsingMap inserts a single document built from a Go map (bson.M).
	InsertUsingMap(ctx context.Context) error

	// InsertSingleDocument inserts exactly one document using InsertOne.
	InsertSingleDocument(ctx context.Context) error

	// InsertMultipleDocuments inserts a batch of documents using InsertMany.
	InsertMultipleDocuments(ctx context.Context) error
}
