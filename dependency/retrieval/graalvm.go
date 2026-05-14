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

func generateGraalVM(id string, constraint cargo.ConfigMetadataDependencyConstraint, existing []cargo.ConfigMetadataDependency) ([]Dependency, error) {
	release, err := fetchLatestRelease("graalvm", "graalvm-ce-builds")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch GraalVM release: %w", err)
	}

	extractedVersion := extractGraalVMVersion(release.TagName)
	if extractedVersion == "" {
		return nil, fmt.Errorf("unable to extract version from tag %s", release.TagName)
	}

	ch := make(chan *Dependency, len(getSupportedPlatformStackTargets()))

	for _, pt := range getSupportedPlatformStackTargets() {
		go func(pt PlatformStackTarget) {
			archSuffix := "x64"
			if pt.arch == "arm64" {
				archSuffix = "aarch64"
			}

			assetURL := findGraalVMAsset(release.Assets, archSuffix)
			if assetURL == "" {
				fmt.Printf("Warning: no GraalVM asset found for %s %s\n", id, pt.target)
				ch <- nil
				return
			}

			checksum, err := downloadAndCalculateSHA256(assetURL)
			if err != nil {
				fmt.Printf("Warning: failed to calculate checksum for %s %s %s: %v\n", id, extractedVersion, pt.target, err)
				ch <- nil
				return
			}

			sourceURL := release.TarballURL
			sourceChecksum := ""
			if sourceURL != "" {
				sc, err := downloadAndCalculateSHA256(sourceURL)
				if err != nil {
					fmt.Printf("Warning: failed to calculate source checksum for %s %s: %v\n", id, extractedVersion, err)
				} else {
					sourceChecksum = sc
				}
			}

			purl := fmt.Sprintf("pkg:generic/graalvm-jdk@%s?arch=%s", extractedVersion, pt.arch)

			cpe := generateOracleCPE(extractedVersion)

			name := "GraalVM JDK"

			dep := cargo.ConfigMetadataDependency{
				ID:           id,
				Name:         name,
				Version:      extractedVersion,
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
			ch <- &d
		}(pt)
	}

	var dependencies []Dependency
	for range getSupportedPlatformStackTargets() {
		if d := <-ch; d != nil {
			dependencies = append(dependencies, *d)
		}
	}

	return dependencies, nil
}

func findGraalVMAsset(assets []GitHubAsset, archPattern string) string {
	for _, asset := range assets {
		if strings.Contains(asset.Name, fmt.Sprintf("linux-%s", archPattern)) &&
			strings.HasSuffix(asset.Name, ".tar.gz") &&
			strings.Contains(asset.Name, "jdk") {
			return asset.BrowserDownloadURL
		}
	}
	return ""
}

func extractGraalVMVersion(tagName string) string {
	if strings.HasPrefix(tagName, "jdk-") {
		return strings.TrimPrefix(tagName, "jdk-")
	}
	return ""
}
