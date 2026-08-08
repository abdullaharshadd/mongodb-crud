// Package crud defines the contracts and implementations for MongoDB CRUD
// operations used by the application.
//
// MIGRATION_NOTE: The Java source (com.mongo.crud.QueryDocuments) was an
// interface that extended com.mongo.utils.Commons and declared two query
// operations (getAllDocuments, getSpecificDocument). In Go we model this as an
// interface that embeds the already-migrated util.Lifecycle contract (the Go
// equivalent of the Commons lifecycle methods) and adds the two query
// behaviors.
//
// Several Java-isms were deliberately adapted to idiomatic Go:
//
//   Java pattern                          Go equivalent / decision
//   -----------------------------         -----------------------------
//   extends Commons                       embeds util.Lifecycle
//   void getAllDocuments()                GetAllDocuments(ctx) ([]bson.M, error)
//     — the Java method printed/void      A read operation must return the
//                                         documents it fetched and any error.
//   void getSpecificDocument(String op)   GetSpecificDocument(ctx, operator)
//                                         (bson.M, error) — returns the matched
//                                         document plus an error. The Java
//                                         parameter name "operator" is
//                                         preserved.
//
// The concrete implementation of these methods (using the MongoDB driver's
// Find / FindOne with bson.M filters, letting Mongo generate the document
// _id) is intentionally NOT declared here — the Java source is a pure
// interface (contract) with no behavior, so this file mirrors that by only
// declaring the contract.
package crud

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/mongo/crud/internal/mongo/util"
)

// QueryDocuments is the contract for MongoDB document query operations.
//
// MIGRATION_NOTE: It embeds util.Lifecycle, which is the Go equivalent of the
// lifecycle methods that were bundled into the Java com.mongo.utils.Commons
// "constant interface". The two query methods below correspond to the Java
// getAllDocuments() and getSpecificDocument(String operator) declarations,
// adapted to return their results and an error rather than being void.
type QueryDocuments interface {
	util.Lifecycle

	// GetAllDocuments retrieves every document from the target collection.
	//
	// The provided context governs cancellation and deadlines for the
	// underlying MongoDB Find operation. It returns the matched documents
	// and any error encountered while querying.
	GetAllDocuments(ctx context.Context) ([]bson.M, error)

	// GetSpecificDocument retrieves a single document matched by the given
	// operator.
	//
	// The provided context governs cancellation and deadlines for the
	// underlying MongoDB FindOne operation. The operator argument mirrors the
	// Java parameter of the same name and is used to build the bson.M filter
	// in the concrete implementation. It returns the matched document and any
	// error encountered while querying.
	GetSpecificDocument(ctx context.Context, operator string) (bson.M, error)
}
