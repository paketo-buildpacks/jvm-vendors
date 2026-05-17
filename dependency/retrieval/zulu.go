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
	"strings"

	"github.com/paketo-buildpacks/packit/v2/cargo"
)

// AzulPackage represents Azul metadata API response
type AzulPackage struct {
	PackageUUID        string `json:"package_uuid"`
	Name               string `json:"name"`
	JavaVersion        []int  `json:"java_version"`
	OpenJDKBuildNumber int    `json:"openjdk_build_number"`
	Latest             bool   `json:"latest"`
	DownloadURL        string `json:"download_url"`
	SHA256Hash         string `json:"sha256_hash"`
	Architecture       string `json:"arch"`
	OS                 string `json:"os"`
	SupportTerm        string `json:"support_term"`
	PackageType        string `json:"package_type"`
}

func generateZulu(id string, constraint cargo.ConfigMetadataDependencyConstraint, existing []cargo.ConfigMetadataDependency) ([]Dependency, error) {
	imageType := "jdk"
	if id == "jre-azul-zulu" {
		imageType = "jre"
	}

	majorVersion, err := extractVersionFromConstraint(constraint.Constraint)
	if err != nil {
		return nil, fmt.Errorf("skipping Azul Zulu version %s: %w", constraint.Constraint, err)
	}

	type result struct {
		dep Dependency
	}

	platformTargets := getSupportedPlatformStackTargets()
	ch := make(chan *Dependency, len(platformTargets))

	for _, pt := range platformTargets {
		go func(pt PlatformStackTarget) {
			azulArch := toAzulArch(pt.arch)

			apiURL := fmt.Sprintf(
				"https://api.azul.com/metadata/v1/zulu/packages/?java_version=%d&arch=%s&os=linux&java_package_type=%s&latest=true&release_status=ga&availability_status=CA&certifications=tck&page=1&page_size=1",
				majorVersion,
				azulArch,
				imageType,
			)

			packages, err := fetchAzulPackages(apiURL)
			if err != nil {
				fmt.Printf("Warning: failed to fetch Azul packages for %s: %v\n", id, err)
				ch <- nil
				return
			}

			if len(packages) == 0 {
				ch <- nil
				return
			}

			pkg := packages[0]

			extractedVersion := extractAzulVersion(pkg.JavaVersion)
			if extractedVersion == "" {
				ch <- nil
				return
			}

			if existingDep := findExistingDependency(existing, id, pkg.DownloadURL); existingDep != nil {
				fmt.Printf("  Using cached metadata for %s %s %s\n", id, extractedVersion, pt.target)
				d := dependencyFromExisting(existingDep, pt.os, pt.arch)
				ch <- &d
				return
			}

			purl := fmt.Sprintf("pkg:generic/azul-zulu-%s@%s?arch=%s", imageType, extractedVersion, pt.arch)

			cpe := generateOracleCPE(extractedVersion)

			dep := cargo.ConfigMetadataDependency{
				ID:       id,
				Name:     "Azul Zulu " + strings.ToUpper(imageType),
				Version:  extractedVersion,
				URI:      pkg.DownloadURL,
				SHA256:   pkg.SHA256Hash,
				Stacks:   pt.stacks,
				OS:       pt.os,
				Arch:     pt.arch,
				CPE:      cpe,
				PURL:     purl,
				Licenses: getLicenses(cargo.ConfigMetadataDependency{}),
			}

			d := createDependency(dep, pt.target)
			ch <- &d
		}(pt)
	}

	var dependencies []Dependency
	for range platformTargets {
		if d := <-ch; d != nil {
			dependencies = append(dependencies, *d)
		}
	}

	return dependencies, nil
}

func fetchAzulPackages(url string) ([]AzulPackage, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Azul API returned status %d", resp.StatusCode)
	}

	var packages []AzulPackage
	if err := json.NewDecoder(resp.Body).Decode(&packages); err != nil {
		return nil, err
	}

	return packages, nil
}

func extractAzulVersion(javaVersion []int) string {
	if len(javaVersion) < 3 {
		return ""
	}

	major := javaVersion[0]
	minor := javaVersion[1]
	security := javaVersion[2]

	if major == 1 && minor == 8 {
		return fmt.Sprintf("8.0.%d", security)
	}

	return fmt.Sprintf("%d.%d.%d", major, minor, security)
}
