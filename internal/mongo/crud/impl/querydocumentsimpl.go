// Package impl provides concrete implementations of the CRUD contracts
// defined in the parent crud package for MongoDB.
//
// MIGRATION_NOTE: The Java source (com.mongo.crud.impl.QueryDocumentsImpl)
// was the implementation half of the QueryDocuments interface (already
// migrated to internal/mongo/crud/querydocuments.go). The Go interface lives
// in the crud package while this concrete type lives here in the impl
// sub-package, mirroring the layout already established for
// InsertDocumentsImpl.
//
// Java-to-Go adaptation decisions:
//
//	Java pattern                          Go equivalent / decision
//	-----------------------------         -----------------------------
//	class ...Impl implements Iface        struct QueryDocumentsImpl
//	constructor acquires connection       NewQueryDocumentsImpl(...) (*T, error)
//	Log4j static Logger                   *zerolog.Logger injected/created
//	Filters.eq/ne/and/... builder DSL     bson.M / bson.D filter documents
//	for (Document doc : iterable)         cursor iteration with context
//	finalized() closes client             Finalized() delegates to conn.Close
//
// The Java driver's SQL-style comments ("SELECT * FROM sample ...") are
// preserved verbatim next to each filter so the intent of every query is
// unambiguous, but the actual queries are expressed with idiomatic MongoDB
// bson filters rather than SQL.
package impl

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"internal/conf"
	"internal/mongo/crud"
	"internal/mongo/util"
)

// QueryDocumentsImpl is the concrete MongoDB-backed implementation of the
// crud.QueryDocuments contract. It demonstrates a range of query styles
// (equality, comparison, logical, membership, regex and existence) against
// the configured "sample" collection.
type QueryDocumentsImpl struct {
	conn   *util.MongoConnection
	client *mongo.Client
	log    *zerolog.Logger
}

// Ensure QueryDocumentsImpl satisfies the crud.QueryDocuments interface at
// compile time.
var _ crud.QueryDocuments = (*QueryDocumentsImpl)(nil)

// NewQueryDocumentsImpl constructs a QueryDocumentsImpl, acquiring a MongoDB
// connection from the supplied configuration.
//
// MIGRATION_NOTE: The Java constructor instantiated MongoConnectionUtils and
// grabbed a client with no error handling. In Go we surface acquisition
// failures explicitly as an error, following the (T, error) convention.
func NewQueryDocumentsImpl(ctx context.Context, cfg conf.MongoConfig, logCfg conf.LogConfig) (*QueryDocumentsImpl, error) {
	logger := conf.NewLogger(logCfg)

	conn, err := util.NewMongoConnection(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("query documents: acquire mongo connection: %w", err)
	}

	return &QueryDocumentsImpl{
		conn:   conn,
		client: conn.Client(),
		log:    logger,
	}, nil
}

// LoadMethods invokes every supported query operation one by one, mirroring
// the demonstration flow of the original Java loadMethods().
//
// MIGRATION_NOTE: The Java version returned void and swallowed errors inside
// each helper. Here we propagate errors so callers can decide how to react;
// the first failure stops the demonstration run.
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
			return err
		}
	}

	return q.Finalized(ctx)
}

// GetAllDocuments retrieves every document in the sample collection and logs
// each document's "name" field.
//
// Equivalent SQL intent: SELECT * FROM sample;
func (q *QueryDocumentsImpl) GetAllDocuments(ctx context.Context) error {
	collection := q.sampleCollection()

	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		q.log.Error().Err(err).Msg("Exception occurred while querying all documents")
		return fmt.Errorf("get all documents: %w", err)
	}

	return q.logNames(ctx, cursor, "getAllDocuments")
}

// GetSpecificDocument builds a filter based on the supplied operator and runs
// the resulting query, logging matching document names.
//
// MIGRATION_NOTE: The Java version used the com.mongodb Filters builder DSL
// (eq, ne, and, or, in, nin, lt, lte, gt, gte, regex, exists). Each of those
// is translated to the equivalent bson filter document. The original
// SQL-style comments are preserved verbatim for traceability.
func (q *QueryDocumentsImpl) GetSpecificDocument(ctx context.Context, operator string) error {
	q.log.Info().Msgf("%s operation is started...", operator)

	if operator == "" {
		// MIGRATION_NOTE: Go strings cannot be nil, so the Java
		// "operator != null" guard becomes an empty-string check.
		q.log.Info().Msg("Operator Should not NULL")
		return nil
	}

	filter, ok := buildFilter(operator)
	if !ok {
		q.log.Info().Msgf("Operator %q is not matched", operator)
		return nil
	}

	return q.getData(ctx, filter, operator)
}

