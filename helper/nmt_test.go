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

package helper_test

import (
	"io"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/paketo-buildpacks/libpak/v2/log"
	"github.com/sclevine/spec"

	"github.com/paketo-buildpacks/jvm-vendors/helper"
)

func testNMT(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		n = helper.NMT{Logger: log.NewPaketoLogger(io.Discard)}
	)

	it("returns if $BPL_JAVA_NMT_ENABLED is set to false", func() {
		t.Setenv("BPL_JAVA_NMT_ENABLED", "false")
		Expect(n.Execute()).To(BeNil())
	})

	context("$BPL_JAVA_NMT_ENABLED", func() {
		it.Before(func() {
			t.Setenv("BPL_JAVA_NMT_ENABLED", "true")
		})

		it("contributes configuration for summary level", func() {
			t.Setenv("BPL_JAVA_NMT_LEVEL", "summary")
			Expect(n.Execute()).To(Equal(map[string]string{"NMT_LEVEL_1": "summary",
				"JAVA_TOOL_OPTIONS": "-XX:+UnlockDiagnosticVMOptions -XX:NativeMemoryTracking=summary -XX:+PrintNMTStatistics",
			}))
		})

		it("contributes configuration for detail level", func() {
			t.Setenv("BPL_JAVA_NMT_LEVEL", "detail")
			Expect(n.Execute()).To(Equal(map[string]string{"NMT_LEVEL_1": "detail",
				"JAVA_TOOL_OPTIONS": "-XX:+UnlockDiagnosticVMOptions -XX:NativeMemoryTracking=detail -XX:+PrintNMTStatistics",
			}))
		})

		context("$JAVA_TOOL_OPTIONS", func() {
			it.Before(func() {
				t.Setenv("JAVA_TOOL_OPTIONS", "test-java-tool-options")
			})

			it("contributes configuration appended to existing $JAVA_TOOL_OPTIONS - level defaults to summary", func() {
				Expect(n.Execute()).To(Equal(map[string]string{"NMT_LEVEL_1": "summary",
					"JAVA_TOOL_OPTIONS": "test-java-tool-options -XX:+UnlockDiagnosticVMOptions -XX:NativeMemoryTracking=summary -XX:+PrintNMTStatistics",
				}))
			})
		})
	})
}
