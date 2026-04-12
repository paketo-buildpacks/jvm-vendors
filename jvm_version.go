/*
 * Copyright 2018-2026 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package jvmvendors

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/heroku/color"
	"github.com/paketo-buildpacks/libpak/v2"
	"github.com/paketo-buildpacks/libpak/v2/log"
)

type JVMVersion struct {
	Logger log.Logger
}

func NewJVMVersion(logger log.Logger) JVMVersion {
	return JVMVersion{Logger: logger}
}

func (j JVMVersion) GetJVMVersion(appPath string, cr libpak.ConfigurationResolver, dr libpak.DependencyResolver, vendor string) (string, error) {
	version, explicit := cr.Resolve("BP_JVM_VERSION")
	if explicit {
		f := color.New(color.Faint)
		j.Logger.Body(f.Sprintf("Using Java version %s from BP_JVM_VERSION", version))
		return version, nil
	}

	sdkmanrcJavaVersion, err := readJavaVersionFromSDKMANRCFile(appPath)
	if err != nil {
		return "", fmt.Errorf("unable to read Java version from SDMANRC file\n%w", err)
	}

	if len(sdkmanrcJavaVersion) > 0 {
		sdkmanrcJavaMajorVersion := extractMajorVersion(sdkmanrcJavaVersion)
		f := color.New(color.Faint)
		j.Logger.Body(f.Sprintf("Using Java version %s extracted from .sdkmanrc", sdkmanrcJavaMajorVersion))
		return sdkmanrcJavaMajorVersion, nil
	}

	mavenJavaVersion, err := readJavaVersionFromMavenMetadata(appPath)
	if err != nil {
		return "", fmt.Errorf("unable to read Java version from Maven metadata\n%w", err)
	}

	if len(mavenJavaVersion) > 0 {
		mavenJavaMajorVersion := extractMajorVersion(mavenJavaVersion)
		retrieveNextAvailableJavaVersionIfMavenVersionNotAvailable(dr, &mavenJavaMajorVersion, vendor)
		f := color.New(color.Faint)
		j.Logger.Body(f.Sprintf("Using Java version %s extracted from MANIFEST.MF", mavenJavaMajorVersion))
		return mavenJavaMajorVersion, nil
	}

	f := color.New(color.Faint)
	j.Logger.Body(f.Sprintf("Using buildpack default Java version %s", version))
	return version, nil
}

func retrieveNextAvailableJavaVersionIfMavenVersionNotAvailable(dr libpak.DependencyResolver, mavenJavaMajorVersion *string, vendor string) {
	_, jdkErr := dr.Resolve(fmt.Sprintf("jdk-%s", vendor), *mavenJavaMajorVersion)
	_, jreErr := dr.Resolve(fmt.Sprintf("jre-%s", vendor), *mavenJavaMajorVersion)
	if libpak.IsNoValidDependencies(jdkErr) && libpak.IsNoValidDependencies(jreErr) {
		//	the buildpack does not provide the wanted JDK or JRE version - let's check if we can choose a more recent version
		mavenJavaMajorVersionAsInt, _ := strconv.ParseInt(*mavenJavaMajorVersion, 10, 64)
		nextVersionToEvaluate := mavenJavaMajorVersionAsInt + 1
		_, jdkErr := dr.Resolve(fmt.Sprintf("jdk-%s", vendor), strconv.FormatInt(nextVersionToEvaluate, 10))
		_, jreErr := dr.Resolve(fmt.Sprintf("jre-%s", vendor), strconv.FormatInt(nextVersionToEvaluate, 10))
		if libpak.IsNoValidDependencies(jdkErr) && libpak.IsNoValidDependencies(jreErr) {
			// we tried with the next major version, still no Java candidate, we are done trying.
		} else {
			*mavenJavaMajorVersion = strconv.FormatInt(nextVersionToEvaluate, 10)
		}
	}
}

func readJavaVersionFromSDKMANRCFile(appPath string) (string, error) {
	components, err := ReadSDKMANRC(filepath.Join(appPath, ".sdkmanrc"))
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return "", nil
	} else if err != nil {
		return "", err
	}

	for _, component := range components {
		if component.Type == "java" {
			return component.Version, nil
		}
	}

	return "", nil
}

func readJavaVersionFromMavenMetadata(appPath string) (string, error) {
	manifest, err := NewManifest(appPath)
	if err != nil {
		return "", err
	}

	javaVersion, ok := manifest.Get("Build-Jdk-Spec")
	if !ok {
		javaVersion, _ = manifest.Get("Build-Jdk")
	}

	return javaVersion, nil
}

func extractMajorVersion(version string) string {
	versionParts := strings.Split(version, ".")

	if versionParts[0] == "1" {
		return versionParts[1]
	}

	return versionParts[0]
}