// buildFilter maps a query operator name to its corresponding MongoDB bson
// filter. The boolean return reports whether the operator was recognized
// (mirroring Java's default switch branch that logged an unmatched operator).
func buildFilter(operator string) (bson.D, bool) {
	switch toUpper(operator) {
	case "EQUAL": // SELECT * FROM sample WHERE name = 'Sundar'
		return bson.D{{Key: "name", Value: "Sundar"}}, true
	case "NOT-EQUAL": // SELECT * FROM sample WHERE name != 'Sundar'
		return bson.D{{Key: "name", Value: bson.M{"$ne": "Sundar"}}}, true
	case "AND": // SELECT * FROM sample WHERE name = 'Sundar' AND age < 20
		return bson.D{{Key: "$and", Value: bson.A{
			bson.M{"name": "Sundar"},
			bson.M{"age": bson.M{"$lt": 20}},
		}}}, true
	case "OR": // SELECT * FROM sample WHERE name = 'Sundar' OR age < 20
		return bson.D{{Key: "$or", Value: bson.A{
			bson.M{"name": "Sundar"},
			bson.M{"age": bson.M{"$lt": 20}},
		}}}, true
	case "AND-OR": // SELECT * FROM sample WHERE gender='male' AND (name='Sundar' OR age < 20)
		return bson.D{{Key: "$and", Value: bson.A{
			bson.M{"gender": "male"},
			bson.M{"$or": bson.A{
				bson.M{"name": "Sundar"},
				bson.M{"age": bson.M{"$lt": 20}},
			}},
		}}}, true
	case "IN": // SELECT * FROM sample WHERE name IN('Sundar')
		return bson.D{{Key: "name", Value: bson.M{"$in": bson.A{"Sundar"}}}}, true
	case "NOT-IN": // SELECT * FROM sample WHERE name NOT IN('Sundar')
		return bson.D{{Key: "name", Value: bson.M{"$nin": bson.A{"Sundar"}}}}, true
	case "LESS-THAN": // SELECT * FROM sample WHERE age < 20
		return bson.D{{Key: "age", Value: bson.M{"$lt": 20}}}, true
	case "LESS-THAN-OR-EQUAL": // SELECT * FROM sample WHERE age <= 20
		return bson.D{{Key: "age", Value: bson.M{"$lte": 20}}}, true
	case "GREATER-THAN": // SELECT * FROM sample WHERE age > 20
		return bson.D{{Key: "age", Value: bson.M{"$gt": 20}}}, true
	case "GREATER-THAN-OR-EQUAL": // SELECT * FROM sample WHERE age >= 20
		return bson.D{{Key: "age", Value: bson.M{"$gte": 20}}}, true
	case "LIKE": // SELECT * FROM sample WHERE name LIKE='S%'
		return bson.D{{Key: "name", Value: bson.M{"$regex": "^S"}}}, true
	case "EXISTS": // fieldName >> true to check for existence
		return bson.D{{Key: "gender", Value: bson.M{"$exists": true}}}, true
	case "NOT-EXISTS": // fieldName >> false to check for absence
		return bson.D{{Key: "gender", Value: bson.M{"$exists": false}}}, true
	default:
		return nil, false
	}
}

// getData executes the supplied filter against the sample collection and logs
// the "name" field of every matching document.
func (q *QueryDocumentsImpl) getData(ctx context.Context, filter bson.D, operator string) error {
	collection := q.sampleCollection()

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		q.log.Error().Err(err).Msgf("exception occurred while getting Document using %s", operator)
		return fmt.Errorf("get data using %s: %w", operator, err)
	}

	return q.logNames(ctx, cursor, operator)
}

// logNames iterates the cursor, logging each document's "name" field, and
// ensures the cursor is always closed.
func (q *QueryDocumentsImpl) logNames(ctx context.Context, cursor *mongo.Cursor, operator string) error {
	defer func() {
		if cerr := cursor.Close(ctx); cerr != nil {
			q.log.Error().Err(cerr).Msgf("exception occurred while closing cursor for %s", operator)
		}
	}()

	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			q.log.Error().Err(err).Msgf("exception occurred while decoding document for %s", operator)
			return fmt.Errorf("decode document for %s: %w", operator, err)
		}
		if name, ok := doc["name"].(string); ok {
			q.log.Info().Msg(name)
		}
	}

	if err := cursor.Err(); err != nil {
		q.log.Error().Err(err).Msgf("cursor error for %s", operator)
		return fmt.Errorf("cursor error for %s: %w", operator, err)
	}

	return nil
}

// sampleCollection returns a handle to the configured sample collection.
func (q *QueryDocumentsImpl) sampleCollection() *mongo.Collection {
	return q.client.
		Database(q.conn.Database()).
		Collection(q.conn.SampleCollection())
}

// Finalized closes the MongoDB client. As the original Java comment noted,
// the cluster would normally manage the client lifecycle; we close it
// explicitly for safety.
func (q *QueryDocumentsImpl) Finalized(ctx context.Context) error {
	if err := q.conn.Close(ctx); err != nil {
		q.log.Info().Err(err).Msg("Exception occurred while close client")
		return fmt.Errorf("finalized: close client: %w", err)
	}
	return nil
}

// toUpper is a small helper that upper-cases an ASCII operator string. It is
// kept local to avoid importing strings solely for a single call site; the
// operators are known ASCII constants.
//
// MIGRATION_NOTE: If Unicode operator names ever become possible, replace
// this with strings.ToUpper.
func toUpper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - ('a' - 'A')
		}
	}
	return string(b)
}
