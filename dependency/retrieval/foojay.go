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
	"net/http"
	"net/url"

	"github.com/paketo-buildpacks/packit/v2/cargo"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
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
		PkgDownloadRedirect string `json:"pkg_download_redirect"`
	} `json:"links"`
}

var foojayDistroMap = map[string]string{}

func generateFoojay(id string, constraint cargo.ConfigMetadataDependencyConstraint, existing []cargo.ConfigMetadataDependency) ([]Dependency, error) {
	distro, ok := foojayDistroMap[id]
	if !ok {
		return nil, fmt.Errorf("no FooJay distro mapping for %s", id)
	}

	majorVersion, err := extractVersionFromConstraint(constraint.Constraint)
	if err != nil {
		return nil, fmt.Errorf("skipping %s version %s: %w", id, constraint.Constraint, err)
	}

	packages, err := fetchFoojayPackages(distro, majorVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch FooJay packages for %s: %w", id, err)
	}

	packagesByArch := make(map[string]FoojayPackage)
	for _, pkg := range packages {
		packagesByArch[pkg.Architecture] = pkg
	}

	var dependencies []Dependency

	for _, pt := range getSupportedPlatformStackTargets() {
		foojayArch := pt.arch

		switch pt.arch {
		case "amd64":
			foojayArch = "x64"
		case "arm64":
			foojayArch = "aarch64"
		}

		pkg, ok := packagesByArch[foojayArch]
		if !ok {
			fmt.Printf("Warning: no FooJay package found for %s %s\n", id, pt.target)
			continue
		}

		downloadURL := pkg.DirectDownloadURI
		if downloadURL == "" {
			downloadURL = pkg.Links.PkgDownloadRedirect
		}

		if downloadURL == "" {
			fmt.Printf("Warning: no download URL found for %s %s\n", id, pt.target)
			continue
		}

		version := pkg.JavaVersion
		if version == "" {
			version = pkg.DistributionVersion
		}

		if existingDep := findExistingDependency(existing, id, downloadURL); existingDep != nil {
			fmt.Printf("  Using cached metadata for %s %s %s\n", id, version, pt.target)
			d := dependencyFromExisting(existingDep, pt.os, pt.arch)
			dependencies = append(dependencies, d)
			continue
		}

		purl := fmt.Sprintf("pkg:generic/%s/openjdk@%s?arch=%s", distro, version, pt.arch)

		cpe := generateOracleCPE(version)

		name := cases.Title(language.English).String(distro) + " OpenJDK"

		dep := cargo.ConfigMetadataDependency{
			ID:       id,
			Name:     name,
			Version:  version,
			URI:      downloadURL,
			SHA256:   pkg.Checksum,
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
