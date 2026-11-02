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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/paketo-buildpacks/packit/v2/cargo"
)

// BellsoftRelease represents the BellSoft Liberica API response
type BellsoftRelease struct {
	FeatureVersion int    `json:"featureVersion"`
	InterimVersion int    `json:"interimVersion"`
	UpdateVersion  int    `json:"updateVersion"`
	BuildVersion   int    `json:"buildVersion"`
	Version        string `json:"version"`
	DownloadURL    string `json:"downloadUrl"`
	Architecture   string `json:"architecture"`
	Components     []struct {
		Version   string `json:"version"`
		Component string `json:"component"`
	}
}

// bellsoftArchitectures maps the architecture names used by the BellSoft API to
// the architecture names used by buildpacks.
var bellsoftArchitectures = map[string]string{
	"arm":   "arm64",
	"x86":   "amd64",
	"ppc":   "ppc64le",
	"riscv": "riscv64",
	"s390x": "s390x",
}

// bellsoftPlatforms are the architectures this buildpack ships BellSoft Liberica
// for. BellSoft also publishes riscv64 and s390x builds, but those are not
// supported by the buildpack, so they are skipped rather than being reported
// under the wrong architecture.
var bellsoftPlatforms = []string{"amd64", "arm64", "ppc64le"}

func generateBellsoft(id string, constraint cargo.ConfigMetadataDependencyConstraint, existing []cargo.ConfigMetadataDependency) ([]Dependency, error) {
	bundleType := "jdk"
	product := "liberica"

	majorVersion, err := extractVersionFromConstraint(constraint.Constraint)
	if err != nil {
		return nil, fmt.Errorf("unable to extract version from constraint %s: %w", constraint.Constraint, err)
	}

	switch id {
	case "jre-bellsoft-liberica":
		bundleType = "jre"
	case "native-image-svm-bellsoft-liberica":
		if majorVersion >= 25 {
			bundleType = "standard"
		} else {
			bundleType = "core"
		}
		product = "nik"
	}

	uri := ""
	sourceURI := ""

	switch product {
	case "liberica":
		uriStaticParams := "?bitness=64&os=linux&package-type=tar.gz&version-modifier=latest"
		sourceURIStaticParams := "?package-type=src.tar.gz&version-modifier=latest"

		uri = fmt.Sprintf("https://api.bell-sw.com/v1/%s/releases%s&bundle-type=%s&version-feature=%d",
			product, uriStaticParams, bundleType, majorVersion)
		sourceURI = fmt.Sprintf("https://api.bell-sw.com/v1/%s/releases%s&bundle-type=jdk&version-feature=%d",
			product, sourceURIStaticParams, majorVersion)
	case "nik":
		uriStaticParams := "?bitness=64&os=linux&package-type=tar.gz"
		sourceURIStaticParams := "?package-type=src.tar.gz"

		uri = fmt.Sprintf("https://api.bell-sw.com/v1/%s/releases%s&bundle-type=%s&component-version=liberica%%40%d",
			product, uriStaticParams, bundleType, majorVersion)
		sourceURI = fmt.Sprintf("https://api.bell-sw.com/v1/%s/releases%s&bundle-type=%s&component-version=liberica%%40%d",
			product, sourceURIStaticParams, "standard", majorVersion)
	}

	releases, err := fetchBellsoftReleases(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch BellSoft releases for %s %s: %w", id, constraint.Constraint, err)
	}

	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found for %s %s", id, constraint.Constraint)
	}

	if product == "nik" {
		// for NIK, the API's "latest" feature does not work, so we sort client side.
		// The release-level version fields are not populated for NIK, so order by the
		// liberica component version instead - the version this dependency is published under.
		sort.SliceStable(releases, func(i, j int) bool {
			return compareBellsoftComponentVersion(releases[i], releases[j]) > 0
		})
		// keep the newest release for each architecture; truncating to a single
		// element here would silently drop every architecture but one
		newestPerArch := make([]BellsoftRelease, 0, len(releases))
		seenArch := make(map[string]bool, len(releases))
		for _, release := range releases {
			if seenArch[release.Architecture] {
				continue
			}
			seenArch[release.Architecture] = true
			newestPerArch = append(newestPerArch, release)
		}
		releases = newestPerArch
	}

	sourceReleases, err := fetchBellsoftReleases(sourceURI)
	if err != nil {
		fmt.Printf("Warning: failed to fetch BellSoft source releases for %s %s: %v\n", id, constraint.Constraint, err)
	}

	sourceURL := ""
	if len(sourceReleases) > 0 {
		sourceURL = sourceReleases[0].DownloadURL
	}
	sourceChecksum := getSourceChecksum(sourceURL, existing)

	var dependencies []Dependency
	for _, release := range releases {
		arch, ok := bellsoftArchitectures[release.Architecture]
		if !ok {
			fmt.Printf("Warning: unknown BellSoft architecture %q for %s\n", release.Architecture, id)
			continue
		}
		if !slices.Contains(bellsoftPlatforms, arch) {
			continue
		}

		pt := PlatformStackTarget{
			stacks: supportedStacks,
			target: fmt.Sprintf("linux-%s", arch),
			os:     "linux",
			arch:   arch,
		}

		rawVersion := fmt.Sprintf("%d.%d.%d", release.FeatureVersion, release.InterimVersion, release.UpdateVersion)

		var version string
		var fullVersion string
		if product == "nik" {
			version, fullVersion = determineBellsoftNIKVersion(release)
		} else {
			sourceVersion := release.Version
			if sourceVersion == "" {
				sourceVersion = rawVersion
			}
			version = normalizeLibericaVersion(sourceVersion)
			fullVersion = rawVersion
		}

		if existingDep := findExistingDependency(existing, id, release.DownloadURL); existingDep != nil {
			fmt.Printf("  Using cached metadata for %s %s %s\n", id, version, pt.target)
			d := dependencyFromExisting(existingDep, version, pt.os, pt.arch)
			dependencies = append(dependencies, d)
			continue
		}

		checksum, err := downloadAndCalculateSHA256(release.DownloadURL)
		if err != nil {
			fmt.Printf("Warning: failed to calculate checksum for %s %s %s: %v\n", id, version, pt.target, err)
			continue
		}

		purl := fmt.Sprintf("pkg:generic/liberica/openjdk@%s?arch=%s", fullVersion, pt.arch)
		if product == "nik" {
			purl = fmt.Sprintf("pkg:generic/liberica/native-image@%s?arch=%s", fullVersion, pt.arch)
		}

		cpe := generateOracleCPE(fullVersion)

		name := "BellSoft Liberica " + strings.ToUpper(bundleType)
		if product == "nik" {
			name = "BellSoft Liberica Native Image"
		}

		dep := cargo.ConfigMetadataDependency{
			ID:           id,
			Name:         name,
			Version:      version,
			URI:          release.DownloadURL,
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

func fetchBellsoftReleases(url string) ([]BellsoftRelease, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("unable to get %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unable to download %s: %d", url, resp.StatusCode)
	}

	var releases []BellsoftRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("unable to decode payload: %w", err)
	}

	return releases, nil
}

