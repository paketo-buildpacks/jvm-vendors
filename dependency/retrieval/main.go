// Copyright 2018-2026 the original author or authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/paketo-buildpacks/packit/v2/cargo"
)

type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

var supportedStacks = []string{"*"}

var supportedPlatforms = map[string][]string{
	"linux": {"amd64", "arm64"},
}

type PlatformStackTarget struct {
	stacks []string
	target string
	os     string
	arch   string
}

func getSupportedPlatformStackTargets() []PlatformStackTarget {
	var platformStackTargets []PlatformStackTarget

	for os, architectures := range supportedPlatforms {
		for _, arch := range architectures {
			target := fmt.Sprintf("%s-%s", os, arch)
			platformStackTargets = append(platformStackTargets, PlatformStackTarget{
				stacks: supportedStacks,
				target: target,
				os:     os,
				arch:   arch,
			})
		}
	}

	return platformStackTargets
}

// OutputMetadata represents the output JSON structure
type OutputMetadata struct {
	ID             string `json:"id"`
	Version        string `json:"version"`
	URI            string `json:"uri"`
	Checksum       string `json:"checksum"`
	Source         string `json:"source,omitempty"`
	SourceChecksum string `json:"source-checksum,omitempty"`
	Target         string `json:"target"`
	OS             string `json:"os"`
	Arch           string `json:"arch"`
	CPE            string `json:"cpe,omitempty"`
	PURL           string `json:"purl,omitempty"`
}

// Dependency represents a dependency entry
type Dependency struct {
	cargo.ConfigMetadataDependency
	Target string `json:"target"`
}

func main() {
	var buildpackTomlPath string
	var outputPath string
	var filterIDs stringSliceFlag
	flag.StringVar(&buildpackTomlPath, "buildpack-toml-path", "", "Path to buildpack.toml")
	flag.StringVar(&outputPath, "output", "", "Path to output metadata.json")
	flag.Var(&filterIDs, "filter-ids", "Filter to only process these dependency IDs (can be specified multiple times)")
	flag.Parse()

	if buildpackTomlPath == "" || outputPath == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s --buildpack-toml-path <path> --output <path>\n", os.Args[0])
		os.Exit(1)
	}

	// Load buildpack.toml
	file, err := os.Open(buildpackTomlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening buildpack.toml: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	var config cargo.Config
	if _, err := toml.NewDecoder(file).Decode(&config); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing buildpack.toml: %v\n", err)
		os.Exit(1)
	}

	// Use existing dependencies from buildpack.toml for caching
	existingDeps := config.Metadata.Dependencies
	fmt.Printf("Loaded %d existing dependency entries from buildpack.toml for caching\n", len(existingDeps))

	// Get all unique constraint IDs
	ids := getUniqueConstraintIDs(config.Metadata.DependencyConstraints)

	// Filter IDs if --filter-ids was specified
	if len(filterIDs) > 0 {
		ids = slices.DeleteFunc(ids, func(id string) bool {
			return !slices.Contains(filterIDs, id)
		})
	}

	// Collect all dependencies
	var allDependencies []Dependency

	fmt.Printf("Retrieving dependencies for %d unique constraint IDs...\n", len(ids))

	for i, id := range ids {
		constraints := getConstraintsForID(config.Metadata.DependencyConstraints, id)

		if len(constraints) == 0 {
			fmt.Printf("[%d/%d] No constraints found for %s, skipping\n", i+1, len(ids), id)
			continue
		}

		fmt.Printf("[%d/%d] Retrieving %s (%d constraints)...\n", i+1, len(ids), id, len(constraints))

		deps, err := retrieveAndGenerate(id, constraints, existingDeps)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: error retrieving versions for %s: %v\n", id, err)
			continue
		}

		fmt.Printf("[%d/%d] Retrieved %d entries for %s\n", i+1, len(ids), len(deps), id)

		allDependencies = append(allDependencies, deps...)
	}

	// Convert to output format
	output := convertToOutputFormat(allDependencies)

	// Write output
	outFile, err := os.Create(outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer outFile.Close()

	encoder := json.NewEncoder(outFile)
	encoder.SetIndent("", "  ")
	if err = encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding output: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully wrote %d dependency entries to %s\n", len(output), outputPath)
}

func getUniqueConstraintIDs(constraints []cargo.ConfigMetadataDependencyConstraint) []string {
	seen := make(map[string]bool)
	var ids []string
	for _, c := range constraints {
		if !seen[c.ID] {
			seen[c.ID] = true
			ids = append(ids, c.ID)
		}
	}
	return ids
}

