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

func testVersions(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect
	)

	it("determines whether a version is before Java 9", func() {
		Expect(jvmvendors.IsBeforeJava9("8.0.0")).To(BeTrue())
		Expect(jvmvendors.IsBeforeJava9("9.0.0")).To(BeFalse())
		Expect(jvmvendors.IsBeforeJava9("11.0.0")).To(BeFalse())
		Expect(jvmvendors.IsBeforeJava9("")).To(BeFalse())
	})

	it("determines whether a version is before Java 18", func() {
		Expect(jvmvendors.IsBeforeJava18("17.0.0")).To(BeTrue())
		Expect(jvmvendors.IsBeforeJava18("18.0.0")).To(BeFalse())
		Expect(jvmvendors.IsBeforeJava18("19.0.0")).To(BeFalse())
		Expect(jvmvendors.IsBeforeJava18("")).To(BeFalse())
	})

	it("determines whether a version is before Java 17", func() {
		Expect(jvmvendors.IsBeforeJava17("16.0.0")).To(BeTrue())
		Expect(jvmvendors.IsBeforeJava17("17.0.0")).To(BeFalse())
		Expect(jvmvendors.IsBeforeJava17("18.0.0")).To(BeFalse())
		Expect(jvmvendors.IsBeforeJava17("")).To(BeFalse())
	})

	it("determines whether a version is Java 25 or later", func() {
		Expect(jvmvendors.IsJava25OrLater("24.0.0")).To(BeFalse())
		Expect(jvmvendors.IsJava25OrLater("25.0.0")).To(BeTrue())
		Expect(jvmvendors.IsJava25OrLater("26.0.0")).To(BeTrue())
		Expect(jvmvendors.IsJava25OrLater("11.0.0")).To(BeFalse())
		Expect(jvmvendors.IsJava25OrLater("")).To(BeFalse())
	})
}
