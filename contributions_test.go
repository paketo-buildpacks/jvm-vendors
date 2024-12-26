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
	"testing"

	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"

	jvmvendors "github.com/paketo-buildpacks/jvm-vendors"
)

func testContributions(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect
	)

	it("identifies a build contribution", func() {
		Expect(jvmvendors.IsBuildContribution(map[string]any{"build": true})).To(BeTrue())
		Expect(jvmvendors.IsBuildContribution(map[string]any{"launch": true})).To(BeFalse())
	})

	it("identifies a launch contribution", func() {
		Expect(jvmvendors.IsLaunchContribution(map[string]any{"build": true})).To(BeFalse())
		Expect(jvmvendors.IsLaunchContribution(map[string]any{"launch": true})).To(BeTrue())
	})
}
