// Package conf provides MongoDB connection configuration for the application.
//
// MIGRATION_NOTE: The source was a Java/Spring `.properties` file
// (conf/mongo.properties) that externalized MongoDB connection parameters
// (host, port) and the target database/collection names. In idiomatic Go we
// express these as a typed configuration struct with sensible defaults, a
// constructor, and environment-variable overrides via viper (consistent with
// internal/config/config.go). The Spring-style host+port pair is exposed here
// but also composed into a standard MongoDB connection URI, which is what the
// go.mongodb.org/mongo-driver client expects.
package conf

import (
	"fmt"

	"github.com/spf13/viper"
)

// Default MongoDB configuration values, migrated from conf/mongo.properties.
const (
	// DefaultHost is the default MongoDB host (source: host=localhost).
	DefaultHost = "localhost"
	// DefaultPort is the default MongoDB port (source: port=27017).
	DefaultPort = 27017
	// DefaultDatabase is the default MongoDB database name (source: sundarDB=sundar).
	DefaultDatabase = "sundar"
	// DefaultCollection is the default MongoDB collection name (source: sampleCollection=sample).
	DefaultCollection = "sample"
)

// MongoConfig holds the MongoDB connection parameters and target
// database/collection names for the application.
type MongoConfig struct {
	// Host is the MongoDB server hostname.
	Host string `mapstructure:"MONGO_HOST"`
	// Port is the MongoDB server port.
	Port int `mapstructure:"MONGO_PORT"`
	// Database is the target MongoDB database name.
	Database string `mapstructure:"MONGO_DATABASE"`
	// Collection is the target MongoDB collection name.
	Collection string `mapstructure:"MONGO_COLLECTION"`
}

// NewMongoConfig returns a MongoConfig populated with the default values
// migrated from conf/mongo.properties.
func NewMongoConfig() *MongoConfig {
	return &MongoConfig{
		Host:       DefaultHost,
		Port:       DefaultPort,
		Database:   DefaultDatabase,
		Collection: DefaultCollection,
	}
}

// LoadMongoConfig builds a MongoConfig by applying the defaults from
// conf/mongo.properties and then overriding them with any matching
// environment variables (MONGO_HOST, MONGO_PORT, MONGO_DATABASE,
// MONGO_COLLECTION). It returns an error if the configuration cannot be
// unmarshalled.
func LoadMongoConfig() (*MongoConfig, error) {
	v := viper.New()
	v.AutomaticEnv()
	v.SetDefault("MONGO_HOST", DefaultHost)
	v.SetDefault("MONGO_PORT", DefaultPort)
	v.SetDefault("MONGO_DATABASE", DefaultDatabase)
	v.SetDefault("MONGO_COLLECTION", DefaultCollection)

	cfg := NewMongoConfig()
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal mongo config: %w", err)
	}
	return cfg, nil
}

// URI returns a standard MongoDB connection URI composed from the configured
// host and port. This is the form expected by the mongo-driver client
// (e.g. options.Client().ApplyURI(cfg.URI())).
func (c *MongoConfig) URI() string {
	return fmt.Sprintf("mongodb://%s:%d", c.Host, c.Port)
}
