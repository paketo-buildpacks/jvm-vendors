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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/paketo-buildpacks/packit/v2/cargo"
)

// FoojayPackage represents FooJay Disco API response
type FoojayPackage struct {
	ID                  string `json:"id"`
	ArchiveType         string `json:"archive_type"`
	Distribution        string `json:"distribution"`
	JDKVersion          int    `json:"jdk_version"`
	JavaVersion         string `json:"java_version"`
	DistributionVersion string `json:"distribution_version"`
	DirectDownloadURI   string `json:"direct_download_uri"`
	Checksum            string `json:"checksum"`
	ChecksumURI         string `json:"checksum_uri"`
	Filename            string `json:"filename"`
	PackageType         string `json:"package_type"`
	Architecture        string `json:"architecture"`
	OS                  string `json:"os"`
	Size                int    `json:"size"`
	Links               struct {
		PkgInfoURI          string `json:"pkg_info_uri"`
		PkgDownloadRedirect string `json:"pkg_download_redirect"`
	} `json:"links"`
}

type foojayDistro struct {
	// distro is the distribution name used by the FooJay API.
	distro string
	// name is the dependency name recorded in buildpack.toml.
	name string
	// purlName is the package name used in the dependency PURL.
	purlName string
}

var foojayDistroMap = map[string]foojayDistro{
	"jdk-microsoft-openjdk": {
		distro:   "microsoft",
		name:     "Microsoft OpenJDK",
		purlName: "microsoft/openjdk",
	},
	"jdk-oracle": {
		distro:   "oracle",
		name:     "Oracle JDK",
		purlName: "oracle-jdk",
	},
	"native-image-svm-oracle": {
		distro:   "graalvm",
		name:     "Oracle GraalVM",
		purlName: "graalvm-jdk",
	},
}

func generateFoojay(id string, constraint cargo.ConfigMetadataDependencyConstraint, existing []cargo.ConfigMetadataDependency) ([]Dependency, error) {
	distro, ok := foojayDistroMap[id]
	if !ok {
		return nil, fmt.Errorf("no FooJay distro mapping for %s", id)
	}

	majorVersion, err := extractVersionFromConstraint(constraint.Constraint)
	if err != nil {
		return nil, fmt.Errorf("skipping %s version %s: %w", id, constraint.Constraint, err)
	}

	packages, err := fetchFoojayPackages(distro.distro, majorVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch FooJay packages for %s: %w", id, err)
	}

	// FooJay can return more than one release and package type per architecture,
	// so keep only the newest compressed archive for each architecture.
	packagesByArch := make(map[string]FoojayPackage)
	for _, pkg := range packages {
		if !strings.HasSuffix(pkg.Filename, ".tar.gz") {
			continue
		}

		arch, ok := normalizeFoojayArch(pkg.Architecture)
		if !ok {
			continue
		}

		if current, ok := packagesByArch[arch]; !ok || newerFoojayPackage(pkg, current) {
			packagesByArch[arch] = pkg
		}
	}

	var dependencies []Dependency

	for _, pt := range getSupportedPlatformStackTargets() {
		pkg, ok := packagesByArch[pt.arch]
		if !ok {
			fmt.Printf("  Skipping %s %s: no package published for this platform\n", id, pt.target)
			continue
		}

		rawVersion := pkg.JavaVersion
		if rawVersion == "" {
			rawVersion = pkg.DistributionVersion
		}

		// Prefer the distribution version, which carries the vendor build (e.g.
		// 21.0.13.0.2 -> 21.0.13+0.2), so rebuilds are not treated as duplicates.
		version := vendorVersionToSemver(rawVersion)
		if pkg.DistributionVersion != "" && strings.HasPrefix(pkg.DistributionVersion, rawVersion) {
			version = vendorVersionToSemver(pkg.DistributionVersion)
		}

		downloadURL, checksum, err := resolveFoojayPackage(pkg)
		if err != nil {
			fmt.Printf("Warning: failed to resolve package for %s %s %s: %v\n", id, version, pt.target, err)
			continue
		}

		if existingDep := findExistingDependency(existing, id, downloadURL); existingDep != nil {
			fmt.Printf("  Using cached metadata for %s %s %s\n", id, version, pt.target)
			d := dependencyFromExisting(existingDep, version, pt.os, pt.arch)
			dependencies = append(dependencies, d)
			continue
		}

		purl := fmt.Sprintf("pkg:generic/%s@%s?arch=%s", distro.purlName, rawVersion, pt.arch)

		cpe := generateOracleCPE(rawVersion)

		dep := cargo.ConfigMetadataDependency{
			ID:       id,
			Name:     distro.name,
			Version:  version,
			URI:      downloadURL,
			SHA256:   checksum,
			Stacks:   pt.stacks,
			OS:       pt.os,
			Arch:     pt.arch,
			CPE:      cpe,
			PURL:     purl,
			Licenses: getLicenses(cargo.ConfigMetadataDependency{}),
		}

		dependencies = append(dependencies, createDependency(dep, pt.target))
	}

	return dependencies, nil
}

