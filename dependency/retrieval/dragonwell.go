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
	"fmt"
	"regexp"
	"strings"

	"github.com/paketo-buildpacks/packit/v2/cargo"
)

var dragonwellRepoMap = map[int]string{
	8:  "dragonwell8",
	11: "dragonwell11",
	17: "dragonwell17",
	21: "dragonwell21",
	25: "dragonwell25",
}

func generateDragonwell(id string, constraint cargo.ConfigMetadataDependencyConstraint, existing []cargo.ConfigMetadataDependency) ([]Dependency, error) {
	majorVersion, err := extractVersionFromConstraint(constraint.Constraint)
	if err != nil {
		return nil, fmt.Errorf("unable to extract version from constraint %s: %w", constraint.Constraint, err)
	}

	repo, ok := dragonwellRepoMap[majorVersion]
	if !ok {
		return nil, fmt.Errorf("unsupported Dragonwell major version: %d", majorVersion)
	}

	release, err := fetchLatestRelease("dragonwell-project", repo)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Dragonwell release for %s: %w", repo, err)
	}

	javaVersion := extractDragonwellJavaVersion(release.TagName, majorVersion)

	// keep the Dragonwell build (e.g. 21.0.12.0.12+8 -> 21.0.12+0.12.8) so that
	// a rebuild of the same Java version is not treated as a duplicate
	version := dragonwellVersion(release.TagName, majorVersion, javaVersion)

	sourceURL := release.TarballURL
	sourceChecksum := getSourceChecksum(sourceURL, existing)

	var dependencies []Dependency

	for _, pt := range getSupportedPlatformStackTargets() {
		archPattern := "x64"
		if pt.arch == "arm64" {
			archPattern = "aarch64"
		}

		assetURL := findDragonwellAsset(release.Assets, archPattern)
		if assetURL == "" {
			fmt.Printf("Warning: no Dragonwell asset found for %s %s %s\n", id, version, pt.target)
			continue
		}

		if existingDep := findExistingDependency(existing, id, assetURL); existingDep != nil {
			fmt.Printf("  Using cached metadata for %s %s %s\n", id, version, pt.target)
			d := dependencyFromExisting(existingDep, version, pt.os, pt.arch)
			dependencies = append(dependencies, d)
			continue
		}

		checksum, err := downloadAndCalculateSHA256(assetURL)
		if err != nil {
			fmt.Printf("Warning: failed to calculate checksum for %s %s %s: %v\n", id, version, pt.target, err)
			continue
		}

		purl := fmt.Sprintf("pkg:generic/alibaba/dragonwell-jdk@%s?arch=%s", javaVersion, pt.arch)

		cpe := generateOracleCPE(javaVersion)

		name := "Alibaba Dragonwell JDK"

		dep := cargo.ConfigMetadataDependency{
			ID:           id,
			Name:         name,
			Version:      version,
			URI:          assetURL,
			SHA256:       checksum,
			Source:       sourceURL,
			SourceSHA256: sourceChecksum,
			Stacks:       pt.stacks,
			OS:           pt.os,
			Arch:         pt.arch,
			CPE:          cpe,
			PURL:         purl,
			Licenses:     getLicenses(cargo.ConfigMetadataDependency{}),
		}

		d := createDependency(dep, pt.target)
		dependencies = append(dependencies, d)
	}

	return dependencies, nil
}

func findDragonwellAsset(assets []GitHubAsset, archPattern string) string {
	for _, asset := range assets {
		if strings.Contains(asset.Name, archPattern) &&
			strings.Contains(asset.Name, "_linux") &&
			strings.HasSuffix(asset.Name, ".tar.gz") &&
			!strings.HasSuffix(asset.Name, ".tar.gz.sig") &&
			!strings.HasSuffix(asset.Name, ".tar.gz.json") &&
			!strings.HasSuffix(asset.Name, ".tar.gz.sha256.txt") {
			return asset.BrowserDownloadURL
		}
	}
	return ""
}

func extractDragonwellJavaVersion(tagName string, majorVersion int) string {
	if majorVersion == 8 {
		re := regexp.MustCompile(`_jdk8u(\d+)-ga`)
		matches := re.FindStringSubmatch(tagName)
		if len(matches) > 1 {
			return fmt.Sprintf("8.0.%s", matches[1])
		}
	} else {
		re := regexp.MustCompile(`_jdk-(\d+\.\d+\.\d+)-ga`)
		matches := re.FindStringSubmatch(tagName)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	return tagName
}

// dragonwellVersionPattern captures the Dragonwell release version, which sits
// between the release edition and the Java version, e.g.
// "dragonwell-standard-21.0.12.0.12+8_jdk-21.0.12-ga" -> "21.0.12.0.12+8".
var dragonwellVersionPattern = regexp.MustCompile(`^dragonwell-[a-z]+-([^_]+)_jdk`)

func dragonwellVersion(tagName string, majorVersion int, javaVersion string) string {
	matches := dragonwellVersionPattern.FindStringSubmatch(tagName)
	if len(matches) < 2 {
		return javaVersion
	}

	// Dragonwell tracks Java 8 with a release line (e.g. 8.30.29) rather than a
	// build number, so keep the Java version as the core and record the release
	// line as build metadata: 8.30.29 + 8u502 -> 8.0.502+30.29
	if majorVersion == 8 {
		return withBuildMetadata(javaVersion, strings.TrimPrefix(matches[1], "8."))
	}

	return vendorVersionToSemver(matches[1])
}