func getConstraintsForID(constraints []cargo.ConfigMetadataDependencyConstraint, id string) []cargo.ConfigMetadataDependencyConstraint {
	var result []cargo.ConfigMetadataDependencyConstraint
	for _, c := range constraints {
		if c.ID == id {
			result = append(result, c)
		}
	}
	return result
}

func retrieveAndGenerate(id string, constraints []cargo.ConfigMetadataDependencyConstraint, existing []cargo.ConfigMetadataDependency) ([]Dependency, error) {
	var allDeps []Dependency

	for _, constraint := range constraints {
		fmt.Printf("  Processing constraint %s...\n", constraint.Constraint)

		deps, err := generateForConstraint(id, constraint, existing)
		if err != nil {
			return nil, fmt.Errorf("failed to generate for constraint %s: %w", constraint.Constraint, err)
		}
		allDeps = append(allDeps, deps...)
	}

	return allDeps, nil
}

func generateForConstraint(id string, constraint cargo.ConfigMetadataDependencyConstraint, existing []cargo.ConfigMetadataDependency) ([]Dependency, error) {
	switch id {
	case "jdk-adoptium", "jre-adoptium":
		return generateAdoptium(id, constraint, existing)
	case "jdk-microsoft-openjdk":
		return generateMicrosoft(id, constraint, existing)
	case "jdk-azul-zulu", "jre-azul-zulu":
		return generateZulu(id, constraint, existing)
	case "jdk-bellsoft-liberica", "jre-bellsoft-liberica", "native-image-svm-bellsoft-liberica":
		return generateBellsoft(id, constraint, existing)
	case "jdk-amazon-corretto":
		return generateCorretto(id, constraint, existing)
	case "jdk-alibaba-dragonwell":
		return generateDragonwell(id, constraint, existing)
	case "jdk-graalvm", "native-image-svm-graalvm":
		return generateGraalVM(id, constraint, existing)
	case "jdk-eclipse-openj9", "jre-eclipse-openj9":
		return generateOpenJ9(id, constraint, existing)
	case "jdk-oracle", "native-image-svm-oracle":
		return generateOracle(id, constraint, existing)
	case "jdk-sap-machine", "jre-sap-machine":
		return generateSapMachine(id, constraint, existing)
	default:
		if _, ok := foojayDistroMap[id]; ok {
			return generateFoojay(id, constraint, existing)
		}
		return nil, fmt.Errorf("unknown dependency ID: %s", id)
	}
}

func findExistingDependency(existing []cargo.ConfigMetadataDependency, id, uri string) *cargo.ConfigMetadataDependency {
	for _, dep := range existing {
		if dep.ID == id && dep.URI == uri {
			return &dep
		}
	}
	return nil
}

func convertToOutputFormat(deps []Dependency) []OutputMetadata {
	var output []OutputMetadata
	for _, dep := range deps {
		os, arch := parseTarget(dep.Target)

		output = append(output, OutputMetadata{
			ID:             dep.ID,
			Version:        dep.Version,
			URI:            dep.URI,
			Checksum:       dep.SHA256,
			Source:         dep.Source,
			SourceChecksum: dep.SourceSHA256,
			Target:         dep.Target,
			OS:             os,
			Arch:           arch,
			CPE:            dep.CPE,
			PURL:           dep.PURL,
		})
	}
	return output
}

func parseTarget(target string) (os, arch string) {
	switch target {
	case "linux-amd64":
		return "linux", "amd64"
	case "linux-arm64":
		return "linux", "arm64"
	}
	return "linux", "amd64"
}

func createDependency(dep cargo.ConfigMetadataDependency, targetName string) Dependency {
	return Dependency{
		ConfigMetadataDependency: dep,
		Target:                   targetName,
	}
}

func dependencyFromExisting(existing *cargo.ConfigMetadataDependency, os, arch string) Dependency {
	return Dependency{
		ConfigMetadataDependency: cargo.ConfigMetadataDependency{
			ID:           existing.ID,
			Name:         existing.Name,
			Version:      existing.Version,
			URI:          existing.URI,
			SHA256:       existing.SHA256,
			Source:       existing.Source,
			SourceSHA256: existing.SourceSHA256,
			Stacks:       existing.Stacks,
			OS:           os,
			Arch:         arch,
			CPE:          existing.CPE,
			PURL:         existing.PURL,
			Licenses:     existing.Licenses,
		},
		Target: fmt.Sprintf("%s-%s", os, arch),
	}
}
