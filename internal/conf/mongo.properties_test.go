```go
package conf_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"migrated-app/internal/conf"
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


// ---------------------------------------------------------------------------
// LoadMongoConfig – invariants
// ---------------------------------------------------------------------------



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



// ---------------------------------------------------------------------------
// MongoConfig struct field validation
// ---------------------------------------------------------------------------


// ---------------------------------------------------------------------------
// Error cases / edge cases
// ---------------------------------------------------------------------------







// ---------------------------------------------------------------------------
// All four properties present (global invariant)
// ---------------------------------------------------------------------------

