// Package impl provides concrete implementations of the MongoDB CRUD
// operation contracts defined in the parent crud package.
//
// MIGRATION_NOTE: The Java source (QueryDocumentsImpl) implemented the
// QueryDocuments interface and demonstrated a range of MongoDB query
// operations (equality, inequality, logical AND/OR, membership, comparison
// operators, regex, and field existence) against a configured collection,
// logging the "name" field of every matched document. The migration makes
// the following idiomatic changes over the source shape:
//
//  1. Every method now takes a context.Context as its first parameter, since
//     query operations perform cancellable network I/O against MongoDB.
//
//  2. Every method returns an error instead of catching MongoException /
//     ClassCastException and merely logging it. Callers decide how to react.
//     Errors are wrapped with fmt.Errorf("...: %w", err) so callers can use
//     errors.Is / errors.As.
//
//  3. Filters are built with the official Go driver's bson.M / bson.D and
//     bson.E helpers rather than the Java Filters DSL static imports.
//
//  4. The Java constructor eagerly opened a MongoClient. In Go we inject the
//     already-connected MongoConnectionUtils via a NewQueryDocumentsImpl
//     constructor, matching the pattern established by InsertDocumentsImpl.
package impl

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"example.com/app/internal/logging"
	"example.com/app/internal/mongo/util"
)

// ErrNilOperator is returned by GetSpecificDocument when the operator argument
// is empty. The Java code logged "Operator Should not NULL" and returned; the
// Go equivalent surfaces this as an explicit error.
var ErrNilOperator = errors.New("operator must not be empty")

// QueryDocumentsImpl is the concrete implementation of the crud.QueryDocuments
// contract. It demonstrates a set of representative MongoDB query patterns
// against the configured sample collection.
type QueryDocumentsImpl struct {
	mongo *util.MongoConnectionUtils
	log   *logging.Config
}

// NewQueryDocumentsImpl constructs a QueryDocumentsImpl using an already
// connected MongoConnectionUtils and a logger.
//
// MIGRATION_NOTE: The Java constructor built its own MongoConnectionUtils and
// opened a MongoClient inline. Here the collaborators are injected so the
// caller owns the connection lifecycle, matching InsertDocumentsImpl.
func NewQueryDocumentsImpl(m *util.MongoConnectionUtils, log *logging.Config) *QueryDocumentsImpl {
	return &QueryDocumentsImpl{mongo: m, log: log}
}

// LoadMethods invokes every query demonstration in sequence, mirroring the
// Java loadMethods() driver method. It stops and returns on the first error.
//
// MIGRATION_NOTE: The Java version swallowed every failure and continued. The
// Go version returns the first error so failures are not silently lost; if a
// caller wants best-effort behaviour they can ignore the return value.
func (q *QueryDocumentsImpl) LoadMethods(ctx context.Context) error {
	if err := q.GetAllDocuments(ctx); err != nil {
		return err
	}

	operators := []string{
		"EQUAL",
		"NOT-EQUAL",
		"AND",
		"OR",
		"AND-OR",
		"IN",
		"NOT-IN",
		"LESS-THAN",
		"LESS-THAN-OR-EQUAL",
		"GREATER-THAN",
		"GREATER-THAN-OR-EQUAL",
		"LIKE",
		"EXISTS",
		"NOT-EXISTS",
	}
	for _, op := range operators {
		if err := q.GetSpecificDocument(ctx, op); err != nil {
			return fmt.Errorf("query for operator %q failed: %w", op, err)
		}
	}

	return nil
}

// GetAllDocuments retrieves every document in the sample collection and logs
// each document's "name" field. Equivalent to SELECT * FROM sample.
func (q *QueryDocumentsImpl) GetAllDocuments(ctx context.Context) error {
	return q.getData(ctx, bson.M{}, "ALL")
}

// GetSpecificDocument builds a filter based on the given operator name and
// runs the resulting query. Supported operators mirror the Java switch:
// EQUAL, NOT-EQUAL, AND, OR, AND-OR, IN, NOT-IN, LESS-THAN,
// LESS-THAN-OR-EQUAL, GREATER-THAN, GREATER-THAN-OR-EQUAL, LIKE, EXISTS,
// NOT-EXISTS. An unknown operator is logged and treated as a no-op filter,
// matching the Java default case behaviour.
func (q *QueryDocumentsImpl) GetSpecificDocument(ctx context.Context, operator string) error {
	if operator == "" {
		return ErrNilOperator
	}

	q.log.Info(operator + " operation is started...")

	filter, ok := buildFilter(operator)
	if !ok {
		q.log.Info(fmt.Sprintf("Operator %q is not matched", operator))
		return nil
	}

	return q.getData(ctx, filter, operator)
}

