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

package jvmvendors_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/buildpacks/libcnb/v2"
	. "github.com/onsi/gomega"
	"github.com/paketo-buildpacks/libpak/v2"
	"github.com/paketo-buildpacks/libpak/v2/log"
	"github.com/pavlo-v-chernykh/keystore-go/v4"
	"github.com/sclevine/spec"

	jvmvendors "github.com/paketo-buildpacks/jvm-vendors"
)

func testJDK(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		cl = jvmvendors.CertificateLoader{
			CertDirs: []string{filepath.Join("testdata", "certificates")},
			Logger:   log.NewDiscardLogger(),
		}

		ctx libcnb.BuildContext
	)

	it.Before(func() {
		ctx.Layers.Path = t.TempDir()
	})

	it.After(func() {
		Expect(os.RemoveAll(ctx.Layers.Path)).To(Succeed())
	})

	it("contributes JDK", func() {
		dep := libpak.BuildModuleDependency{
			Version: "11.0.0",
			URI:     "https://localhost/stub-jdk-11.tar.gz",
			SHA256:  "e40a6ddb7d74d78a6d5557380160a174b1273813db1caf9b1f7bcbfe1578e818",
		}
		dc := libpak.DependencyCache{CachePath: "testdata", Logger: log.NewDiscardLogger()}

		j, err := jvmvendors.NewJDK(dep, dc, cl)
		Expect(err).NotTo(HaveOccurred())

		Expect(j.LayerContributor.ExpectedMetadata.(map[string]any)["cert-dir"]).To(HaveLen(4))

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		err = j.Contribute(&layer)
		Expect(err).NotTo(HaveOccurred())

		Expect(layer.LayerTypes.Build).To(BeTrue())
		Expect(layer.LayerTypes.Cache).To(BeTrue())
		Expect(filepath.Join(layer.Path, "fixture-marker")).To(BeARegularFile())
		Expect(layer.BuildEnvironment["JAVA_HOME.override"]).To(Equal(layer.Path))
		Expect(layer.BuildEnvironment["JDK_HOME.override"]).To(Equal(layer.Path))
	})

	it("contributes JDK from a zip file", func() {
		dep := libpak.BuildModuleDependency{
			Version: "11.0.0",
			URI:     "https://localhost/stub-jdk-11.zip",
			SHA256:  "8138da5a0340f89b47ec4ab3fb5f12034ce793eb45af257820ec457316559a39",
		}
		dc := libpak.DependencyCache{CachePath: "testdata", Logger: log.NewDiscardLogger()}

		j, err := jvmvendors.NewJDK(dep, dc, cl)
		Expect(err).NotTo(HaveOccurred())

		Expect(j.LayerContributor.ExpectedMetadata.(map[string]any)["cert-dir"]).To(HaveLen(4))

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		err = j.Contribute(&layer)
		Expect(err).NotTo(HaveOccurred())

		Expect(layer.LayerTypes.Build).To(BeTrue())
		Expect(layer.LayerTypes.Cache).To(BeTrue())
		Expect(filepath.Join(layer.Path, "fixture-marker")).To(BeARegularFile())
		Expect(layer.BuildEnvironment["JAVA_HOME.override"]).To(Equal(layer.Path))
		Expect(layer.BuildEnvironment["JDK_HOME.override"]).To(Equal(layer.Path))
	})

	it("updates before Java 9 certificates", func() {
		dep := libpak.BuildModuleDependency{
			Version: "8.0.0",
			URI:     "https://localhost/stub-jdk-8.tar.gz",
			SHA256:  "6860fb9a9a66817ec285fac64c342b678b0810656b1f2413f063911a8bde6447",
		}
		dc := libpak.DependencyCache{CachePath: "testdata", Logger: log.NewDiscardLogger()}

		j, err := jvmvendors.NewJDK(dep, dc, cl)
		Expect(err).NotTo(HaveOccurred())

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		err = j.Contribute(&layer)
		Expect(err).NotTo(HaveOccurred())

		in, err := os.Open(filepath.Join(layer.Path, "jre", "lib", "security", "cacerts"))
		Expect(err).NotTo(HaveOccurred())
		defer in.Close()

		ks := keystore.New()
		err = ks.Load(in, []byte("changeit"))
		Expect(err).NotTo(HaveOccurred())
		Expect(ks.Aliases()).To(HaveLen(3))
	})

	it("updates after Java 9 certificates", func() {
		dep := libpak.BuildModuleDependency{
			Version: "11.0.0",
			URI:     "https://localhost/stub-jdk-11.tar.gz",
			SHA256:  "e40a6ddb7d74d78a6d5557380160a174b1273813db1caf9b1f7bcbfe1578e818",
		}
		dc := libpak.DependencyCache{CachePath: "testdata", Logger: log.NewDiscardLogger()}

		j, err := jvmvendors.NewJDK(dep, dc, cl)
		Expect(err).NotTo(HaveOccurred())

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		err = j.Contribute(&layer)
		Expect(err).NotTo(HaveOccurred())

		in, err := os.Open(filepath.Join(layer.Path, "lib", "security", "cacerts"))
		Expect(err).NotTo(HaveOccurred())
		defer in.Close()
		ks := keystore.New()
		err = ks.Load(in, []byte("changeit"))
		Expect(err).NotTo(HaveOccurred())
		Expect(ks.Aliases()).To(HaveLen(3))
	})
}
