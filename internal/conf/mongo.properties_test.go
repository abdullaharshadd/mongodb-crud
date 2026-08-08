```go
package conf

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// setEnv sets environment variables for the duration of a test and restores
// them via t.Cleanup so that parallel or sequential tests are not polluted.
func setEnv(t *testing.T, key, value string) {
	t.Helper()
	t.Setenv(key, value)
}

// ---------------------------------------------------------------------------
// MongoConfig.URI tests
// ---------------------------------------------------------------------------

func TestMongoConfig_URI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      MongoConfig
		expected string
	}{
		{
			name:     "default host and port",
			cfg:      MongoConfig{Host: "localhost", Port: 27017},
			expected: "mongodb://localhost:27017",
		},
		{
			name:     "custom host and port",
			cfg:      MongoConfig{Host: "mongo.example.com", Port: 27018},
			expected: "mongodb://mongo.example.com:27018",
		},
		{
			name:     "IP address host",
			cfg:      MongoConfig{Host: "192.168.1.100", Port: 27017},
			expected: "mongodb://192.168.1.100:27017",
		},
		{
			name:     "non-standard port",
			cfg:      MongoConfig{Host: "localhost", Port: 1234},
			expected: "mongodb://localhost:1234",
		},
		{
			name:     "port at lower boundary",
			cfg:      MongoConfig{Host: "localhost", Port: 1},
			expected: "mongodb://localhost:1",
		},
		{
			name:     "port at upper boundary",
			cfg:      MongoConfig{Host: "localhost", Port: 65535},
			expected: "mongodb://localhost:65535",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.cfg.URI()
			assert.Equal(t, tc.expected, got)
		})
	}
}

// ---------------------------------------------------------------------------
// LoadMongoConfig – defaults (no env overrides)
// ---------------------------------------------------------------------------

func TestLoadMongoConfig_Defaults(t *testing.T) {
	t.Parallel()

	// No env vars set – all values come from the coded defaults.
	cfg, err := LoadMongoConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, defaultHost, cfg.Host, "default host must be 'localhost'")
	assert.Equal(t, defaultPort, cfg.Port, "default port must be 27017")
	assert.Equal(t, defaultDatabase, cfg.Database, "default database must be 'sundar'")
	assert.Equal(t, defaultSampleCollection, cfg.SampleCollection, "default collection must be 'sample'")
}

// ---------------------------------------------------------------------------
// LoadMongoConfig – individual field defaults
// ---------------------------------------------------------------------------

func TestLoadMongoConfig_DefaultHost(t *testing.T) {
	t.Parallel()

	cfg, err := LoadMongoConfig()
	require.NoError(t, err)
	assert.Equal(t, "localhost", cfg.Host,
		"host property must default to 'localhost' as defined in the original mongo.properties")
}

func TestLoadMongoConfig_DefaultPort(t *testing.T) {
	t.Parallel()

	cfg, err := LoadMongoConfig()
	require.NoError(t, err)
	assert.Equal(t, 27017, cfg.Port,
		"port property must default to 27017 (standard MongoDB port)")
}

func TestLoadMongoConfig_DefaultDatabase(t *testing.T) {
	t.Parallel()

	cfg, err := LoadMongoConfig()
	require.NoError(t, err)
	assert.Equal(t, "sundar", cfg.Database,
		"sundarDB property must default to 'sundar'")
}

func TestLoadMongoConfig_DefaultSampleCollection(t *testing.T) {
	t.Parallel()

	cfg, err := LoadMongoConfig()
	require.NoError(t, err)
	assert.Equal(t, "sample", cfg.SampleCollection,
		"sampleCollection property must default to 'sample'")
}

// ---------------------------------------------------------------------------
// LoadMongoConfig – env-var overrides (table-driven)
// ---------------------------------------------------------------------------

func TestLoadMongoConfig_EnvOverrides(t *testing.T) {
	tests := []struct {
		name             string
		envHost          string
		envPort          string
		envDatabase      string
		envCollection    string
		expectedHost     string
		expectedPort     int
		expectedDatabase string
		expectedColl     string
	}{
		{
			name:             "all overridden",
			envHost:          "mongo.prod.example.com",
			envPort:          "27018",
			envDatabase:      "prodDB",
			envCollection:    "prodCollection",
			expectedHost:     "mongo.prod.example.com",
			expectedPort:     27018,
			expectedDatabase: "prodDB",
			expectedColl:     "prodCollection",
		},
		{
			name:             "host override only",
			envHost:          "remote-host",
			envPort:          "",
			envDatabase:      "",
			envCollection:    "",
			expectedHost:     "remote-host",
			expectedPort:     27017,
			expectedDatabase: "sundar",
			expectedColl:     "sample",
		},
		{
			name:             "port override only",
			envHost:          "",
			envPort:          "37017",
			envDatabase:      "",
			envCollection:    "",
			expectedHost:     "localhost",
			expectedPort:     37017,
			expectedDatabase: "sundar",
			expectedColl:     "sample",
		},
		{
			name:             "database override only",
			envHost:          "",
			envPort:          "",
			envDatabase:      "myDB",
			envCollection:    "",
			expectedHost:     "localhost",
			expectedPort:     27017,
			expectedDatabase: "myDB",
			expectedColl:     "sample",
		},
		{
			name:             "collection override only",
			envHost:          "",
			envPort:          "",
			envDatabase:      "",
			envCollection:    "myCollection",
			expectedHost:     "localhost",
			expectedPort:     27017,
			expectedDatabase: "sundar",
			expectedColl:     "myCollection",
		},
		{
			name:             "port boundary lower (1)",
			envHost:          "",
			envPort:          "1",
			envDatabase:      "",
			envCollection:    "",
			expectedHost:     "localhost",
			expectedPort:     1,
			expectedDatabase: "sundar",
			expectedColl:     "sample",
		},
		{
			name:             "port boundary upper (65535)",
			envHost:          "",
			envPort:          "65535",
			envDatabase:      "",
			envCollection:    "",
			expectedHost:     "localhost",
			expectedPort:     65535,
			expectedDatabase: "sundar",
			expectedColl:     "sample",
		},
		{
			name:             "IP address as host",
			envHost:          "10.0.0.1",
			envPort:          "",
			envDatabase:      "",
			envCollection:    "",
			expectedHost:     "10.0.0.1",
			expectedPort:     27017,
			expectedDatabase: "sundar",
			expectedColl:     "sample",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Do NOT run in parallel because t.Setenv manipulates the process
			// environment; parallel sub-tests would race.
			if tc.envHost != "" {
				setEnv(t, "MONGO_HOST", tc.envHost)
			}
			if tc.envPort != "" {
				setEnv(t, "MONGO_PORT", tc.envPort)
			}
			if tc.envDatabase != "" {
				setEnv(t, "MONGO_DATABASE", tc.envDatabase)
			}
			if tc.envCollection != "" {
				setEnv(t, "MONGO_SAMPLE_COLLECTION", tc.envCollection)
			}

			cfg, err := LoadMongoConfig()
			require.NoError(t, err)
			require.NotNil(t, cfg)

			assert.Equal(t, tc.expectedHost, cfg.Host)
			assert.Equal(t, tc.expectedPort, cfg.Port)
			assert.Equal(t, tc.expectedDatabase, cfg.Database)
			assert.Equal(t, tc.expectedColl, cfg.SampleCollection)
		})
	}
}

// ---------------------------------------------------------------------------
// LoadMongoConfig – validation error cases (table-driven)
// ---------------------------------------------------------------------------

func TestLoadMongoConfig_ValidationErrors(t *testing.T) {
	tests := []struct {
		name          string
		envVars       map[string]string
		expectErrFrag string
	}{
		{
			name: "empty host is rejected",
			envVars: map[string]string{
				"MONGO_HOST":              "",
				"MONGO_PORT":              "27017",
				"MONGO_DATABASE":          "sundar",
				"MONGO_SAMPLE_COLLECTION": "sample",
			},
			expectErrFrag: "host must not be empty",
		},
		{
			name: "port zero is rejected",
			envVars: map[string]string{
				"MONGO_HOST":              "localhost",
				"MONGO_PORT":              "0",
				"MONGO_DATABASE":          "sundar",
				"MONGO_SAMPLE_COLLECTION": "sample",
			},
			expectErrFrag: "out of range",
		},
		{
			name: "negative port is rejected",
			envVars: map[string]string{
				"MONGO_HOST":              "localhost",
				"MONGO_PORT":              "-1",
				"MONGO_DATABASE":          "sundar",
				"MONGO_SAMPLE_COLLECTION": "sample",
			},
			expectErrFrag: "out of range",
		},
		{
			name: "port above 65535 is rejected",
			envVars: map[string]string{
				"MONGO_HOST":              "localhost",
				"MONGO_PORT":              "65536",
				"MONGO_DATABASE":          "sundar",
				"MONGO_SAMPLE_COLLECTION": "sample",
			},
			expectErrFrag: "out of range",
		},
		{
			name: "empty database is rejected",
			envVars: map[string]string{
				"MONGO_HOST":              "localhost",
				"MONGO_PORT":              "27017",
				"MONGO_DATABASE":          "",
				"MONGO_SAMPLE_COLLECTION": "sample",
			},
			expectErrFrag: "database must not be empty",
		},
		{
			name: "empty sample collection is rejected",
			envVars: map[string]string{
				"MONGO_HOST":              "localhost",
				"MONGO_PORT":              "27017",
				"MONGO_DATABASE":          "sundar",
				"MONGO_SAMPLE_COLLECTION": "",
			},
			expectErrFrag: "sample collection must not be empty",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.envVars {
				setEnv(t, k, v)
			}

			cfg, err := LoadMongoConfig()
			assert.Error(t, err, "expected a validation error but got none")
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), tc.expectErrFrag,
				fmt.Sprintf("error message should contain %q", tc.expectErrFrag))
		})
	}
}

// ---------------------------------------------------------------------------
// Invariant: all four keys must be present / non-empty for a valid config
// ---------------------------------------------------------------------------

func TestLoadMongoConfig_AllFourKeysMustBePresent(t *testing.T) {
	// Verify that a fully-specified config (all four keys present and valid)
	// always returns a non-nil config without error.
	t.Run("all four keys valid", func(t *testing.T) {
		setEnv(t, "MONGO_HOST", "mongo.test.local")
		setEnv(t, "MONGO_PORT", "27017")
		setEnv(t, "MONGO_DATABASE", "testdb")
		setEnv(t, "MONGO_SAMPLE_COLLECTION", "testcol")

		cfg, err := LoadMongoConfig()
		require.NoError(t, err)
		require.NotNil(t, cfg)

		assert.NotEmpty(t, cfg.Host)
		assert.Greater(t, cfg.Port, 0)
		assert.LessOrEqual(t, cfg.Port, 65535)
		assert.NotEmpty(t, cfg.Database)
		assert.NotEmpty(t, cfg.SampleCollection)
	})
}

// ---------------------------------------------------------------------------
// Invariant: URI is formed correctly for various host/port combos
// ---------------------------------------------------------------------------

func TestMongoConfig_URI_FormsValidEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		host     string
		port     int
		wantURI  string
	}{
		{
			name:    "defaults form valid URI",
			host:    defaultHost,
			port:    defaultPort,
			wantURI: "mongodb://localhost:27017",
		},
		{
			name:    "custom host and port form valid URI",
			host:    "db.prod.internal",
			port:    27018,
			wantURI: "mongodb://db.prod.internal:27018",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := &MongoConfig{
				Host: tc.host,
				Port: tc.port,
			}
			assert.Equal(t, tc.wantURI, cfg.URI())
		})
	}
}

// ---------------------------------------------------------------------------
// Spec: host property default 'localhost'
// ---------------------------------------------------------------------------

func TestSpec_Host_DefaultIsLocalhost(t *testing.T) {
	cfg, err := LoadMongoConfig()
	require.NoError(t, err)
	assert.Equal(t, "localhost", cfg.Host,
		"spec: host property must return 'localhost' as the MongoDB host at startup")
	assert.NotEmpty(t, cfg.Host,
		"spec invariant: host must be a non-empty string")
}

// ---------------------------------------------------------------------------
// Spec: port property default 27017
// ---------------------------------------------------------------------------

func TestSpec_Port_DefaultIs27017(t *testing.T) {
	cfg, err := LoadMongoConfig()
	require.NoError(t, err)
	assert.Equal(t, 27017, cfg.Port,
		"spec: port property must return 27017 as the MongoDB port at startup")
	assert.Greater(t, cfg.Port, 0,
		"spec invariant: port must be within TCP range (> 0)")
	assert.LessOrEqual(t, cfg.Port, 65535,
		"spec invariant: port must be within TCP range (<= 65535)")
}

// ---------------------------------------------------------------------------
// Spec: sundarDB property default 'sundar'
// ---------------------------------------------------------------------------

func TestSpec_SundarDB_DefaultIsSundar(t *testing.T) {
	cfg, err := LoadMongoConfig()
	require.NoError(t, err)
	assert.Equal(t, "sundar", cfg.Database,
		"spec: sundarDB property must return 'sundar' as the target database name")
	assert.NotEmpty(t, cfg.Database,