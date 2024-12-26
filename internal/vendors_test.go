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

package internal_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"

	"github.com/paketo-buildpacks/libpak/v2"

	"github.com/paketo-buildpacks/jvm-vendors/internal"
)

func testVendors(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect
	)

	it("fails if no vendors found", func() {
		cf := libpak.ConfigurationResolver{
			Configurations: []libpak.BuildModuleConfiguration{},
		}

		_, err := internal.LoadJvmVendors(&cf)
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError("BP_JVM_VENDORS is empty"))
	})

	it("load jvm vendors", func() {
		cf := libpak.ConfigurationResolver{
			Configurations: []libpak.BuildModuleConfiguration{
				{Name: "BP_JVM_VENDORS", Default: "adopt-openjdk,corretto"},
			},
		}

		vendors, err := internal.LoadJvmVendors(&cf)
		Expect(err).NotTo(HaveOccurred())
		Expect(vendors).To(HaveLen(2))
		Expect(vendors).To(ConsistOf("adopt-openjdk", "corretto"))
	})

	it("load jvm vendors and trim spaces", func() {
		cf := libpak.ConfigurationResolver{
			Configurations: []libpak.BuildModuleConfiguration{
				{Name: "BP_JVM_VENDORS", Default: " adopt-openjdk , corretto "},
			},
		}

		vendors, err := internal.LoadJvmVendors(&cf)
		Expect(err).NotTo(HaveOccurred())
		Expect(vendors).To(HaveLen(2))
		Expect(vendors).To(ConsistOf("adopt-openjdk", "corretto"))
	})
}
