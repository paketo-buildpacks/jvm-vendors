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
		sourceChecksum := dep.SourceChecksum
		if sourceChecksum == "" {
			sourceChecksum = dep.SourceSHA256
		}

		if dep.Source == sourceURL && sourceChecksum != "" {
			sourceChecksumCache[sourceURL] = sourceChecksum
			return sourceChecksum
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

	supportedVersions := []int{8, 11, 17, 21, 25, 27}

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

	// buildMetadataSeparators matches everything that is not allowed in a semver
	// build metadata identifier.
	buildMetadataSeparators = regexp.MustCompile(`[^0-9A-Za-z-]+`)
)

// Convert adoptium version format to semver
func adoptiumVersionToSemver(version string) string {
	// Handle formats like "8u432-b06" or "jdk-11.0.25+9" or "27.0.1-8"
	version = strings.TrimPrefix(version, "jdk")
	version = strings.TrimPrefix(version, "-")

	// Java 8: 8u432-b06 -> 8.0.432
	if matches := java8Pattern.FindStringSubmatch(version); matches != nil {
		return fmt.Sprintf("8.0.%s", matches[2])
	}

	// Java 11+: 27.0.1-8 -> 27.0.1+8. The trailing component is a build number, so
	// it is preserved as build metadata rather than dropped.
	if core, build, ok := strings.Cut(version, "-"); ok {
		version = core + "+" + build
	}

	return vendorVersionToSemver(version)
}

// withBuildMetadata attaches the given components to version as semver build
// metadata, e.g. withBuildMetadata("21.0.13", "0", "11") -> "21.0.13+0.11".
// Components are split on characters that are not valid in a build metadata
// identifier and empty parts are dropped.
//
// Build metadata is ignored when versions are compared, so recording a vendor's
// build/revision this way keeps rebuilds of the same Java version distinct
// without changing how dependencies sort.
func withBuildMetadata(version string, components ...string) string {
	var identifiers []string
	for _, component := range components {
		for _, identifier := range buildMetadataSeparators.Split(component, -1) {
			if identifier != "" {
				identifiers = append(identifiers, normalizeBuildIdentifier(identifier))
			}
		}
	}

	if len(identifiers) == 0 {
		return version
	}

	return version + "+" + strings.Join(identifiers, ".")
}

// normalizeBuildIdentifier strips leading zeroes from purely numeric build
// components, so that Corretto's "09" is recorded as "9".
func normalizeBuildIdentifier(identifier string) string {
	for _, r := range identifier {
		if r < '0' || r > '9' {
			return identifier
		}
	}

	trimmed := strings.TrimLeft(identifier, "0")
	if trimmed == "" {
		return "0"
	}

	return trimmed
}

// vendorVersionToSemver converts a version as published by a vendor into a
// semver compliant version. The major.minor.patch core is kept and anything the
// vendor appends beyond it, either additional numeric components or a trailing
// "+build", is retained as build metadata.
//
//	21.0.13.0.11 -> 21.0.13+0.11
//	25.4.4.1.1   -> 25.4.4+1.1
//	17.0.21+9    -> 17.0.21+9
func vendorVersionToSemver(rawVersion string) string {
	core, build, _ := strings.Cut(rawVersion, "+")

	parts := strings.Split(core, ".")
	if len(parts) > 3 {
		core = strings.Join(parts[:3], ".")
		parts = parts[3:]
	} else {
		parts = nil
	}

	return withBuildMetadata(core, append(parts, build)...)
}
