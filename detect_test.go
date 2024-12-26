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
	"testing"

	"github.com/buildpacks/libcnb/v2"
	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"

	jvmvendors "github.com/paketo-buildpacks/jvm-vendors"
)

func testDetect(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		ctx libcnb.DetectContext
	)

	it("includes build plan options", func() {
		ctx.Buildpack.Metadata = map[string]any{
			"configurations": []map[string]any{
				{
					"name":    "BP_JVM_VENDORS",
					"default": "adopt-openjdk,corretto",
				},
				{
					"name":    "BP_JVM_VENDOR",
					"default": "adopt-openjdk",
				},
			},
		}

		Expect(jvmvendors.Detect(ctx)).To(Equal(libcnb.DetectResult{
			Pass: true,
			Plans: []libcnb.BuildPlan{
				{
					Provides: []libcnb.BuildPlanProvide{
						{Name: "jdk"},
						{Name: "native-image-builder"},
						{Name: "jre"},
					},
					Requires: []libcnb.BuildPlanRequire{
						{Name: "jdk"},
					},
				},
				{
					Provides: []libcnb.BuildPlanProvide{
						{Name: "jdk"},
						{Name: "native-image-builder"},
					},
					Requires: []libcnb.BuildPlanRequire{
						{Name: "jdk"},
					},
				},
				{
					Provides: []libcnb.BuildPlanProvide{
						{Name: "jdk"},
						{Name: "jre"},
					},
				},
				{
					Provides: []libcnb.BuildPlanProvide{
						{Name: "jdk"},
					},
				},
				{
					Provides: []libcnb.BuildPlanProvide{
						{Name: "jre"},
					},
				},
			},
		}))
	})

	it("fails if vendors don't load", func() {
		ctx.Buildpack.Metadata = map[string]any{
			"configurations": []map[string]any{},
		}

		_, err := jvmvendors.Detect(ctx)
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError("unable to load JVM vendors\nBP_JVM_VENDORS is empty"))
	})

	it("pass if vendor doesn't match", func() {
		ctx.Buildpack.Metadata = map[string]any{
			"configurations": []map[string]any{
				{
					"name":    "BP_JVM_VENDORS",
					"default": "adopt-openjdk,corretto",
				},
				{
					"name":    "BP_JVM_VENDOR",
					"default": "missing",
				},
			},
		}

		result, err := jvmvendors.Detect(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Pass).To(BeFalse())
	})
}