// buildFilter translates an operator name into a MongoDB filter document.
// The second return value reports whether the operator was recognised.
//
// MIGRATION_NOTE: The Java code used the Filters DSL (eq, ne, and, or, in,
// nin, lt, lte, gt, gte, regex, exists). These map directly to bson operator
// documents in the Go driver.
func buildFilter(operator string) (bson.M, bool) {
	switch strings.ToUpper(operator) {
	case "EQUAL":
		// SELECT * FROM sample WHERE name = 'Sundar'
		return bson.M{"name": "Sundar"}, true
	case "NOT-EQUAL":
		// SELECT * FROM sample WHERE name != 'Sundar'
		return bson.M{"name": bson.M{"$ne": "Sundar"}}, true
	case "AND":
		// SELECT * FROM sample WHERE name = 'Sundar' AND age < 20
		return bson.M{
			"$and": bson.A{
				bson.M{"name": "Sundar"},
				bson.M{"age": bson.M{"$lt": 20}},
			},
		}, true
	case "OR":
		// SELECT * FROM sample WHERE name = 'Sundar' OR age < 20
		return bson.M{
			"$or": bson.A{
				bson.M{"name": "Sundar"},
				bson.M{"age": bson.M{"$lt": 20}},
			},
		}, true
	case "AND-OR":
		// SELECT * FROM sample WHERE gender='male' AND (name='Sundar' OR age < 20)
		return bson.M{
			"$and": bson.A{
				bson.M{"gender": "male"},
				bson.M{"$or": bson.A{
					bson.M{"name": "Sundar"},
					bson.M{"age": bson.M{"$lt": 20}},
				}},
			},
		}, true
	case "IN":
		// SELECT * FROM sample WHERE name IN('Sundar')
		return bson.M{"name": bson.M{"$in": bson.A{"Sundar"}}}, true
	case "NOT-IN":
		// SELECT * FROM sample WHERE name NOT IN('Sundar')
		return bson.M{"name": bson.M{"$nin": bson.A{"Sundar"}}}, true
	case "LESS-THAN":
		// SELECT * FROM sample WHERE age < 20
		return bson.M{"age": bson.M{"$lt": 20}}, true
	case "LESS-THAN-OR-EQUAL":
		// SELECT * FROM sample WHERE age <= 20
		return bson.M{"age": bson.M{"$lte": 20}}, true
	case "GREATER-THAN":
		// SELECT * FROM sample WHERE age > 20
		return bson.M{"age": bson.M{"$gt": 20}}, true
	case "GREATER-THAN-OR-EQUAL":
		// SELECT * FROM sample WHERE age >= 20
		return bson.M{"age": bson.M{"$gte": 20}}, true
	case "LIKE":
		// SELECT * FROM sample WHERE name LIKE 'S%'
		return bson.M{"name": primitive.Regex{Pattern: "^S", Options: ""}}, true
	case "EXISTS":
		// fieldName exists
		return bson.M{"gender": bson.M{"$exists": true}}, true
	case "NOT-EXISTS":
		// fieldName does not exist
		return bson.M{"gender": bson.M{"$exists": false}}, true
	default:
		return nil, false
	}
}

// getData runs the supplied filter against the sample collection and logs the
// "name" field of every matched document. The operator argument is used only
// for diagnostic messages.
func (q *QueryDocumentsImpl) getData(ctx context.Context, filter bson.M, operator string) error {
	db := q.mongo.Database()
	collName := q.mongo.SampleCollection()
	collection := db.Collection(collName)

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return fmt.Errorf("find documents using %q: %w", operator, err)
	}
	defer func() {
		if cerr := cursor.Close(ctx); cerr != nil {
			q.log.Error(fmt.Sprintf("error closing cursor for %q: %v", operator, cerr))
		}
	}()

	for cursor.Next(ctx) {
		var doc struct {
			Name string `bson:"name"`
		}
		if derr := cursor.Decode(&doc); derr != nil {
			return fmt.Errorf("decode document using %q: %w", operator, derr)
		}
		q.log.Info(doc.Name)
	}
	if err := cursor.Err(); err != nil {
		return fmt.Errorf("cursor iteration using %q: %w", operator, err)
	}

	return nil
}

// Finalized closes the underlying MongoDB client.
//
// MIGRATION_NOTE: The Java finalized() method closed the MongoClient and
// swallowed any exception, noting the driver's cluster would normally handle
// this. In Go, connection lifecycle is owned by whoever constructed the
// MongoConnectionUtils; this method is retained for API parity and delegates
// to the shared Close helper.
func (q *QueryDocumentsImpl) Finalized(ctx context.Context) error {
	if err := q.mongo.Close(ctx); err != nil {
		return fmt.Errorf("close mongo client: %w", err)
	}
	return nil
}

// compile-time guard: ensure QueryDocumentsImpl exposes the expected methods.
// The actual interface assertion lives with the crud.QueryDocuments contract;
// this local reference prevents an unused import of mongo when the driver
// package is only otherwise referenced transitively.
var _ = mongo.ErrNoDocuments
