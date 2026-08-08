// Package conf provides externalized configuration for the application's
// MongoDB connection settings and target database/collection names.
//
// This file replaces the legacy conf/mongo.properties file. In idiomatic Go
// we express configuration as typed structs loaded from the environment
// (via viper) rather than a raw .properties file consumed by Spring's
// @PropertySource / @Value machinery.
package conf

import (
	"fmt"

	"github.com/spf13/viper"
)

// Default MongoDB connection values, mirroring the original
// conf/mongo.properties file.
const (
	defaultHost             = "localhost"
	defaultPort             = 27017
	defaultDatabase         = "sundar"
	defaultSampleCollection = "sample"
)

// MongoConfig holds the MongoDB connection settings and the names of the
// database and collections the application operates on. It is the typed
// replacement for the key/value pairs previously stored in mongo.properties.
type MongoConfig struct {
	// Host is the MongoDB server hostname.
	Host string `mapstructure:"MONGO_HOST"`
	// Port is the MongoDB server port.
	Port int `mapstructure:"MONGO_PORT"`
	// Database is the target database name (formerly "sundarDB").
	Database string `mapstructure:"MONGO_DATABASE"`
	// SampleCollection is the name of the sample collection.
	SampleCollection string `mapstructure:"MONGO_SAMPLE_COLLECTION"`
}

// URI returns a MongoDB connection URI built from the configured host and
// port. Use this when constructing options.Client().ApplyURI(...).
func (c *MongoConfig) URI() string {
	return fmt.Sprintf("mongodb://%s:%d", c.Host, c.Port)
}

// LoadMongoConfig loads the MongoDB configuration from environment variables,
// falling back to the defaults defined in the original mongo.properties file.
//
// MIGRATION_NOTE: The original .properties keys (host, port, sundarDB,
// sampleCollection) are ambiguously named. They have been namespaced with a
// MONGO_ prefix to avoid collisions with other config sources (e.g. an HTTP
// server "port"). Adjust the env var names below if your deployment expects
// different keys.
//
// MIGRATION_NOTE: viper.AutomaticEnv() alone does not populate
// mapstructure-tagged fields on Unmarshal unless viper already knows about
// each key. We therefore call SetDefault for every key first — this both
// registers the key (so env vars are picked up) and supplies the original
// .properties defaults. This mirrors the fix documented in
// internal/config/config.go. Per the migration guidance, Load must run after
// validateArgs in the application startup sequence.
func LoadMongoConfig() (*MongoConfig, error) {
	v := viper.New()
	v.AutomaticEnv()

	v.SetDefault("MONGO_HOST", defaultHost)
	v.SetDefault("MONGO_PORT", defaultPort)
	v.SetDefault("MONGO_DATABASE", defaultDatabase)
	v.SetDefault("MONGO_SAMPLE_COLLECTION", defaultSampleCollection)

	var cfg MongoConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("conf: unmarshal mongo config: %w", err)
	}

	if cfg.Host == "" {
		return nil, fmt.Errorf("conf: mongo host must not be empty")
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return nil, fmt.Errorf("conf: mongo port %d out of range", cfg.Port)
	}
	if cfg.Database == "" {
		return nil, fmt.Errorf("conf: mongo database must not be empty")
	}
	if cfg.SampleCollection == "" {
		return nil, fmt.Errorf("conf: mongo sample collection must not be empty")
	}

	return &cfg, nil
}
