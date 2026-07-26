```go
package conf_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yourusername/yourproject/internal/conf"
)

// ---------------------------------------------------------------------------
// Constants / defaults
// ---------------------------------------------------------------------------

func TestDefaultConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{
			name:     "DefaultHost equals localhost",
			got:      conf.DefaultHost,
			expected: "localhost",
		},
		{
			name:     "DefaultPort equals 27017",
			got:      conf.DefaultPort,
			expected: 27017,
		},
		{
			name:     "DefaultDatabase equals sundar",
			got:      conf.DefaultDatabase,
			expected: "sundar",
		},
		{
			name:     "DefaultCollection equals sample",
			got:      conf.DefaultCollection,
			expected: "sample",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, tc.got)
		})
	}
}

// ---------------------------------------------------------------------------
// NewMongoConfig
// ---------------------------------------------------------------------------

func TestNewMongoConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		field         string
		expectedValue interface{}
	}{
		{
			name:          "host returns localhost",
			field:         "Host",
			expectedValue: "localhost",
		},
		{
			name:          "port returns 27017",
			field:         "Port",
			expectedValue: 27017,
		},
		{
			name:          "database returns sundar",
			field:         "Database",
			expectedValue: "sundar",
		},
		{
			name:          "collection returns sample",
			field:         "Collection",
			expectedValue: "sample",
		},
	}

	cfg := conf.NewMongoConfig()
	require.NotNil(t, cfg)

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			switch tc.field {
			case "Host":
				assert.Equal(t, tc.expectedValue, cfg.Host)
			case "Port":
				assert.Equal(t, tc.expectedValue, cfg.Port)
			case "Database":
				assert.Equal(t, tc.expectedValue, cfg.Database)
			case "Collection":
				assert.Equal(t, tc.expectedValue, cfg.Collection)
			}
		})
	}
}

func TestNewMongoConfig_NotNil(t *testing.T) {
	t.Parallel()
	cfg := conf.NewMongoConfig()
	assert.NotNil(t, cfg)
}

func TestNewMongoConfig_HostNonEmpty(t *testing.T) {
	t.Parallel()
	cfg := conf.NewMongoConfig()
	assert.NotEmpty(t, cfg.Host, "host must be a non-empty string")
}

func TestNewMongoConfig_PortInValidRange(t *testing.T) {
	t.Parallel()
	cfg := conf.NewMongoConfig()
	assert.GreaterOrEqual(t, cfg.Port, 1)
	assert.LessOrEqual(t, cfg.Port, 65535)
}

func TestNewMongoConfig_DatabaseNonEmpty(t *testing.T) {
	t.Parallel()
	cfg := conf.NewMongoConfig()
	assert.NotEmpty(t, cfg.Database, "database must be a non-empty string")
}

func TestNewMongoConfig_CollectionNonEmpty(t *testing.T) {
	t.Parallel()
	cfg := conf.NewMongoConfig()
	assert.NotEmpty(t, cfg.Collection, "collection must be a non-empty string")
}

// ---------------------------------------------------------------------------
// LoadMongoConfig – defaults (no env vars set)
// ---------------------------------------------------------------------------

func TestLoadMongoConfig_Defaults(t *testing.T) {
	// Ensure env vars are clean for this test.
	unsetMongoEnv(t)

	tests := []struct {
		name          string
		field         string
		expectedValue interface{}
	}{
		{
			name:          "host property returns localhost",
			field:         "Host",
			expectedValue: "localhost",
		},
		{
			name:          "port property returns 27017",
			field:         "Port",
			expectedValue: 27017,
		},
		{
			name:          "sundarDB property returns sundar",
			field:         "Database",
			expectedValue: "sundar",
		},
		{
			name:          "sampleCollection property returns sample",
			field:         "Collection",
			expectedValue: "sample",
		},
	}

	cfg, err := conf.LoadMongoConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			switch tc.field {
			case "Host":
				assert.Equal(t, tc.expectedValue, cfg.Host)
			case "Port":
				assert.Equal(t, tc.expectedValue, cfg.Port)
			case "Database":
				assert.Equal(t, tc.expectedValue, cfg.Database)
			case "Collection":
				assert.Equal(t, tc.expectedValue, cfg.Collection)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// LoadMongoConfig – environment variable overrides
// ---------------------------------------------------------------------------

func TestLoadMongoConfig_EnvOverrides(t *testing.T) {
	tests := []struct {
		name       string
		envKey     string
		envValue   string
		field      string
		assertFunc func(t *testing.T, cfg *conf.MongoConfig)
	}{
		{
			name:     "MONGO_HOST overrides default host",
			envKey:   "MONGO_HOST",
			envValue: "mongo.example.com",
			field:    "Host",
			assertFunc: func(t *testing.T, cfg *conf.MongoConfig) {
				assert.Equal(t, "mongo.example.com", cfg.Host)
			},
		},
		{
			name:     "MONGO_PORT overrides default port",
			envKey:   "MONGO_PORT",
			envValue: "27018",
			field:    "Port",
			assertFunc: func(t *testing.T, cfg *conf.MongoConfig) {
				assert.Equal(t, 27018, cfg.Port)
			},
		},
		{
			name:     "MONGO_DATABASE overrides default database",
			envKey:   "MONGO_DATABASE",
			envValue: "production_db",
			field:    "Database",
			assertFunc: func(t *testing.T, cfg *conf.MongoConfig) {
				assert.Equal(t, "production_db", cfg.Database)
			},
		},
		{
			name:     "MONGO_COLLECTION overrides default collection",
			envKey:   "MONGO_COLLECTION",
			envValue: "orders",
			field:    "Collection",
			assertFunc: func(t *testing.T, cfg *conf.MongoConfig) {
				assert.Equal(t, "orders", cfg.Collection)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Set and clean up the environment variable.
			t.Setenv(tc.envKey, tc.envValue)

			cfg, err := conf.LoadMongoConfig()
			require.NoError(t, err)
			require.NotNil(t, cfg)

			tc.assertFunc(t, cfg)
		})
	}
}

// ---------------------------------------------------------------------------
// LoadMongoConfig – invariants
// ---------------------------------------------------------------------------

func TestLoadMongoConfig_PortInValidRange(t *testing.T) {
	unsetMongoEnv(t)

	cfg, err := conf.LoadMongoConfig()
	require.NoError(t, err)

	assert.GreaterOrEqual(t, cfg.Port, 1, "port must be >= 1")
	assert.LessOrEqual(t, cfg.Port, 65535, "port must be <= 65535")
}

func TestLoadMongoConfig_AllFieldsPresent(t *testing.T) {
	unsetMongoEnv(t)

	cfg, err := conf.LoadMongoConfig()
	require.NoError(t, err)

	assert.NotEmpty(t, cfg.Host, "host must be present")
	assert.NotZero(t, cfg.Port, "port must be present")
	assert.NotEmpty(t, cfg.Database, "database must be present")
	assert.NotEmpty(t, cfg.Collection, "collection must be present")
}

// ---------------------------------------------------------------------------
// URI
// ---------------------------------------------------------------------------

func TestMongoConfig_URI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		host        string
		port        int
		expectedURI string
	}{
		{
			name:        "default host and port produce correct URI",
			host:        conf.DefaultHost,
			port:        conf.DefaultPort,
			expectedURI: "mongodb://localhost:27017",
		},
		{
			name:        "custom host and port produce correct URI",
			host:        "mongo.example.com",
			port:        27018,
			expectedURI: "mongodb://mongo.example.com:27018",
		},
		{
			name:        "IP address host produces correct URI",
			host:        "192.168.1.100",
			port:        27017,
			expectedURI: "mongodb://192.168.1.100:27017",
		},
		{
			name:        "non-standard port produces correct URI",
			host:        "localhost",
			port:        37017,
			expectedURI: "mongodb://localhost:37017",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := &conf.MongoConfig{
				Host:       tc.host,
				Port:       tc.port,
				Database:   conf.DefaultDatabase,
				Collection: conf.DefaultCollection,
			}
			assert.Equal(t, tc.expectedURI, cfg.URI())
		})
	}
}

func TestMongoConfig_URI_Format(t *testing.T) {
	t.Parallel()

	cfg := conf.NewMongoConfig()
	uri := cfg.URI()

	expected := fmt.Sprintf("mongodb://%s:%d", conf.DefaultHost, conf.DefaultPort)
	assert.Equal(t, expected, uri)
}

func TestMongoConfig_URI_HasMongoScheme(t *testing.T) {
	t.Parallel()

	cfg := conf.NewMongoConfig()
	uri := cfg.URI()

	assert.Contains(t, uri, "mongodb://", "URI must use the mongodb:// scheme")
}

// ---------------------------------------------------------------------------
// MongoConfig struct field validation
// ---------------------------------------------------------------------------

func TestMongoConfig_StructFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		cfg        *conf.MongoConfig
		assertFunc func(t *testing.T, cfg *conf.MongoConfig)
	}{
		{
			name: "host field is a valid hostname string",
			cfg: &conf.MongoConfig{
				Host:       "localhost",
				Port:       27017,
				Database:   "sundar",
				Collection: "sample",
			},
			assertFunc: func(t *testing.T, cfg *conf.MongoConfig) {
				assert.Equal(t, "localhost", cfg.Host)
				assert.IsType(t, "", cfg.Host)
			},
		},
		{
			name: "port field is a valid integer",
			cfg: &conf.MongoConfig{
				Host:       "localhost",
				Port:       27017,
				Database:   "sundar",
				Collection: "sample",
			},
			assertFunc: func(t *testing.T, cfg *conf.MongoConfig) {
				assert.Equal(t, 27017, cfg.Port)
				assert.IsType(t, 0, cfg.Port)
			},
		},
		{
			name: "database field is sundar",
			cfg: &conf.MongoConfig{
				Host:       "localhost",
				Port:       27017,
				Database:   "sundar",
				Collection: "sample",
			},
			assertFunc: func(t *testing.T, cfg *conf.MongoConfig) {
				assert.Equal(t, "sundar", cfg.Database)
			},
		},
		{
			name: "collection field is sample",
			cfg: &conf.MongoConfig{
				Host:       "localhost",
				Port:       27017,
				Database:   "sundar",
				Collection: "sample",
			},
			assertFunc: func(t *testing.T, cfg *conf.MongoConfig) {
				assert.Equal(t, "sample", cfg.Collection)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.assertFunc(t, tc.cfg)
		})
	}
}

// ---------------------------------------------------------------------------
// Error cases / edge cases
// ---------------------------------------------------------------------------

func TestLoadMongoConfig_InvalidPortEnv_FallsBackOrErrors(t *testing.T) {
	// If MONGO_PORT is set to a non-integer, viper may use the default or error.
	// We verify LoadMongoConfig doesn't panic and, when successful, the port
	// remains in a safe range.
	t.Setenv("MONGO_PORT", "not-a-number")

	cfg, err := conf.LoadMongoConfig()
	if err != nil {
		// An error is acceptable when env var is invalid.
		assert.Error(t, err)
	} else {
		// If viper falls back gracefully, the port should still be valid.
		assert.GreaterOrEqual(t, cfg.Port, 0)
	}
}

func TestMongoConfig_URI_NoEmptyHost(t *testing.T) {
	t.Parallel()

	cfg := conf.NewMongoConfig()
	uri := cfg.URI()
	assert.NotContains(t, uri, "mongodb://:27017", "URI should not have an empty host")
}

func TestMongoConfig_DefaultPort_IsStandardMongoDB(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 27017, conf.DefaultPort, "default port must be the standard MongoDB port 27017")
}

func TestMongoConfig_DefaultHost_IsLocalhost(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "localhost", conf.DefaultHost, "default host must be 'localhost'")
}

func TestMongoConfig_DefaultDatabase_IsSundar(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "sundar", conf.DefaultDatabase, "default database must be 'sundar'")
}

func TestMongoConfig_DefaultCollection_IsSample(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "sample", conf.DefaultCollection, "default collection must be 'sample'")
}

// ---------------------------------------------------------------------------
// All four properties present (global invariant)
// ---------------------------------------------------------------------------

func TestNewMongoConfig_AllFourPropertiesPresent(t *testing.T) {
	t.Parallel()

	cfg := conf.NewMongoConfig()
	require.NotNil(t, cfg)

	tests := []struct {
		name  string
		check func() bool
	}{
		{
			name:  "host property is present",
			check: func() bool { return cfg.Host !=