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

var correttoRepoMap = map[int]string{
	8:  "corretto-8",
	11: "corretto-11",
	17: "corretto-17",
	21: "corretto-21",
	23: "corretto-23",
	25: "corretto-25",
	26: "corretto-26",
}

func generateCorretto(id string, constraint cargo.ConfigMetadataDependencyConstraint, existing []cargo.ConfigMetadataDependency) ([]Dependency, error) {
	majorVersion, err := extractVersionFromConstraint(constraint.Constraint)
	if err != nil {
		return nil, fmt.Errorf("unable to extract version from constraint %s: %w", constraint.Constraint, err)
	}

	repo, ok := correttoRepoMap[majorVersion]
	if !ok {
		return nil, fmt.Errorf("unsupported Corretto major version: %d", majorVersion)
	}

	release, err := fetchLatestRelease("corretto", repo)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Corretto release for %s: %w", repo, err)
	}

	correttoVersion := release.TagName
	javaVersion := correttoVersionToJavaVersion(correttoVersion, majorVersion)

	ch := make(chan *Dependency, len(getSupportedPlatformStackTargets()))

	for _, pt := range getSupportedPlatformStackTargets() {
		go func(pt PlatformStackTarget) {
			archSuffix := "x64"
			if pt.arch == "arm64" {
				archSuffix = "aarch64"
			}

			assetURL := fmt.Sprintf(
				"https://corretto.aws/downloads/resources/%s/amazon-corretto-%s-linux-%s.tar.gz",
				correttoVersion,
				correttoVersion,
				archSuffix,
			)

			checksum, err := downloadAndCalculateSHA256(assetURL)
			if err != nil {
				fmt.Printf("Warning: failed to calculate checksum for %s %s %s: %v\n", id, javaVersion, pt.target, err)
				ch <- nil
				return
			}

			sourceURL := fmt.Sprintf(
				"https://github.com/corretto/%s/archive/refs/tags/%s.tar.gz",
				repo,
				correttoVersion,
			)

			sourceChecksum, err := downloadAndCalculateSHA256(sourceURL)
			if err != nil {
				fmt.Printf("Warning: failed to calculate source checksum for %s %s: %v\n", id, javaVersion, err)
				sourceChecksum = ""
			}

			purl := fmt.Sprintf("pkg:generic/amazon/corretto-jdk@%s?arch=%s", javaVersion, pt.arch)

			cpe := generateOracleCPE(javaVersion)

			name := "Amazon Corretto JDK"

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

func correttoVersionToJavaVersion(correttoVersion string, majorVersion int) string {
	parts := strings.Split(correttoVersion, ".")
	if majorVersion == 8 {
		if len(parts) >= 2 {
			return fmt.Sprintf("8.0.%s", parts[1])
		}
	} else {
		if len(parts) >= 3 {
			return fmt.Sprintf("%s.%s.%s", parts[0], parts[1], parts[2])
		}
	}
	return correttoVersion
}
