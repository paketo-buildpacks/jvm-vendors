/*
 * Copyright 2018-2020 the original author or authors.
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
	"fmt"
	"os"
	"path/filepath"

	"github.com/buildpacks/libcnb/v2"
	"github.com/paketo-buildpacks/libpak/v2"
	"github.com/paketo-buildpacks/libpak/v2/crush"
)

type JDK struct {
	CertificateLoader CertificateLoader
	LayerContributor  libpak.DependencyLayerContributor
}

func NewJDK(dependency libpak.BuildModuleDependency, cache libpak.DependencyCache, certificateLoader CertificateLoader) (JDK, error) {
	expected := map[string]any{"dependency": dependency}

	if md, err := certificateLoader.Metadata(); err != nil {
		return JDK{}, fmt.Errorf("unable to generate certificate loader metadata")
	} else {
		for k, v := range md {
			expected[k] = v
		}
	}

	contributor := libpak.NewDependencyLayerContributor(
		dependency,
		cache,
		libcnb.LayerTypes{
			Build: true,
			Cache: true,
		},
		cache.Logger,
	)
	contributor.ExpectedMetadata = expected

	return JDK{
		CertificateLoader: certificateLoader,
		LayerContributor:  contributor,
	}, nil
}

func (j JDK) Contribute(layer *libcnb.Layer) error {
	return j.LayerContributor.Contribute(layer, func(layer *libcnb.Layer, artifact *os.File) error {
		j.LayerContributor.Logger.Bodyf("Expanding to %s", layer.Path)
		if err := crush.Extract(artifact, layer.Path, 1); err != nil {
			return fmt.Errorf("unable to expand JDK\n%w", err)
		}

		layer.BuildEnvironment.Override("JAVA_HOME", layer.Path)
		layer.BuildEnvironment.Override("JDK_HOME", layer.Path)

		var keyStorePath string
		if IsBeforeJava9(j.LayerContributor.Dependency.Version) {
			keyStorePath = filepath.Join(layer.Path, "jre", "lib", "security", "cacerts")
		} else {
			keyStorePath = filepath.Join(layer.Path, "lib", "security", "cacerts")
		}
		if err := os.Chmod(keyStorePath, 0664); err != nil {
			return fmt.Errorf("unable to set keystore file permissions\n%w", err)
		}

		if err := j.CertificateLoader.Load(keyStorePath, "changeit"); err != nil {
			return fmt.Errorf("unable to load certificates\n%w", err)
		}
		return nil
	})
}

func (j JDK) Name() string {
	return j.LayerContributor.LayerName()
}