func normalizeFoojayArch(arch string) (string, bool) {
	switch arch {
	case "x64", "amd64":
		return "amd64", true
	case "aarch64", "arm64":
		return "arm64", true
	}

	return "", false
}

// resolveFoojayPackage returns the download URL and SHA256 checksum for a
// package. FooJay does not include either in the package listing for every
// distribution (Oracle, for example), so the package info document is consulted
// and the checksum is calculated from the archive as a last resort.
func resolveFoojayPackage(pkg FoojayPackage) (string, string, error) {
	downloadURL := pkg.DirectDownloadURI
	if downloadURL == "" {
		downloadURL = pkg.Links.PkgDownloadRedirect
	}
	checksum := pkg.Checksum

	if pkg.Links.PkgInfoURI != "" && (pkg.DirectDownloadURI == "" || checksum == "") {
		var info struct {
			Result []struct {
				DirectDownloadURI string `json:"direct_download_uri"`
				Checksum          string `json:"checksum"`
				ChecksumURI       string `json:"checksum_uri"`
			} `json:"result"`
		}

		if err := fetchJSON(pkg.Links.PkgInfoURI, &info); err == nil && len(info.Result) > 0 {
			result := info.Result[0]

			if result.DirectDownloadURI != "" {
				downloadURL = result.DirectDownloadURI
			}

			switch {
			case result.Checksum != "":
				checksum = result.Checksum
			case result.ChecksumURI != "":
				if value, err := fetchChecksumFile(result.ChecksumURI); err == nil {
					checksum = value
				}
			}
		}
	}

	if downloadURL == "" {
		return "", "", fmt.Errorf("no download URL found")
	}

	if checksum == "" {
		var err error

		checksum, err = downloadAndCalculateSHA256(downloadURL)
		if err != nil {
			return "", "", fmt.Errorf("unable to calculate checksum: %w", err)
		}
	}

	return downloadURL, checksum, nil
}

func fetchJSON(url string, target any) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unable to download %s: %d", url, resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

func fetchChecksumFile(url string) (string, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unable to download %s: %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	match := sha256Pattern.Find(body)
	if match == nil {
		return "", fmt.Errorf("no sha256 checksum found in %s", url)
	}

	return string(match), nil
}

var sha256Pattern = regexp.MustCompile(`[0-9a-fA-F]{64}`)

func newerFoojayPackage(a, b FoojayPackage) bool {
	aVersion, err := semver.NewVersion(vendorVersionToSemver(a.JavaVersion))
	if err != nil {
		return false
	}

	bVersion, err := semver.NewVersion(vendorVersionToSemver(b.JavaVersion))
	if err != nil {
		return true
	}

	return aVersion.GreaterThan(bVersion)
}

func fetchFoojayPackages(distro string, majorVersion int) ([]FoojayPackage, error) {
	params := url.Values{}
	params.Set("version", fmt.Sprintf("%d", majorVersion))
	params.Set("distro", distro)
	params.Set("operating_system", "linux")
	params.Set("package_type", "jdk")
	params.Set("latest", "available")

	for _, arch := range []string{"x64", "aarch64"} {
		params.Add("architecture", arch)
	}

	apiURL := fmt.Sprintf("https://api.foojay.io/disco/v3.0/packages?%s", params.Encode())

	resp, err := httpClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from FooJay API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("FooJay API returned status %d", resp.StatusCode)
	}

	var result struct {
		Result []FoojayPackage `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode FooJay API response: %w", err)
	}

	if len(result.Result) == 0 {
		return nil, fmt.Errorf("no packages found for distro=%s version=%d", distro, majorVersion)
	}

	return result.Result, nil
}
