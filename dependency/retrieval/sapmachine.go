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
	"time"

	"github.com/paketo-buildpacks/packit/v2/cargo"
)

func generateSapMachine(id string, constraint cargo.ConfigMetadataDependencyConstraint, existing []cargo.ConfigMetadataDependency) ([]Dependency, error) {
	imageType := "jdk"
	if id == "jre-sap-machine" {
		imageType = "jre"
	}

	release, err := fetchLatestRelease("SAP", "SapMachine")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch SapMachine release: %w", err)
	}

	sapVersion := extractSapMachineVersion(release.TagName)
	if sapVersion == "" {
		return nil, fmt.Errorf("unable to extract version from tag %s", release.TagName)
	}

	majorVersion, err := extractVersionFromConstraint(constraint.Constraint)
	if err != nil {
		return nil, fmt.Errorf("unable to extract version from constraint %s: %w", constraint.Constraint, err)
	}

	deprecationDate := calculateSapMachineDeprecationDate(majorVersion)

	ch := make(chan *Dependency, len(getSupportedPlatformStackTargets()))

	for _, pt := range getSupportedPlatformStackTargets() {
		go func(pt PlatformStackTarget) {
			archSuffix := "x64"
			if pt.arch == "arm64" {
				archSuffix = "aarch64"
			}

			assetURL := findSapMachineAsset(release.Assets, imageType, archSuffix)
			if assetURL == "" {
				fmt.Printf("Warning: no SapMachine asset found for %s %s %s\n", id, sapVersion, pt.target)
				ch <- nil
				return
			}

			checksum, err := downloadAndCalculateSHA256(assetURL)
			if err != nil {
				fmt.Printf("Warning: failed to calculate checksum for %s %s %s: %v\n", id, sapVersion, pt.target, err)
				ch <- nil
				return
			}

			sourceURL := release.TarballURL
			sourceChecksum := ""
			if sourceURL != "" {
				sc, err := downloadAndCalculateSHA256(sourceURL)
				if err != nil {
					fmt.Printf("Warning: failed to calculate source checksum for %s %s: %v\n", id, sapVersion, err)
				} else {
					sourceChecksum = sc
				}
			}

			purl := fmt.Sprintf("pkg:generic/sap-machine-%s@%s?arch=%s", imageType, sapVersion, pt.arch)

			cpe := generateOracleCPE(sapVersion)

			name := "SapMachine " + strings.ToUpper(imageType)

			dep := cargo.ConfigMetadataDependency{
				ID:              id,
				Name:            name,
				Version:         sapVersion,
				URI:             assetURL,
				SHA256:          checksum,
				Source:          sourceURL,
				SourceSHA256:    sourceChecksum,
				Stacks:          pt.stacks,
				OS:              pt.os,
				Arch:            pt.arch,
				CPE:             cpe,
				PURL:            purl,
				Licenses:        getLicenses(cargo.ConfigMetadataDependency{}),
				DeprecationDate: deprecationDate,
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

func findSapMachineAsset(assets []GitHubAsset, imageType, archPattern string) string {
	expectedSuffix := fmt.Sprintf("linux-%s_bin.tar.gz", archPattern)
	for _, asset := range assets {
		if strings.Contains(asset.Name, imageType) &&
			strings.HasSuffix(asset.Name, expectedSuffix) {
			return asset.BrowserDownloadURL
		}
	}
	return ""
}

func extractSapMachineVersion(tagName string) string {
	return strings.TrimPrefix(tagName, "sapmachine-")
}

func calculateSapMachineDeprecationDate(majorVersion int) *time.Time {
	switch majorVersion {
	case 17:
		t := time.Date(2029, 9, 30, 0, 0, 0, 0, time.UTC)
		return &t
	case 21:
		t := time.Date(2031, 9, 30, 0, 0, 0, 0, time.UTC)
		return &t
	default:
		return nil
	}
}
