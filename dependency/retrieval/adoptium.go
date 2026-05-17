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
	"strings"

	"github.com/paketo-buildpacks/packit/v2/cargo"
)

// AdoptiumAsset represents the Adoptium API response
type AdoptiumAsset struct {
	Binaries []struct {
		Architecture string `json:"architecture"`
		HeapSize     string `json:"heap_size"`
		ImageType    string `json:"image_type"`
		JVMImpl      string `json:"jvm_impl"`
		OS           string `json:"os"`
		Package      struct {
			Checksum string `json:"checksum"`
			Link     string `json:"link"`
			Name     string `json:"name"`
			Size     int    `json:"size"`
		} `json:"package"`
		ProjectType string `json:"project_type"`
		SCMRef      string `json:"scm_ref"`
		UpdatedAt   string `json:"updated_at"`
	} `json:"binaries"`
	ReleaseName string `json:"release_name"`
	VersionData struct {
		Major          int    `json:"major"`
		Minor          int    `json:"minor"`
		Security       int    `json:"security"`
		Patch          int    `json:"patch"`
		Build          int    `json:"build"`
		Semver         string `json:"semver"`
		OpenjdkVersion string `json:"openjdk_version"`
	} `json:"version_data"`
	Source struct {
		Link     string `json:"link"`
		Name     string `json:"name"`
		Size     int    `json:"size"`
		Checksum string `json:"checksum"`
	} `json:"source"`
}

func generateAdoptium(id string, constraint cargo.ConfigMetadataDependencyConstraint, existing []cargo.ConfigMetadataDependency) ([]Dependency, error) {
	imageType := "jdk"
	if id == "jre-adoptium" {
		imageType = "jre"
	}

	ch := make(chan *Dependency, len(getSupportedPlatformStackTargets()))

	for _, pt := range getSupportedPlatformStackTargets() {
		go func(pt PlatformStackTarget) {
			adoptiumArch := toAdoptiumArch(pt.arch)

			major, err := extractVersionFromConstraint(constraint.Constraint)
			if err != nil {
				fmt.Printf("Warning: failed to parse constraint for %s %s: %v\n", id, constraint.Constraint, err)
				ch <- nil
				return
			}

			versionRange := fmt.Sprintf("[%d,%d)", major, major+1)

			apiURL := fmt.Sprintf("https://api.adoptium.net/v3/assets/version/%s"+
				"?architecture=%s"+
				"&os=linux"+
				"&image-type=%s"+ // either `jre` or `jdk`
				"&project=jdk"+ // always `jdk`
				"&release_type=ga"+
				"&semver=false",
				url.PathEscape(versionRange), adoptiumArch, imageType)

			assets, err := fetchAdoptiumAssets(apiURL)
			if err != nil {
				fmt.Printf("Warning: failed to fetch Adoptium assets for %s %s %s: %v\n", id, constraint.Constraint, pt.arch, err)
				ch <- nil
				return
			}

			if len(assets) == 0 || len(assets[0].Binaries) == 0 {
				ch <- nil
				return
			}

			binary := assets[0].Binaries[0]

			extractedVersion := adoptiumVersionToSemver(assets[0].VersionData.Semver)
			if extractedVersion == "" {
				extractedVersion = constraint.Constraint
			}

			if existingDep := findExistingDependency(existing, id, binary.Package.Link); existingDep != nil {
				fmt.Printf("  Using cached metadata for %s %s %s\n", id, extractedVersion, pt.target)
				d := dependencyFromExisting(existingDep, pt.os, pt.arch)
				ch <- &d
				return
			}

			purl := fmt.Sprintf("pkg:generic/adoptium-%s@%s?arch=%s", imageType, extractedVersion, pt.arch)

			cpe := generateOracleCPE(extractedVersion)

			dep := cargo.ConfigMetadataDependency{
				ID:           id,
				Name:         "Adoptium " + strings.ToUpper(imageType),
				Version:      extractedVersion,
				URI:          binary.Package.Link,
				SHA256:       binary.Package.Checksum,
				Source:       assets[0].Source.Link,
				SourceSHA256: assets[0].Source.Checksum,
				Stacks:       pt.stacks,
				OS:           pt.os,
				Arch:         pt.arch,
				CPE:          cpe,
				PURL:         purl,
				Licenses: []any{
					cargo.ConfigBuildpackLicense{
						Type: "GPL-2.0 WITH Classpath-exception-2.0",
						URI:  "https://openjdk.java.net/legal/gplv2+ce.html",
					},
				},
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

func fetchAdoptiumAssets(url string) ([]AdoptiumAsset, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("adoptium API returned status %d", resp.StatusCode)
	}

	var assets []AdoptiumAsset
	if err := json.NewDecoder(resp.Body).Decode(&assets); err != nil {
		return nil, err
	}

	return assets, nil
}
