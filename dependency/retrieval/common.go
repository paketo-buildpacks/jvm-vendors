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
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/paketo-buildpacks/packit/v2/cargo"
)

var httpClient = &http.Client{
	Timeout: 5 * time.Minute,
	Transport: &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   6 * time.Second,
			KeepAlive: 60 * time.Second,
		}).Dial,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
}

var sourceChecksumCache = make(map[string]string)

// Convert architecture to various vendor-specific formats
func toAdoptiumArch(arch string) string {
	switch arch {
	case "arm64":
		return "aarch64"
	case "amd64":
		return "x64"
	}
	return arch
}

func toAzulArch(arch string) string {
	if arch == "arm64" {
		return "aarch64"
	}
	return arch
}

// Generate CPE for Oracle JDK (using Oracle CPE as the primary one)
func generateOracleCPE(version string) string {
	// Try to parse as semver
	v, err := semver.NewVersion(version)
	if err != nil {
		// Java 8 style version: 8.0.432
		if after, ok := strings.CutPrefix(version, "8.0."); ok {
			patch := after
			return fmt.Sprintf("cpe:2.3:a:oracle:jdk:1.8.0:update%s:*:*:*:*:*:*", patch)
		}
		return fmt.Sprintf("cpe:2.3:a:oracle:jdk:%s:*:*:*:*:*:*:*", version)
	}

	major := v.Major()
	minor := v.Minor()
	patch := v.Patch()

	if major == 8 {
		return fmt.Sprintf("cpe:2.3:a:oracle:jdk:1.8.0:update%d:*:*:*:*:*:*", patch)
	}
	return fmt.Sprintf("cpe:2.3:a:oracle:jdk:%d.%d.%d:*:*:*:*:*:*:*", major, minor, patch)
}

// Get licenses from existing dependency
func getLicenses(existing cargo.ConfigMetadataDependency) []any {
	if len(existing.Licenses) > 0 {
		return existing.Licenses
	}
	return []any{
		cargo.ConfigBuildpackLicense{
			Type: "GPL-2.0 WITH Classpath-exception-2.0",
			URI:  "https://openjdk.java.net/legal/gplv2+ce.html",
		},
	}
}

// Get or compute source checksum, using cache and existing dependencies to avoid re-downloading
func getSourceChecksum(sourceURL string, existing []cargo.ConfigMetadataDependency) string {
	if sourceURL == "" {
		return ""
	}

	if checksum, ok := sourceChecksumCache[sourceURL]; ok {
		return checksum
	}

	for _, dep := range existing {
		if dep.Source == sourceURL && dep.SourceSHA256 != "" {
			sourceChecksumCache[sourceURL] = dep.SourceSHA256
			return dep.SourceSHA256
		}
	}

	checksum, err := downloadAndCalculateSHA256(sourceURL)
	if err != nil {
		fmt.Printf("Warning: failed to calculate source checksum for %s: %v\n", sourceURL, err)
		return ""
	}

	sourceChecksumCache[sourceURL] = checksum
	return checksum
}

func extractVersionFromConstraint(constraint string) (int, error) {
	c, err := semver.NewConstraint(constraint)
	if err != nil {
		return -1, fmt.Errorf("unable to parse constraint %s: %w", constraint, err)
	}

	supportedVersions := []int{8, 11, 17, 21, 23, 25, 26}

	for _, major := range supportedVersions {
		testVersion := semver.New(uint64(major), 0, 0, "", "")
		if c.Check(testVersion) {
			return major, nil
		}
	}
	return -1, fmt.Errorf("unable to match version major version %s", constraint)
}

// Regex patterns
var (
	java8Pattern = regexp.MustCompile(`^(\d+)u(\d+)`)
)

// Convert adoptium version format to semver
func adoptiumVersionToSemver(version string) string {
	// Handle formats like "8u432-b06" or "jdk-11.0.25+9" or "26.0.1-8"
	version = strings.TrimPrefix(version, "jdk")
	version = strings.TrimPrefix(version, "-")

	// Java 8: 8u432-b06 -> 8.0.432
	if matches := java8Pattern.FindStringSubmatch(version); matches != nil {
		return fmt.Sprintf("8.0.%s", matches[2])
	}

	// Java 11+: 11.0.25+9 -> 11.0.25, 26.0.1-8 -> 26.0.1
	if before, _, ok := strings.Cut(version, "+"); ok {
		return before
	}
	if before, _, ok := strings.Cut(version, "-"); ok {
		return before
	}
	return version
}
