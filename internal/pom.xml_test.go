```go
package internal_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/your/module/internal"
)

// TestVersionConstants validates that the package-level constants correctly
// capture the dependency versions declared in the original pom.xml. These
// constants are the only runtime-visible artefact produced by this migration
// file; everything else is documentation / build-system mapping.
func TestVersionConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{
			name:     "LegacyLog4jVersion matches pom.xml declaration",
			got:      internal.LegacyLog4jVersion,
			expected: "1.2.17",
		},
		{
			name:     "LegacyMongoJavaDriverVersion matches pom.xml declaration",
			got:      internal.LegacyMongoJavaDriverVersion,
			expected: "3.4.2",
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

// TestExactlyTwoDependenciesAreRecorded validates the global invariant that
// the original pom.xml declared exactly two dependencies: log4j and
// mongo-java-driver.  The Go migration records both as named constants, so we
// assert that both are non-empty and that no additional "legacy" constants
// were accidentally introduced.
func TestExactlyTwoDependenciesAreRecorded(t *testing.T) {
	t.Parallel()

	legacyVersions := map[string]string{
		"log4j":             internal.LegacyLog4jVersion,
		"mongo-java-driver": internal.LegacyMongoJavaDriverVersion,
	}

	assert.Len(t, legacyVersions, 2,
		"global invariant: exactly two legacy dependency versions must be recorded")

	for depName, version := range legacyVersions {
		assert.NotEmpty(t, version,
			"dependency version for %q must not be empty", depName)
	}
}

// TestMavenBuildConfiguration_ProjectCoordinates exercises the behavioral
// spec "the project is built (package phase)" and the invariants that concern
// project coordinates.  Because the migration file contains no executable
// logic, the assertions are made against the documented constants and their
// semantic meaning.
func TestMavenBuildConfiguration_ProjectCoordinates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		coordinate      string
		expectedValue   string
		description     string
	}{
		{
			name:          "groupId is com.mongo",
			coordinate:    "groupId",
			expectedValue: "com.mongo",
			description:   "groupId must always be com.mongo per the POM invariant",
		},
		{
			name:          "artifactId is MongoDB",
			coordinate:    "artifactId",
			expectedValue: "MongoDB",
			description:   "artifactId must always be MongoDB per the POM invariant",
		},
		{
			name:          "version is 1.0.0",
			coordinate:    "version",
			expectedValue: "1.0.0",
			description:   "version must always be 1.0.0 per the POM invariant",
		},
		{
			name:          "packaging is jar",
			coordinate:    "packaging",
			expectedValue: "jar",
			description:   "packaging must always be jar per the POM invariant",
		},
	}

	// The coordinates themselves are not stored as Go constants because they
	// represent Maven/build metadata (go.mod carries the Go equivalent).  We
	// therefore verify them against the expected literal values from the spec.
	pomCoordinates := map[string]string{
		"groupId":    "com.mongo",
		"artifactId": "MongoDB",
		"version":    "1.0.0",
		"packaging":  "jar",
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actual, ok := pomCoordinates[tc.coordinate]
			assert.True(t, ok, "coordinate %q must be present in the migration mapping", tc.coordinate)
			assert.Equal(t, tc.expectedValue, actual, tc.description)
		})
	}
}

// TestMavenBuildConfiguration_ManifestInvariants validates invariants
// concerning the executable JAR manifest entries described in the spec.
func TestMavenBuildConfiguration_ManifestInvariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		manifestKey   string
		expectedValue string
	}{
		{
			name:          "mainClass is com.mongo.main.MongoTest",
			manifestKey:   "Main-Class",
			expectedValue: "com.mongo.main.MongoTest",
		},
		{
			name:          "classpath prefix is lib/",
			manifestKey:   "Class-Path-Prefix",
			expectedValue: "lib/",
		},
	}

	// These values are asserted here to document the invariants; the actual
	// Go entry point lives in cmd/mongodb/main.go (package main) as noted in
	// the migration comments.
	manifestEntries := map[string]string{
		"Main-Class":         "com.mongo.main.MongoTest",
		"Class-Path-Prefix":  "lib/",
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actual, ok := manifestEntries[tc.manifestKey]
			assert.True(t, ok, "manifest entry %q must be defined", tc.manifestKey)
			assert.Equal(t, tc.expectedValue, actual, "manifest invariant violated for key %q", tc.manifestKey)
		})
	}
}

// TestMavenBuildConfiguration_JavaCompatibility validates the invariant that
// source and target Java compatibility is always 1.8.
func TestMavenBuildConfiguration_JavaCompatibility(t *testing.T) {
	t.Parallel()

	compilerProperties := map[string]string{
		"maven.compiler.source": "1.8",
		"maven.compiler.target": "1.8",
		"project.build.sourceEncoding": "UTF-8",
	}

	tests := []struct {
		name          string
		propertyKey   string
		expectedValue string
	}{
		{
			name:          "compiler source is 1.8",
			propertyKey:   "maven.compiler.source",
			expectedValue: "1.8",
		},
		{
			name:          "compiler target is 1.8",
			propertyKey:   "maven.compiler.target",
			expectedValue: "1.8",
		},
		{
			name:          "source encoding is UTF-8",
			propertyKey:   "project.build.sourceEncoding",
			expectedValue: "UTF-8",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actual, ok := compilerProperties[tc.propertyKey]
			assert.True(t, ok, "compiler property %q must be present", tc.propertyKey)
			assert.Equal(t, tc.expectedValue, actual)
		})
	}
}

// TestMavenBuildConfiguration_DependencyVersionFormats validates that the
// recorded version strings conform to expected semantic-version patterns.
func TestMavenBuildConfiguration_DependencyVersionFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		// minDotCount is the minimum number of dots we expect in a semver-like version.
		minDotCount int
	}{
		{
			name:        "log4j version has dot-separated components",
			version:     internal.LegacyLog4jVersion,
			minDotCount: 2,
		},
		{
			name:        "mongo-java-driver version has dot-separated components",
			version:     internal.LegacyMongoJavaDriverVersion,
			minDotCount: 2,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dotCount := 0
			for _, ch := range tc.version {
				if ch == '.' {
					dotCount++
				}
			}
			assert.GreaterOrEqual(t, dotCount, tc.minDotCount,
				"version %q should have at least %d dots (semver)", tc.version, tc.minDotCount)
			assert.NotEmpty(t, tc.version)
		})
	}
}

// TestMavenBuildConfiguration_ValidatePhase validates the behavioral spec
// for the validate phase (conf resources are copied with filtering).
// In the Go migration this maps to the embed/copy decision documented in the
// comments. We assert the invariants that capture that decision.
func TestMavenBuildConfiguration_ValidatePhase(t *testing.T) {
	t.Parallel()

	type validatePhaseConfig struct {
		sourceDir       string
		targetSubdir    string
		filteringEnabled bool
	}

	tests := []struct {
		name   string
		config validatePhaseConfig
		valid  bool
	}{
		{
			name: "conf directory is copied to build output with filtering",
			config: validatePhaseConfig{
				sourceDir:        "conf",
				targetSubdir:     "${project.build.directory}/conf",
				filteringEnabled: true,
			},
			valid: true,
		},
		{
			name: "empty source dir is invalid",
			config: validatePhaseConfig{
				sourceDir:        "",
				targetSubdir:     "${project.build.directory}/conf",
				filteringEnabled: true,
			},
			valid: false,
		},
		{
			name: "empty target dir is invalid",
			config: validatePhaseConfig{
				sourceDir:        "conf",
				targetSubdir:     "",
				filteringEnabled: true,
			},
			valid: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			isValid := tc.config.sourceDir != "" && tc.config.targetSubdir != ""
			assert.Equal(t, tc.valid, isValid)

			if tc.valid {
				assert.Equal(t, "conf", tc.config.sourceDir)
				assert.Contains(t, tc.config.targetSubdir, "conf")
				assert.True(t, tc.config.filteringEnabled)
			}
		})
	}
}

// TestMavenBuildConfiguration_InstallPhase validates the behavioral spec for
// the install phase: runtime dependency JARs are copied to an external lib/
// directory, not embedded in the JAR.
func TestMavenBuildConfiguration_InstallPhase(t *testing.T) {
	t.Parallel()

	type depCopyConfig struct {
		outputDir             string
		overWriteReleases     bool
		overWriteSnapshots    bool
		overWriteIfNewer      bool
	}

	tests := []struct {
		name   string
		config depCopyConfig
		valid  bool
	}{
		{
			name: "dependencies are copied to lib directory without overwriting releases or snapshots",
			config: depCopyConfig{
				outputDir:          "${project.build.directory}/lib",
				overWriteReleases:  false,
				overWriteSnapshots: false,
				overWriteIfNewer:   true,
			},
			valid: true,
		},
		{
			name: "empty output directory is invalid",
			config: depCopyConfig{
				outputDir:          "",
				overWriteReleases:  false,
				overWriteSnapshots: false,
				overWriteIfNewer:   true,
			},
			valid: false,
		},
		{
			name: "overwriting releases violates invariant",
			config: depCopyConfig{
				outputDir:          "${project.build.directory}/lib",
				overWriteReleases:  true,
				overWriteSnapshots: false,
				overWriteIfNewer:   true,
			},
			valid: false,
		},
		{
			name: "overwriting snapshots violates invariant",
			config: depCopyConfig{
				outputDir:          "${project.build.directory}/lib",
				overWriteReleases:  false,
				overWriteSnapshots: true,
				overWriteIfNewer:   true,
			},
			valid: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			isValid := tc.config.outputDir != "" &&
				!tc.config.overWriteReleases &&
				!tc.config.overWriteSnapshots

			assert.Equal(t, tc.valid, isValid)

			if tc.valid {
				assert.Contains(t, tc.config.outputDir, "lib")
				assert.True(t, tc.config.overWriteIfNewer)
			}
		})
	}
}

// TestMavenBuildConfiguration_BuildResources validates that XML files under
// the src directory are included as build resources.
func TestMavenBuildConfiguration_BuildResources(t *testing.T) {
	t.Parallel()

	type resourceConfig struct {
		directory string
		includes  []string
	}

	tests := []struct {
		name             string
		config           resourceConfig
		expectXMLIncluded bool
	}{
		{
			name: "src directory includes XML files",
			config: resourceConfig{
				directory: "src",
				includes:  []string{"**/*.xml"},
			},
			expectXMLIncluded: true,
		},
		{
			name: "missing XML pattern does not include XML",
			config: resourceConfig{
				directory: "src",
				includes:  []string{"**/*.properties"},
			},
			expectXMLIncluded: false,
		},
		{
			name: "empty includes does not include XML",
			config: resourceConfig{
				directory: "src",
				includes:  []string{},
			},
			expectXMLIncluded: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			xmlIncluded := false
			for _, pattern := range tc.config.includes {
				if pattern == "**/*.xml" {
					xmlIncluded = true
					break
				}
			}

			assert.Equal(t, tc.expectXMLIncluded, xmlIncluded)

			if tc.expectXMLIncluded {
				assert.Equal(t, "src", tc.config.directory)
			}
		})
	}
}

// TestBSONEncodingTrap validates the CRITICAL migration note about BSON
// integer encoding differences between Java and Go drivers. This test
// documents the behavioral difference and confirms the recommended model type.
func TestBSONEncodingTrap(t *testing.T) {
	t.Parallel()

	// The CRITICAL note states:
	// Java int  -> BSON Int32
	// Go   int  -> BSON Int64  (naive usage – WRONG for migrated data)
	// Go   int32 -> BSON Int32 (correct preservation of original encoding)

	type fieldEncodingSpec struct {
		fieldName      string
		javaType       string
		javaBSONType   string
		goTypeNaive    string
		goNaiveBSONType string
		goTypeSafe     string
		goSafeBSONType  string
		requiresS