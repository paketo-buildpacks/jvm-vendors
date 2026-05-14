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
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/paketo-buildpacks/packit/v2/cargo"
)

func generateOracle(id string, constraint cargo.ConfigMetadataDependencyConstraint, existing []cargo.ConfigMetadataDependency) ([]Dependency, error) {
	majorVersion, err := extractVersionFromConstraint(constraint.Constraint)
	if err != nil {
		return nil, fmt.Errorf("unable to extract version from constraint %s: %w", constraint.Constraint, err)
	}

	version, err := fetchOracleVersion(majorVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Oracle version for Java %d: %w", majorVersion, err)
	}

	ch := make(chan *Dependency, len(getSupportedPlatformStackTargets()))

	for _, pt := range getSupportedPlatformStackTargets() {
		go func(pt PlatformStackTarget) {
			archSuffix := "x64"
			if pt.arch == "arm64" {
				archSuffix = "aarch64"
			}

			assetURL := fmt.Sprintf(
				"https://download.oracle.com/java/%d/latest/jdk-%d_linux-%s_bin.tar.gz",
				majorVersion,
				majorVersion,
				archSuffix,
			)

			checksum, err := downloadAndCalculateSHA256(assetURL)
			if err != nil {
				fmt.Printf("Warning: failed to calculate checksum for %s %s %s: %v\n", id, version, pt.target, err)
				ch <- nil
				return
			}

			purl := fmt.Sprintf("pkg:generic/oracle-jdk@%s?arch=%s", version, pt.arch)

			cpe := generateOracleCPE(version)

			name := "Oracle JDK"

			dep := cargo.ConfigMetadataDependency{
				ID:           id,
				Name:         name,
				Version:      version,
				URI:          assetURL,
				SHA256:       checksum,
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

func fetchOracleVersion(majorVersion int) (string, error) {
	url := "https://www.oracle.com/java/technologies/downloads/"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("unable to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Cookie", "oraclelicense=accept-securebackup-cookie")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("unable to fetch Oracle downloads page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Oracle downloads page returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("unable to read response body: %w", err)
	}

	html := string(body)

	pattern := regexp.MustCompile(fmt.Sprintf(`<h3 id="java%d">Java SE Development Kit ([\d\.]+) downloads</h3>`, majorVersion))
	matches := pattern.FindStringSubmatch(html)
	if len(matches) < 2 {
		return "", fmt.Errorf("unable to find version for Java %d on Oracle downloads page", majorVersion)
	}

	return strings.TrimSpace(matches[1]), nil
}
