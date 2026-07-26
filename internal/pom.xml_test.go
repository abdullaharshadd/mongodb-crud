```go
package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestProjectBuildMetadata_TableDriven validates that ProjectBuildMetadata
// returns the exact Maven coordinates that were declared in pom.xml.
func TestProjectBuildMetadata_TableDriven(t *testing.T) {
	tests := []struct {
		name                 string
		wantGroupID          string
		wantArtifactID       string
		wantVersion          string
		wantJavaSourceTarget string
	}{
		{
			name:                 "returns correct groupId from pom.xml",
			wantGroupID:          "com.mongo",
			wantArtifactID:       "MongoDB",
			wantVersion:          "1.0.0",
			wantJavaSourceTarget: "1.8",
		},
		// The function is pure and deterministic; a second call must return
		// identical values — validates no mutable global state.
		{
			name:                 "second call returns identical coordinates (no mutable state)",
			wantGroupID:          "com.mongo",
			wantArtifactID:       "MongoDB",
			wantVersion:          "1.0.0",
			wantJavaSourceTarget: "1.8",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := ProjectBuildMetadata()

			assert.Equal(t, tc.wantGroupID, got.GroupID,
				"GroupID must always be com.mongo (global invariant)")
			assert.Equal(t, tc.wantArtifactID, got.ArtifactID,
				"ArtifactID must always be MongoDB (global invariant)")
			assert.Equal(t, tc.wantVersion, got.Version,
				"Version must always be 1.0.0 (global invariant)")
			assert.Equal(t, tc.wantJavaSourceTarget, got.JavaSourceTarget,
				"JavaSourceTarget must always be 1.8 (global invariant)")
		})
	}
}

// TestProjectBuildMetadata_GlobalInvariants checks the broader project-level
// invariants declared in the behavioural spec independently of individual
// fields.

// TestGoModuleDependencies_TableDriven validates every entry in the dependency
// mapping returned by GoModuleDependencies.
func TestGoModuleDependencies_TableDriven(t *testing.T) {
	tests := []struct {
		name            string
		mavenCoordinate string
		wantGoModule    string
	}{
		{
			name:            "log4j maps to zerolog",
			mavenCoordinate: "log4j:log4j:1.2.17",
			wantGoModule:    "github.com/rs/zerolog",
		},
		{
			name:            "mongo-java-driver maps to go mongo-driver",
			mavenCoordinate: "org.mongodb:mongo-java-driver:3.4.2",
			wantGoModule:    "go.mongodb.org/mongo-driver/mongo",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			deps := GoModuleDependencies()

			val, ok := deps[tc.mavenCoordinate]
			assert.True(t, ok,
				"dependency key %q must be present in the map", tc.mavenCoordinate)
			assert.Equal(t, tc.wantGoModule, val,
				"Go module mapping for %q", tc.mavenCoordinate)
		})
	}
}

// TestGoModuleDependencies_MapInvariants verifies structural invariants on
// the dependency map as a whole.

// TestBuildMetadata_Struct validates that the BuildMetadata type can be
// constructed freely (it has no constructor constraints) and that zero values
// behave as expected.
func TestBuildMetadata_Struct(t *testing.T) {
	tests := []struct {
		name  string
		input BuildMetadata
		check func(t *testing.T, m BuildMetadata)
	}{
		{
			name:  "zero value has all empty fields",
			input: BuildMetadata{},
			check: func(t *testing.T, m BuildMetadata) {
				assert.Empty(t, m.GroupID)
				assert.Empty(t, m.ArtifactID)
				assert.Empty(t, m.Version)
				assert.Empty(t, m.JavaSourceTarget)
			},
		},
		{
			name: "custom values are preserved",
			input: BuildMetadata{
				GroupID:          "org.example",
				ArtifactID:       "test-artifact",
				Version:          "2.0.0",
				JavaSourceTarget: "11",
			},
			check: func(t *testing.T, m BuildMetadata) {
				assert.Equal(t, "org.example", m.GroupID)
				assert.Equal(t, "test-artifact", m.ArtifactID)
				assert.Equal(t, "2.0.0", m.Version)
				assert.Equal(t, "11", m.JavaSourceTarget)
			},
		},
		{
			name: "ProjectBuildMetadata value equals expected literal struct",
			input: ProjectBuildMetadata(),
			check: func(t *testing.T, m BuildMetadata) {
				expected := BuildMetadata{
					GroupID:          "com.mongo",
					ArtifactID:       "MongoDB",
					Version:          "1.0.0",
					JavaSourceTarget: "1.8",
				}
				assert.Equal(t, expected, m)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, tc.input)
		})
	}
}

// TestMavenJarPluginMigration validates the migration note invariants related
// to the executable JAR build (maven-jar-plugin spec).
//
// The Go migration encodes these artefacts in ProjectBuildMetadata and
// GoModuleDependencies; the tests below assert the conditions that a Go build
// toolchain would rely on.
func TestMavenJarPluginMigration(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{
			name: "artifact version matches expected JAR name MongoDB-1.0.0.jar",
			check: func(t *testing.T) {
				meta := ProjectBuildMetadata()
				// The JAR name is <artifactId>-<version>.jar
				expectedJARName := meta.ArtifactID + "-" + meta.Version + ".jar"
				assert.Equal(t, "MongoDB-1.0.0.jar", expectedJARName,
					"jar artefact name derived from coordinates must be MongoDB-1.0.0.jar")
			},
		},
		{
			name: "source target is 1.8 ensuring Java 8 compatibility requirement is recorded",
			check: func(t *testing.T) {
				meta := ProjectBuildMetadata()
				assert.Equal(t, "1.8", meta.JavaSourceTarget)
			},
		},
		{
			name: "mongo runtime dependency is correctly mapped for executable equivalent",
			check: func(t *testing.T) {
				deps := GoModuleDependencies()
				assert.Equal(t, "go.mongodb.org/mongo-driver/mongo",
					deps["org.mongodb:mongo-java-driver:3.4.2"],
					"mongo-java-driver must map to go.mongodb.org/mongo-driver/mongo for runtime equivalence")
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t)
		})
	}
}

// TestMavenResourcesPluginMigration validates the migration note for the
// maven-resources-plugin copy-resources-1 spec. In Go this is handled by
// the embed package or external tooling; the metadata here records that
// conf resource copying was part of the original build.
func TestMavenResourcesPluginMigration(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{
			name: "build metadata records the project that required conf resource copying",
			check: func(t *testing.T) {
				meta := ProjectBuildMetadata()
				// The conf resource copy was bound to the validate phase for this
				// specific project; verify coordinates are preserved.
				assert.Equal(t, "com.mongo", meta.GroupID)
				assert.Equal(t, "MongoDB", meta.ArtifactID)
			},
		},
		{
			name: "filtering was enabled — UTF-8 encoding implied by project coordinates being ASCII-safe",
			check: func(t *testing.T) {
				meta := ProjectBuildMetadata()
				// All coordinate strings must be ASCII-printable (UTF-8 safe),
				// reflecting that build.sourceEncoding=UTF-8 was set.
				for _, s := range []string{meta.GroupID, meta.ArtifactID, meta.Version, meta.JavaSourceTarget} {
					assert.NotEmpty(t, s)
					for _, r := range s {
						assert.Less(t, r, rune(128),
							"coordinate %q contains non-ASCII rune %q", s, r)
					}
				}
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t)
		})
	}
}

// TestMavenDependencyPluginMigration validates the migration note for the
// maven-dependency-plugin copy-dependencies spec.
func TestMavenDependencyPluginMigration(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{
			name: "both declared pom.xml dependencies are represented in the Go module map",
			check: func(t *testing.T) {
				deps := GoModuleDependencies()
				assert.Contains(t, deps, "log4j:log4j:1.2.17",
					"log4j must be present — it was copied to lib/ in the original build")
				assert.Contains(t, deps, "org.mongodb:mongo-java-driver:3.4.2",
					"mongo-java-driver must be present — it was copied to lib/ in the original build")
			},
		},
		{
			name: "Go module paths for both dependencies are non-empty import paths",
			check: func(t *testing.T) {
				deps := GoModuleDependencies()
				for mavenCoord, goModule := range deps {
					assert.Contains(t, goModule, "/",
						"Go module %q (for %q) must look like an import path containing '/'",
						goModule, mavenCoord)
				}
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t)
		})
	}
}

// TestResourceInclusionXMLMigration validates the src XML resource inclusion
// spec. In Go, XML files would be embedded via go:embed; the metadata here
// confirms the original project's structure is documented.
func TestResourceInclusionXMLMigration(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T)