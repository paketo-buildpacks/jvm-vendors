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
	"sort"
	"strings"

	"github.com/paketo-buildpacks/packit/v2/cargo"
)

// BellsoftRelease represents the BellSoft Liberica API response
type BellsoftRelease struct {
	FeatureVersion int    `json:"featureVersion"`
	InterimVersion int    `json:"interimVersion"`
	UpdateVersion  int    `json:"updateVersion"`
	BuildVersion   int    `json:"buildVersion"`
	DownloadURL    string `json:"downloadUrl"`
	Architecture   string `json:"architecture"`
	Components     []struct {
		Version   string `json:"version"`
		Component string `json:"component"`
	}
}

func generateBellsoft(id string, constraint cargo.ConfigMetadataDependencyConstraint, existing []cargo.ConfigMetadataDependency) ([]Dependency, error) {
	bundleType := "jdk"
	product := "liberica"

	switch id {
	case "jre-bellsoft-liberica":
		bundleType = "jre"
	case "native-image-svm-bellsoft-liberica":
		bundleType = "core"
		product = "nik"
	}

	majorVersion, err := extractVersionFromConstraint(constraint.Constraint)
	if err != nil {
		return nil, fmt.Errorf("unable to extract version from constraint %s: %w", constraint.Constraint, err)
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
		// for NIK, the API's "latest" feature does not work
		// so we fetch all the versions, sort and pick the most recent one on the client side
		sort.Slice(releases, func(i, j int) bool {
			if releases[i].FeatureVersion != releases[j].FeatureVersion {
				return releases[i].FeatureVersion > releases[j].FeatureVersion
			}
			if releases[i].InterimVersion != releases[j].InterimVersion {
				return releases[i].InterimVersion > releases[j].InterimVersion
			}
			if releases[i].UpdateVersion != releases[j].UpdateVersion {
				return releases[i].UpdateVersion > releases[j].UpdateVersion
			}
			return releases[i].BuildVersion > releases[j].BuildVersion
		})
		releases = releases[:1]
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
		arch := release.Architecture
		switch arch {
		case "arm":
			arch = "arm64"
		case "x86":
			arch = "amd64"
		}

		pt := PlatformStackTarget{
			stacks: supportedStacks,
			target: fmt.Sprintf("linux-%s", arch),
			os:     "linux",
			arch:   arch,
		}

		version := fmt.Sprintf("%d.%d.%d-%d", release.FeatureVersion, release.InterimVersion, release.UpdateVersion, release.BuildVersion)

		if product == "nik" {
			version = determineBellsoftNIKVersion(release)
		}

		if existingDep := findExistingDependency(existing, id, release.DownloadURL); existingDep != nil {
			fmt.Printf("  Using cached metadata for %s %s %s\n", id, version, pt.target)
			d := dependencyFromExisting(existingDep, pt.os, pt.arch)
			dependencies = append(dependencies, d)
			continue
		}

		checksum, err := downloadAndCalculateSHA256(release.DownloadURL)
		if err != nil {
			fmt.Printf("Warning: failed to calculate checksum for %s %s %s: %v\n", id, version, pt.target, err)
			continue
		}

		purl := fmt.Sprintf("pkg:generic/liberica/openjdk@%s?arch=%s", version, pt.arch)
		if product == "nik" {
			purl = fmt.Sprintf("pkg:generic/liberica/native-image@%s?arch=%s", version, pt.arch)
		}

		cpe := generateOracleCPE(version)

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

func determineBellsoftNIKVersion(r BellsoftRelease) string {
	componentVersion, err := retrieveBellsoftComponentVersionFor(r, "liberica")
	if err != nil {
		panic(err)
	}

	if v, err := normalizeVersion(componentVersion); err != nil {
		panic(err)
	} else {
		return v
	}
}

func retrieveBellsoftComponentVersionFor(r BellsoftRelease, componentName string) (string, error) {
	for _, v := range r.Components {
		if v.Component == componentName {
			return v.Version, nil
		}
	}
	return "", fmt.Errorf("unable to find a component for: %s", componentName)
}

func normalizeVersion(version string) (string, error) {
	version = strings.TrimPrefix(version, "jdk")
	version = strings.TrimPrefix(version, "-")

	if matches := java8Pattern.FindStringSubmatch(version); matches != nil {
		return fmt.Sprintf("8.0.%s", matches[2]), nil
	}

	version = strings.ReplaceAll(version, "+", "-")
	return version, nil
}