func downloadAndCalculateSHA256(url string) (string, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("unable to download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unable to download %s: %d", url, resp.StatusCode)
	}

	h := sha256.New()
	if _, err := io.Copy(h, resp.Body); err != nil {
		return "", fmt.Errorf("unable to calculate checksum: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func determineBellsoftNIKVersion(r BellsoftRelease) (string, string) {
	componentVersion, err := retrieveBellsoftComponentVersionFor(r, "liberica")
	if err != nil {
		panic(err)
	}

	return normalizeLibericaVersion(componentVersion), componentVersion
}

func retrieveBellsoftComponentVersionFor(r BellsoftRelease, componentName string) (string, error) {
	for _, v := range r.Components {
		if v.Component == componentName {
			return v.Version, nil
		}
	}
	return "", fmt.Errorf("unable to find a component for: %s", componentName)
}

// normalizeLibericaVersion converts a BellSoft Liberica version into a semver
// version, keeping the Liberica build as build metadata. Liberica republishes
// the same Java version with a new build (e.g. 21.0.13+11 -> 21.0.13.0.3+5), so
// dropping the build makes the two indistinguishable.
//
//	21.0.13.0.3+5 -> 21.0.13+0.3.5
//	8u513+9       -> 8.0.513+9
func normalizeLibericaVersion(rawVersion string) string {
	rawVersion = strings.TrimPrefix(rawVersion, "jdk")
	rawVersion = strings.TrimPrefix(rawVersion, "-")

	if matches := java8Pattern.FindStringSubmatch(rawVersion); matches != nil {
		_, build, _ := strings.Cut(rawVersion, "+")
		return withBuildMetadata(fmt.Sprintf("8.0.%s", matches[2]), build)
	}

	return vendorVersionToSemver(rawVersion)
}

var versionNumberPattern = regexp.MustCompile(`\d+`)

// bellsoftComponentVersionParts returns the comparable numeric parts of a release's
// liberica component version: the version core padded to four fields, followed by the
// build number. "21+37" -> [21, 0, 0, 0, 37] and "21.0.12+2" -> [21, 0, 12, 0, 2].
//
// The core must be padded before the build number is appended, otherwise a GA release
// such as "21+37" compares its build number against the interim field of "21.0.12+2"
// and wrongly sorts ahead of it.
func bellsoftComponentVersionParts(r BellsoftRelease) []int {
	version, err := retrieveBellsoftComponentVersionFor(r, "liberica")
	if err != nil {
		return nil
	}

	core, build, _ := strings.Cut(version, "+")

	parts := make([]int, 0, 5)
	for _, field := range versionNumberPattern.FindAllString(core, -1) {
		number, err := strconv.Atoi(field)
		if err != nil {
			return nil
		}
		parts = append(parts, number)
	}

	for len(parts) < 4 {
		parts = append(parts, 0)
	}

	buildNumber := 0
	if fields := versionNumberPattern.FindAllString(build, -1); len(fields) > 0 {
		if number, err := strconv.Atoi(fields[0]); err == nil {
			buildNumber = number
		}
	}

	return append(parts, buildNumber)
}

// compareBellsoftComponentVersion orders two releases by their liberica component
// version, returning a positive number when a is newer than b.
func compareBellsoftComponentVersion(a, b BellsoftRelease) int {
	aParts, bParts := bellsoftComponentVersionParts(a), bellsoftComponentVersionParts(b)

	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		if aParts[i] != bParts[i] {
			if aParts[i] > bParts[i] {
				return 1
			}
			return -1
		}
	}

	return len(aParts) - len(bParts)
}
