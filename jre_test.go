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

func testJRE(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		cl = jvmvendors.CertificateLoader{
			CertDirs: []string{filepath.Join("testdata", "certificates")},
			Logger:   log.NewDiscardLogger(),
		}

		ctx libcnb.BuildContext
	)

	it.Before(func() {
		ctx.ApplicationPath = t.TempDir()
		ctx.Layers.Path = t.TempDir()
	})

	it.After(func() {
		Expect(os.RemoveAll(ctx.ApplicationPath)).To(Succeed())
		Expect(os.RemoveAll(ctx.Layers.Path)).To(Succeed())
	})

	it("contributes JRE", func() {
		dep := libpak.BuildModuleDependency{
			Version: "11.0.0",
			URI:     "https://localhost/stub-jre-11.tar.gz",
			SHA256:  "3aa01010c0d3592ea248c8353d60b361231fa9bf9a7479b4f06451fef3e64524",
		}
		dc := libpak.DependencyCache{CachePath: "testdata", Logger: log.NewDiscardLogger()}

		j, err := jvmvendors.NewJRE(ctx.ApplicationPath, dep, dc, jvmvendors.JREType, cl, NoContribution)
		Expect(err).NotTo(HaveOccurred())

		Expect(j.LayerContributor.ExpectedMetadata.(map[string]any)["cert-dir"]).To(HaveLen(4))

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		err = j.Contribute(&layer)
		Expect(err).NotTo(HaveOccurred())

		Expect(filepath.Join(layer.Path, "fixture-marker")).To(BeARegularFile())
	})

	it("contributes JRE from a zip file", func() {
		dep := libpak.BuildModuleDependency{
			Version: "11.0.0",
			URI:     "https://localhost/stub-jre-11.zip",
			SHA256:  "e3b22e738f6e956ef576215b39d79d321157f1d3de3bddf9c4120ae0444bdba8",
		}
		dc := libpak.DependencyCache{CachePath: "testdata", Logger: log.NewDiscardLogger()}

		j, err := jvmvendors.NewJRE(ctx.ApplicationPath, dep, dc, jvmvendors.JREType, cl, NoContribution)
		Expect(err).NotTo(HaveOccurred())

		Expect(j.LayerContributor.ExpectedMetadata.(map[string]any)["cert-dir"]).To(HaveLen(4))

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		err = j.Contribute(&layer)
		Expect(err).NotTo(HaveOccurred())

		Expect(filepath.Join(layer.Path, "fixture-marker")).To(BeARegularFile())
	})

	it("updates JRE certificates", func() {
		dep := libpak.BuildModuleDependency{
			Version: "11.0.0",
			URI:     "https://localhost/stub-jre-11.tar.gz",
			SHA256:  "3aa01010c0d3592ea248c8353d60b361231fa9bf9a7479b4f06451fef3e64524",
		}
		dc := libpak.DependencyCache{CachePath: "testdata", Logger: log.NewDiscardLogger()}

		j, err := jvmvendors.NewJRE(ctx.ApplicationPath, dep, dc, jvmvendors.JREType, cl, NoContribution)
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

	it("updates before Java 9 JDK certificates", func() {
		dep := libpak.BuildModuleDependency{
			Version: "8.0.0",
			URI:     "https://localhost/stub-jdk-8.tar.gz",
			SHA256:  "6860fb9a9a66817ec285fac64c342b678b0810656b1f2413f063911a8bde6447",
		}
		dc := libpak.DependencyCache{CachePath: "testdata", Logger: log.NewDiscardLogger()}

		j, err := jvmvendors.NewJRE(ctx.ApplicationPath, dep, dc, jvmvendors.JDKType, cl, NoContribution)
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

	it("updates after Java 9 JDK certificates", func() {
		dep := libpak.BuildModuleDependency{
			Version: "11.0.0",
			URI:     "https://localhost/stub-jdk-11.tar.gz",
			SHA256:  "e40a6ddb7d74d78a6d5557380160a174b1273813db1caf9b1f7bcbfe1578e818",
		}
		dc := libpak.DependencyCache{CachePath: "testdata", Logger: log.NewDiscardLogger()}

		j, err := jvmvendors.NewJRE(ctx.ApplicationPath, dep, dc, jvmvendors.JDKType, cl, NoContribution)
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

	it("marks layer for build", func() {
		dep := libpak.BuildModuleDependency{
			Version: "11.0.0",
			URI:     "https://localhost/stub-jre-11.tar.gz",
			SHA256:  "3aa01010c0d3592ea248c8353d60b361231fa9bf9a7479b4f06451fef3e64524",
		}
		dc := libpak.DependencyCache{CachePath: "testdata", Logger: log.NewDiscardLogger()}

		j, err := jvmvendors.NewJRE(ctx.ApplicationPath, dep, dc, jvmvendors.JREType, cl, BuildContribution)
		Expect(err).NotTo(HaveOccurred())

		Expect(j.LayerContributor.ExpectedMetadata.(map[string]any)["cert-dir"]).To(HaveLen(4))

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		err = j.Contribute(&layer)
		Expect(err).NotTo(HaveOccurred())

		Expect(layer.LayerTypes.Build).To(BeTrue())
		Expect(layer.LayerTypes.Cache).To(BeTrue())
		Expect(layer.BuildEnvironment["JAVA_HOME.default"]).To(Equal(layer.Path))
	})

	it("marks before Java 9 JRE layer for launch", func() {
		dep := libpak.BuildModuleDependency{
			Version: "8.0.0",
			URI:     "https://localhost/stub-jre-8.tar.gz",
			SHA256:  "bb4f0e8cbeec6802ab8e599c83c2fb835f0da9b9213c463102f9092e4f8afdda",
		}
		dc := libpak.DependencyCache{CachePath: "testdata", Logger: log.NewDiscardLogger()}

		j, err := jvmvendors.NewJRE(ctx.ApplicationPath, dep, dc, jvmvendors.JREType, cl, LaunchContribution)
		Expect(err).NotTo(HaveOccurred())

		Expect(j.LayerContributor.ExpectedMetadata.(map[string]any)["cert-dir"]).To(HaveLen(4))

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		err = j.Contribute(&layer)
		Expect(err).NotTo(HaveOccurred())

		Expect(layer.LayerTypes.Launch).To(BeTrue())
		Expect(layer.LaunchEnvironment["BPI_APPLICATION_PATH.default"]).To(Equal(ctx.ApplicationPath))
		Expect(layer.LaunchEnvironment["BPI_JVM_CACERTS.default"]).To(Equal(filepath.Join(layer.Path, "lib", "security", "cacerts")))
		Expect(layer.LaunchEnvironment["BPI_JVM_CLASS_COUNT.default"]).To(Equal("0"))
		Expect(layer.LaunchEnvironment["BPI_JVM_EXT_DIR.default"]).To(Equal(filepath.Join(layer.Path, "lib", "ext")))
		Expect(layer.LaunchEnvironment["BPI_JVM_SECURITY_PROVIDERS.default"]).To(Equal("1|ALPHA"))
		Expect(layer.LaunchEnvironment["JAVA_HOME.default"]).To(Equal(layer.Path))
		Expect(layer.LaunchEnvironment["MALLOC_ARENA_MAX.default"]).To(Equal("2"))
		Expect(layer.LaunchEnvironment["JAVA_TOOL_OPTIONS.delim"]).To(Equal(" "))
		Expect(layer.LaunchEnvironment["JAVA_TOOL_OPTIONS.append"]).To(Equal("-XX:+ExitOnOutOfMemoryError"))
	})

	it("marks after Java 9 JRE layer for launch", func() {
		dep := libpak.BuildModuleDependency{
			Version: "11.0.0",
			URI:     "https://localhost/stub-jre-11.tar.gz",
			SHA256:  "3aa01010c0d3592ea248c8353d60b361231fa9bf9a7479b4f06451fef3e64524",
		}
		dc := libpak.DependencyCache{CachePath: "testdata", Logger: log.NewDiscardLogger()}

		j, err := jvmvendors.NewJRE(ctx.ApplicationPath, dep, dc, jvmvendors.JREType, cl, LaunchContribution)
		Expect(err).NotTo(HaveOccurred())

		Expect(j.LayerContributor.ExpectedMetadata.(map[string]any)["cert-dir"]).To(HaveLen(4))

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		err = j.Contribute(&layer)
		Expect(err).NotTo(HaveOccurred())

		Expect(layer.LayerTypes.Launch).To(BeTrue())
		Expect(layer.LaunchEnvironment["BPI_APPLICATION_PATH.default"]).To(Equal(ctx.ApplicationPath))
		Expect(layer.LaunchEnvironment["BPI_JVM_CACERTS.default"]).To(Equal(filepath.Join(layer.Path, "lib", "security", "cacerts")))
		Expect(layer.LaunchEnvironment["BPI_JVM_CLASS_COUNT.default"]).To(Equal("0"))
		Expect(layer.LaunchEnvironment["BPI_JVM_SECURITY_PROVIDERS.default"]).To(Equal("1|ALPHA"))
		Expect(layer.LaunchEnvironment["JAVA_HOME.default"]).To(Equal(layer.Path))
		Expect(layer.LaunchEnvironment["MALLOC_ARENA_MAX.default"]).To(Equal("2"))
		Expect(layer.LaunchEnvironment["JAVA_TOOL_OPTIONS.delim"]).To(Equal(" "))
		Expect(layer.LaunchEnvironment["JAVA_TOOL_OPTIONS.append"]).To(Equal("-XX:+ExitOnOutOfMemoryError"))
	})

	it("marks before Java 9 JDK layer for launch", func() {
		dep := libpak.BuildModuleDependency{
			Version: "8.0.0",
			URI:     "https://localhost/stub-jdk-8.tar.gz",
			SHA256:  "6860fb9a9a66817ec285fac64c342b678b0810656b1f2413f063911a8bde6447",
		}
		dc := libpak.DependencyCache{CachePath: "testdata", Logger: log.NewDiscardLogger()}

		j, err := jvmvendors.NewJRE(ctx.ApplicationPath, dep, dc, jvmvendors.JDKType, cl, LaunchContribution)
		Expect(err).NotTo(HaveOccurred())

		Expect(j.LayerContributor.ExpectedMetadata.(map[string]any)["cert-dir"]).To(HaveLen(4))

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		err = j.Contribute(&layer)
		Expect(err).NotTo(HaveOccurred())

		Expect(layer.LayerTypes.Launch).To(BeTrue())
		Expect(layer.LaunchEnvironment["BPI_APPLICATION_PATH.default"]).To(Equal(ctx.ApplicationPath))
		Expect(layer.LaunchEnvironment["BPI_JVM_CACERTS.default"]).To(Equal(filepath.Join(layer.Path, "jre", "lib", "security", "cacerts")))
		Expect(layer.LaunchEnvironment["BPI_JVM_CLASS_COUNT.default"]).To(Equal("0"))
		Expect(layer.LaunchEnvironment["BPI_JVM_EXT_DIR.default"]).To(Equal(filepath.Join(layer.Path, "jre", "lib", "ext")))
		Expect(layer.LaunchEnvironment["BPI_JVM_SECURITY_PROVIDERS.default"]).To(Equal("1|ALPHA"))
		Expect(layer.LaunchEnvironment["JAVA_HOME.default"]).To(Equal(layer.Path))
		Expect(layer.LaunchEnvironment["MALLOC_ARENA_MAX.default"]).To(Equal("2"))
		Expect(layer.LaunchEnvironment["JAVA_TOOL_OPTIONS.delim"]).To(Equal(" "))
		Expect(layer.LaunchEnvironment["JAVA_TOOL_OPTIONS.append"]).To(Equal("-XX:+ExitOnOutOfMemoryError"))
	})

	it("marks after Java 9 JDK layer for launch", func() {
		dep := libpak.BuildModuleDependency{
			Version: "11.0.0",
			URI:     "https://localhost/stub-jdk-11.tar.gz",
			SHA256:  "e40a6ddb7d74d78a6d5557380160a174b1273813db1caf9b1f7bcbfe1578e818",
		}
		dc := libpak.DependencyCache{CachePath: "testdata", Logger: log.NewDiscardLogger()}

		j, err := jvmvendors.NewJRE(ctx.ApplicationPath, dep, dc, jvmvendors.JDKType, cl, LaunchContribution)
		Expect(err).NotTo(HaveOccurred())

		Expect(j.LayerContributor.ExpectedMetadata.(map[string]any)["cert-dir"]).To(HaveLen(4))

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		err = j.Contribute(&layer)
		Expect(err).NotTo(HaveOccurred())

		Expect(layer.LayerTypes.Launch).To(BeTrue())
		Expect(layer.LaunchEnvironment["BPI_APPLICATION_PATH.default"]).To(Equal(ctx.ApplicationPath))
		Expect(layer.LaunchEnvironment["BPI_JVM_CACERTS.default"]).To(Equal(filepath.Join(layer.Path, "lib", "security", "cacerts")))
		Expect(layer.LaunchEnvironment["BPI_JVM_CLASS_COUNT.default"]).To(Equal("0"))
		Expect(layer.LaunchEnvironment["BPI_JVM_SECURITY_PROVIDERS.default"]).To(Equal("1|ALPHA"))
		Expect(layer.LaunchEnvironment["JAVA_HOME.default"]).To(Equal(layer.Path))
		Expect(layer.LaunchEnvironment["MALLOC_ARENA_MAX.default"]).To(Equal("2"))
		Expect(layer.LaunchEnvironment["JAVA_TOOL_OPTIONS.delim"]).To(Equal(" "))
		Expect(layer.LaunchEnvironment["JAVA_TOOL_OPTIONS.append"]).To(Equal("-XX:+ExitOnOutOfMemoryError"))
	})
}
