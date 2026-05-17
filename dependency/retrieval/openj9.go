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
	"strings"

	"github.com/paketo-buildpacks/packit/v2/cargo"
)

var semeruRepoMap = map[int]string{
	8:  "semeru8-binaries",
	11: "semeru11-binaries",
	17: "semeru17-binaries",
	21: "semeru21-binaries",
	25: "semeru25-binaries",
	26: "semeru26-binaries",
}

func generateOpenJ9(id string, constraint cargo.ConfigMetadataDependencyConstraint, existing []cargo.ConfigMetadataDependency) ([]Dependency, error) {
	imageType := "jdk"
	if id == "jre-eclipse-openj9" {
		imageType = "jre"
	}

	majorVersion, err := extractVersionFromConstraint(constraint.Constraint)
	if err != nil {
		return nil, fmt.Errorf("unable to extract version from constraint %s: %w", constraint.Constraint, err)
	}

	repo, ok := semeruRepoMap[majorVersion]
	if !ok {
		return nil, fmt.Errorf("unsupported Semeru major version: %d", majorVersion)
	}

	release, err := fetchLatestRelease("ibmruntimes", repo)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Semeru release for %s: %w", repo, err)
	}

	javaVersion := extractSemeruJavaVersion(release.TagName, majorVersion)
	if javaVersion == "" {
		return nil, fmt.Errorf("unable to extract Java version from tag %s", release.TagName)
	}

	var dependencies []Dependency

	for _, pt := range getSupportedPlatformStackTargets() {
		archPattern := "x64"
		if pt.arch == "arm64" {
			archPattern = "aarch64"
		}

		assetURL := findSemeruAsset(release.Assets, imageType, archPattern)
		if assetURL == "" {
			fmt.Printf("Warning: no Semeru asset found for %s %s %s\n", id, javaVersion, pt.target)
			continue
		}

		if existingDep := findExistingDependency(existing, id, assetURL); existingDep != nil {
			fmt.Printf("  Using cached metadata for %s %s %s\n", id, javaVersion, pt.target)
			d := dependencyFromExisting(existingDep, pt.os, pt.arch)
			dependencies = append(dependencies, d)
			continue
		}

		checksum, err := downloadAndCalculateSHA256(assetURL)
		if err != nil {
			fmt.Printf("Warning: failed to calculate checksum for %s %s %s: %v\n", id, javaVersion, pt.target, err)
			continue
		}

		sourceURL := release.TarballURL
		sourceChecksum := ""
		if sourceURL != "" {
			sc, err := downloadAndCalculateSHA256(sourceURL)
			if err != nil {
				fmt.Printf("Warning: failed to calculate source checksum for %s %s: %v\n", id, javaVersion, err)
			} else {
				sourceChecksum = sc
			}
		}

		purl := fmt.Sprintf("pkg:generic/ibmruntimes/semeru-%s@%s?arch=%s", imageType, javaVersion, pt.arch)

		cpe := generateOracleCPE(javaVersion)

		name := "Eclipse OpenJ9 " + strings.ToUpper(imageType)

		dep := cargo.ConfigMetadataDependency{
			ID:           id,
			Name:         name,
			Version:      javaVersion,
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

func findSemeruAsset(assets []GitHubAsset, imageType, archPattern string) string {
	for _, asset := range assets {
		if strings.Contains(asset.Name, imageType) &&
			strings.Contains(asset.Name, archPattern) &&
			strings.Contains(asset.Name, "_linux_") &&
			strings.HasSuffix(asset.Name, ".tar.gz") {
			return asset.BrowserDownloadURL
		}
	}
	return ""
}

func extractSemeruJavaVersion(tagName string, majorVersion int) string {
	if majorVersion == 8 {
		if strings.HasPrefix(tagName, "jdk-") {
			version := strings.TrimPrefix(tagName, "jdk-")
			parts := strings.Split(version, ".")
			if len(parts) >= 3 {
				return fmt.Sprintf("8.0.%s", parts[2])
			}
		}
	} else {
		if strings.HasPrefix(tagName, "jdk-") {
			version := strings.TrimPrefix(tagName, "jdk-")
			parts := strings.Split(version, ".")
			if len(parts) >= 3 {
				return fmt.Sprintf("%s.%s.%s", parts[0], parts[1], parts[2])
			}
		}
	}
	return ""
}
