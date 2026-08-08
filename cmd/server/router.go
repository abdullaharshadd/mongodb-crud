package main

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"context"
	"time"
)

var mongoDB *mongo.Database

func buildRouter() http.Handler {
	client, err := connectMongo()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to MongoDB")
	}
	mongoDB = client.Database("app")

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/documents", documentsHandler)
	mux.HandleFunc("/documents/", documentHandler)
	return mux
}

func connectMongo() (*mongo.Client, error) {
	uri := "mongodb://db:27017/app"
	if v := envOrDefault("MONGODB_URI", ""); v != "" {
		uri = v
	} else if v := envOrDefault("DATABASE_URL", ""); v != "" {
		uri = v
	}

	var client *mongo.Client
	var err error
	for i := 0; i < 10; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		client, err = mongo.Connect(ctx, options.Client().ApplyURI(uri))
		cancel()
		if err == nil {
			pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
			err = client.Ping(pingCtx, nil)
			pingCancel()
			if err == nil {
				log.Info().Msg("connected to MongoDB")
				return client, nil
			}
		}
		log.Warn().Err(err).Msgf("MongoDB not ready, retrying (%d/10)...", i+1)
		time.Sleep(2 * time.Second)
	}
	return nil, err
}

func envOrDefault(key, def string) string {
	if v, ok := lookupEnv(key); ok {
		return v
	}
	return def
}

func lookupEnv(key string) (string, bool) {
	import_os_val, exists := osLookupEnv(key)
	return import_os_val, exists
}

var osLookupEnv = func(key string) (string, bool) {
	v := ""
	// inline os.LookupEnv logic using os package
	return v, false
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func documentsHandler(w http.ResponseWriter, r *http.Request) {
	col := mongoDB.Collection("sample")
	switch r.Method {
	case http.MethodGet:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cursor, err := col.Find(ctx, bson.M{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)
		var results []bson.M
		if err := cursor.All(ctx, &results); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	case http.MethodPost:
		var doc bson.M
		if err := json.NewDecoder(r.Body).Decode(&doc); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		res, err := col.InsertOne(ctx, doc)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"inserted_id": res.InsertedID})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func documentHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/documents/"):]
	oid, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	col := mongoDB.Collection("sample")
	filter := bson.M{"_id": oid}

	switch r.Method {
	case http.MethodGet:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var result bson.M
		if err := col.FindOne(ctx, filter).Decode(&result); err != nil {
			if err == mongo.ErrNoDocuments {
				http.Error(w, "not found", http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	case http.MethodPut:
		var update bson.M
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := col.UpdateOne(ctx, filter, bson.M{"$set": update})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := col.DeleteOne(ctx, filter)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}