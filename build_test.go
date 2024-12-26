/*
 * Copyright 2018-2025 the original author or authors.
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
	"io"
	"testing"

	"github.com/paketo-buildpacks/libpak/v2"
	"github.com/paketo-buildpacks/libpak/v2/log"

	jvmvendors "github.com/paketo-buildpacks/jvm-vendors"

	"github.com/buildpacks/libcnb/v2"
	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"
)

func testBuild(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		ctx    libcnb.BuildContext
		result libcnb.BuildResult

		nativeOptionBundledWithJDK = jvmvendors.WithNativeImage(jvmvendors.NativeImage{
			BundledWithJDK: true,
		})

		nativeOptionSeparateFromJDK = jvmvendors.WithNativeImage(jvmvendors.NativeImage{
			BundledWithJDK: false,
			CustomCommand:  "/bin/gu",
			CustomArgs:     []string{"install", "--local-file"},
		})

		nativeOptionMissingCommand = jvmvendors.WithNativeImage(jvmvendors.NativeImage{
			BundledWithJDK: false,
			CustomCommand:  "",
			CustomArgs:     []string{"install", "--local-file"},
		})
	)

	it.Before(func() {
		t.Setenv("BP_ARCH", "amd64")

		ctx.Buildpack.Metadata = map[string]any{
			"configurations": []map[string]any{
				{
					"name":    "BP_JVM_VENDORS",
					"default": "adopt-openjdk,corretto",
				},
				{
					"name":    "BP_JVM_VENDOR",
					"default": "corretto",
				},
			},
		}
	})

	it("contributes JDK", func() {
		ctx.Plan.Entries = append(ctx.Plan.Entries, libcnb.BuildpackPlanEntry{Name: "jdk"})
		ctx.Buildpack.Metadata["dependencies"] = []map[string]any{
			{
				"id":      "jdk-corretto",
				"version": "1.1.1",
				"stacks":  []any{"test-stack-id"},
			},
		}
		ctx.StackID = "test-stack-id"

		contributors, err := jvmvendors.NewBuild(log.NewPaketoLogger(io.Discard)).Build(ctx, &result)
		Expect(err).NotTo(HaveOccurred())

		Expect(contributors).To(HaveLen(1))
		Expect(contributors[0].Name()).To(Equal("jdk-corretto"))
	})

	it("contributes JRE", func() {
		ctx.Plan.Entries = append(ctx.Plan.Entries, libcnb.BuildpackPlanEntry{Name: "jre", Metadata: LaunchContribution})
		ctx.Buildpack.API = "0.10"
		ctx.Buildpack.Metadata["dependencies"] = []map[string]any{
			{
				"id":      "jre-corretto",
				"version": "1.1.1",
				"stacks":  []any{"test-stack-id"},
			},
		}
		ctx.StackID = "test-stack-id"

		contributors, err := jvmvendors.NewBuild(log.NewPaketoLogger(io.Discard)).Build(ctx, &result)
		Expect(err).NotTo(HaveOccurred())

		Expect(contributors).To(HaveLen(3))
		Expect(contributors[0].Name()).To(Equal("jre-corretto"))
		Expect(contributors[1].Name()).To(Equal("helper"))
		Expect(contributors[2].Name()).To(Equal("java-security-properties"))
	})

	it("contributes security-providers-classpath-8 before Java 9", func() {
		ctx.Plan.Entries = append(ctx.Plan.Entries, libcnb.BuildpackPlanEntry{Name: "jre", Metadata: LaunchContribution})
		ctx.Buildpack.Metadata["dependencies"] = []map[string]any{
			{
				"id":      "jre-corretto",
				"version": "8.0.0",
				"stacks":  []any{"test-stack-id"},
			},
		}
		ctx.StackID = "test-stack-id"

		contributors, err := jvmvendors.NewBuild(log.NewPaketoLogger(io.Discard)).Build(ctx, &result)
		Expect(err).NotTo(HaveOccurred())

		Expect(contributors[0].Name()).To(Equal("jre-corretto"))
		Expect(contributors[1].Name()).To(Equal("helper"))

		Expect(contributors[1].(libpak.HelperLayerContributor).Names).To(Equal([]string{
			"active-processor-count",
			"java-opts",
			"jvm-heap",
			"link-local-dns",
			"memory-calculator",
			"security-providers-configurer",
			"jmx",
			"jfr",
			"openssl-certificate-loader",
			"security-providers-classpath-8",
			"debug-8",
		}))
	})

	it("contributes security-providers-classpath-9 after Java 9", func() {
		ctx.Plan.Entries = append(ctx.Plan.Entries, libcnb.BuildpackPlanEntry{Name: "jre", Metadata: LaunchContribution})
		ctx.Buildpack.Metadata["dependencies"] = []map[string]any{
			{
				"id":      "jre-corretto",
				"version": "11.0.0",
				"stacks":  []any{"test-stack-id"},
			},
		}
		ctx.StackID = "test-stack-id"

		contributors, err := jvmvendors.NewBuild(log.NewPaketoLogger(io.Discard)).Build(ctx, &result)
		Expect(err).NotTo(HaveOccurred())

		Expect(contributors).To(HaveLen(3))
		Expect(contributors[0].Name()).To(Equal("jre-corretto"))
		Expect(contributors[1].Name()).To(Equal("helper"))

		Expect(contributors[1].(libpak.HelperLayerContributor).Names).To(Equal([]string{
			"active-processor-count",
			"java-opts",
			"jvm-heap",
			"link-local-dns",
			"memory-calculator",
			"security-providers-configurer",
			"jmx",
			"jfr",
			"openssl-certificate-loader",
			"security-providers-classpath-9",
			"debug-9",
			"nmt",
		}))
	})

	it("contributes JDK when no JRE and only a JRE is wanted", func() {
		ctx.Plan.Entries = append(ctx.Plan.Entries, libcnb.BuildpackPlanEntry{Name: "jre", Metadata: LaunchContribution})
		ctx.Buildpack.API = "0.10"
		ctx.Buildpack.Metadata["dependencies"] = []map[string]any{
			{
				"id":      "jdk-corretto",
				"version": "1.1.1",
				"stacks":  []any{"test-stack-id"},
			},
		}
		ctx.StackID = "test-stack-id"

		contributors, err := jvmvendors.NewBuild(log.NewPaketoLogger(io.Discard)).Build(ctx, &result)
		Expect(err).NotTo(HaveOccurred())

		Expect(contributors).To(HaveLen(3))
		Expect(contributors[0].Name()).To(Equal("jdk-corretto"))
		Expect(contributors[0].(jvmvendors.JRE).LayerContributor.Dependency.ID).To(Equal("jdk-corretto"))
	})

	it("contributes JDK when no JRE and both a JDK and JRE are wanted", func() {
		ctx.Plan.Entries = append(ctx.Plan.Entries, libcnb.BuildpackPlanEntry{Name: "jdk", Metadata: LaunchContribution})
		ctx.Plan.Entries = append(ctx.Plan.Entries, libcnb.BuildpackPlanEntry{Name: "jre", Metadata: LaunchContribution})
		ctx.Buildpack.API = "0.10"
		ctx.Buildpack.Metadata["dependencies"] = []map[string]any{
			{
				"id":      "jdk-corretto",
				"version": "1.1.1",
				"stacks":  []any{"test-stack-id"},
			},
		}
		ctx.StackID = "test-stack-id"

		contributors, err := jvmvendors.NewBuild(log.NewPaketoLogger(io.Discard)).Build(ctx, &result)
		Expect(err).NotTo(HaveOccurred())

		Expect(contributors).To(HaveLen(3))
		Expect(contributors[0].Name()).To(Equal("jdk-corretto"))
		Expect(contributors[0].(jvmvendors.JRE).LayerContributor.Dependency.ID).To(Equal("jdk-corretto"))
	})

	it("fails when there is an issue loading JVM vendors", func() {
		ctx.Plan.Entries = append(ctx.Plan.Entries, libcnb.BuildpackPlanEntry{Name: "jdk", Metadata: LaunchContribution})
		ctx.Buildpack.Metadata["configurations"] = []map[string]any{}

		_, err := jvmvendors.NewBuild(log.NewPaketoLogger(io.Discard)).Build(ctx, &result)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unable to load JVM vendors"))
	})

	context("native image", func() {
		it.Before(func() {
			ctx.Buildpack.Metadata = map[string]any{
				"configurations": []map[string]any{
					{
						"name":    "BP_JVM_VENDORS",
						"default": "bellsoft-liberica,graalvm",
					},
					{
						"name":    "BP_JVM_VENDOR",
						"default": "graalvm",
					},
				},
			}
		})

		it("contributes NIK API", func() {
			ctx.Plan.Entries = append(
				ctx.Plan.Entries,
				libcnb.BuildpackPlanEntry{Name: "jdk", Metadata: map[string]any{}},
				libcnb.BuildpackPlanEntry{Name: "native-image-builder"},
			)
			ctx.Buildpack.Metadata["dependencies"] = []map[string]any{
				{
					"id":      "native-image-svm-graalvm",
					"version": "1.1.1",
					"stacks":  []interface{}{"test-stack-id"},
					"cpes":    []interface{}{"cpe:2.3:a:bellsoft:nik:1.1.1:*:*:*:*:*:*:*"},
					"purl":    "pkg:generic/provider-nik@1.1.1?arch=amd64",
				},
			}
			ctx.Buildpack.API = "0.10"
			ctx.StackID = "test-stack-id"

			contributors, err := jvmvendors.NewBuild(log.NewPaketoLogger(io.Discard), nativeOptionBundledWithJDK).Build(ctx, &result)
			Expect(err).NotTo(HaveOccurred())

			Expect(contributors).To(HaveLen(1))
			Expect(contributors[0].Name()).To(Equal("native-image-svm-graalvm"))
		})

		context("native image enabled (not bundled with JDK)", func() {
			it("contributes native image dependency", func() {
				ctx.Plan.Entries = append(ctx.Plan.Entries,
					libcnb.BuildpackPlanEntry{
						Name: "jdk",
					},
					libcnb.BuildpackPlanEntry{
						Name: "native-image-builder",
					},
				)
				ctx.Buildpack.Metadata["dependencies"] = []map[string]any{
					{
						"id":      "jdk-corretto",
						"version": "1.1.1",
						"stacks":  []interface{}{"test-stack-id"},
						"cpes":    []string{"cpe:2.3:a:oracle:graalvm:21.2.0:*:*:*:community:*:*:*"},
						"purl":    "pkg:generic/graalvm-jdk@21.2.0",
					},
					{
						"id":      "native-image-svm-graalvm",
						"version": "2.2.2",
						"stacks":  []any{"test-stack-id"},
						"cpes":    []string{"cpe:2.3:a:oracle:graalvm:21.2.0:*:*:*:community:*:*:*"},
						"purl":    "pkg:generic/graalvm-svm@21.2.0",
					},
				}
				ctx.StackID = "test-stack-id"
				ctx.Buildpack.API = "0.10"

				contributors, err := jvmvendors.NewBuild(log.NewPaketoLogger(io.Discard), nativeOptionSeparateFromJDK).Build(ctx, &result)
				Expect(err).NotTo(HaveOccurred())

				Expect(contributors).To(HaveLen(1))
				Expect(contributors[0].Name()).To(Equal("nik"))
				Expect(contributors[0].(jvmvendors.NIK).NativeDependency).NotTo(BeNil())
			})
		})

		context("native image enabled (not bundled with JDK) - custom command missing", func() {
			it("contributes native image dependency", func() {
				ctx.Plan.Entries = append(ctx.Plan.Entries,
					libcnb.BuildpackPlanEntry{
						Name: "jdk",
					},
					libcnb.BuildpackPlanEntry{
						Name: "native-image-builder",
					},
				)
				ctx.Buildpack.Metadata["dependencies"] = []map[string]any{
					{
						"id":      "jdk-corretto",
						"version": "1.1.1",
						"stacks":  []interface{}{"test-stack-id"},
						"cpes":    []string{"cpe:2.3:a:oracle:graalvm:21.2.0:*:*:*:community:*:*:*"},
						"purl":    "pkg:generic/graalvm-jdk@21.2.0",
					},
					{
						"id":      "native-image-svm-graalvm",
						"version": "2.2.2",
						"stacks":  []any{"test-stack-id"},
						"cpes":    []string{"cpe:2.3:a:oracle:graalvm:21.2.0:*:*:*:community:*:*:*"},
						"purl":    "pkg:generic/graalvm-svm@21.2.0",
					},
				}
				ctx.StackID = "test-stack-id"
				ctx.Buildpack.API = "0.10"

				_, err := jvmvendors.NewBuild(log.NewPaketoLogger(io.Discard), nativeOptionMissingCommand).Build(ctx, &result)
				Expect(err).To(HaveOccurred())

				Expect(err.Error()).To(ContainSubstring("unable to create NIK, custom command has not been supplied by buildpack"))
			})
		})

		it("contributes NIK alternative buildplan (NIK bundled with JDK)", func() {
			// NIK includes a JDK, so we don't need a second JDK
			ctx.Plan.Entries = append(
				ctx.Plan.Entries,
				libcnb.BuildpackPlanEntry{Name: "native-image-builder"},
				libcnb.BuildpackPlanEntry{Name: "jdk", Metadata: map[string]any{}},
				libcnb.BuildpackPlanEntry{Name: "jre", Metadata: map[string]any{}})
			ctx.Buildpack.Metadata["dependencies"] = []map[string]any{
				{
					"id":      "native-image-svm-graalvm",
					"version": "1.1.1",
					"stacks":  []any{"test-stack-id"},
				},
			}
			ctx.Buildpack.API = "0.10"
			ctx.StackID = "test-stack-id"

			contributors, err := jvmvendors.NewBuild(log.NewPaketoLogger(io.Discard), nativeOptionBundledWithJDK).Build(ctx, &result)
			Expect(err).NotTo(HaveOccurred())

			Expect(contributors).To(HaveLen(1))
			Expect(contributors[0].Name()).To(Equal("native-image-svm-graalvm"))
		})
	})

	context("$BP_JVM_VERSION", func() {
		it.Before(func() {
			t.Setenv("BP_JVM_VERSION", "1.1.1")
		})

		it("selects versions based on BP_JVM_VERSION", func() {
			ctx.Plan.Entries = append(ctx.Plan.Entries,
				libcnb.BuildpackPlanEntry{Name: "jdk"},
				libcnb.BuildpackPlanEntry{Name: "jre"},
			)
			ctx.Buildpack.Metadata["dependencies"] = []map[string]any{
				{
					"id":      "jdk-corretto",
					"version": "1.1.1",
					"stacks":  []any{"test-stack-id"},
				},
				{
					"id":      "jdk-corretto",
					"version": "2.2.2",
					"stacks":  []any{"test-stack-id"},
				},
				{
					"id":      "jre-corretto",
					"version": "1.1.1",
					"stacks":  []any{"test-stack-id"},
				},
				{
					"id":      "jre-corretto",
					"version": "2.2.2",
					"stacks":  []any{"test-stack-id"},
				},
			}
			ctx.StackID = "test-stack-id"

			contributors, err := jvmvendors.NewBuild(log.NewPaketoLogger(io.Discard)).Build(ctx, &result)
			Expect(err).NotTo(HaveOccurred())

			Expect(contributors).To(HaveLen(2))
			Expect(contributors[0].Name()).To(Equal("jdk-corretto"))
			Expect(contributors[0].(jvmvendors.JDK).LayerContributor.Dependency.Version).To(Equal("1.1.1"))
			Expect(contributors[1].(jvmvendors.JRE).LayerContributor.Dependency.Version).To(Equal("1.1.1"))
		})
	})

	context("$BP_JVM_TYPE", func() {
		it("contributes JDK when specified explicitly in $BP_JVM_TYPE", func() {
			t.Setenv("BP_JVM_TYPE", "jdk")

			ctx.Plan.Entries = append(ctx.Plan.Entries, libcnb.BuildpackPlanEntry{Name: "jdk", Metadata: LaunchContribution})
			ctx.Plan.Entries = append(ctx.Plan.Entries, libcnb.BuildpackPlanEntry{Name: "jre", Metadata: LaunchContribution})
			ctx.Buildpack.Metadata["dependencies"] = []map[string]any{
				{
					"id":      "jdk-corretto",
					"version": "0.0.2",
					"stacks":  []any{"test-stack-id"},
				},
				{
					"id":      "jre-corretto",
					"version": "2.2.2",
					"stacks":  []any{"test-stack-id"},
				},
			}
			ctx.StackID = "test-stack-id"

			contributors, err := jvmvendors.NewBuild(log.NewPaketoLogger(io.Discard)).Build(ctx, &result)
			Expect(err).NotTo(HaveOccurred())

			Expect(contributors).To(HaveLen(3))
			Expect(contributors[0].Name()).To(Equal("jdk-corretto"))
			Expect(contributors[0].(jvmvendors.JRE).LayerContributor.Dependency.ID).To(Equal("jdk-corretto"))
		})

		it("contributes JRE when specified explicitly in $BP_JVM_TYPE", func() {
			t.Setenv("BP_JVM_TYPE", "jre")

			ctx.Plan.Entries = append(ctx.Plan.Entries, libcnb.BuildpackPlanEntry{Name: "jdk", Metadata: LaunchContribution})
			ctx.Plan.Entries = append(ctx.Plan.Entries, libcnb.BuildpackPlanEntry{Name: "jre", Metadata: LaunchContribution})
			ctx.Buildpack.Metadata["dependencies"] = []map[string]any{
				{
					"id":      "jdk-corretto",
					"version": "0.0.1",
					"stacks":  []any{"test-stack-id"},
				}, {
					"id":      "jre-corretto",
					"version": "1.1.1",
					"stacks":  []any{"test-stack-id"},
				},
			}
			ctx.StackID = "test-stack-id"

			contributors, err := jvmvendors.NewBuild(log.NewPaketoLogger(io.Discard)).Build(ctx, &result)
			Expect(err).NotTo(HaveOccurred())

			Expect(contributors).To(HaveLen(4))
			Expect(contributors[0].Name()).To(Equal("jdk-corretto"))
			Expect(contributors[0].(jvmvendors.JDK).LayerContributor.Dependency.ID).To(Equal("jdk-corretto"))
			Expect(contributors[1].(jvmvendors.JRE).LayerContributor.Dependency.ID).To(Equal("jre-corretto"))
		})
	})
}
