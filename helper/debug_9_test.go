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
	"os"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/paketo-buildpacks/libpak/v2/log"
	"github.com/sclevine/spec"

	"github.com/paketo-buildpacks/jvm-vendors/helper"
)

func testDebug9(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect
		d      = helper.Debug9{Logger: log.NewPaketoLogger(io.Discard)}
	)

	it("does nothing if $BPL_DEBUG_ENABLED is no set", func() {
		Expect(d.Execute()).To(BeNil())
	})

	context("$BPL_DEBUG_ENABLED", func() {
		var fakeIPv6File *os.File
		var fakeIPv6Path string

		it.Before(func() {
			t.Setenv("BPL_DEBUG_ENABLED", "true")

			fakeIPv6File, fakeIPv6FileErr := os.CreateTemp("", "IPv6Test")
			Expect(fakeIPv6FileErr).ToNot(HaveOccurred())

			_, err := fakeIPv6File.WriteString("0\n")
			Expect(err).ToNot(HaveOccurred())

			fakeIPv6Path = fakeIPv6File.Name()
			d.CustomIPv6CheckPath = fakeIPv6Path
		})

		it.After(func() {
			if fakeIPv6File != nil && fakeIPv6Path != "" {
				Expect(fakeIPv6File.Close()).To(Succeed())
				Expect(os.Remove(fakeIPv6Path)).To(Succeed())
			}
		})

		it("contributes configuration", func() {
			Expect(d.Execute()).To(Equal(map[string]string{
				"JAVA_TOOL_OPTIONS": "-agentlib:jdwp=transport=dt_socket,server=y,address=*:8000,suspend=n",
			}))
		})

		context("jdwp agent already configured", func() {
			it.Before(func() {
				t.Setenv("JAVA_TOOL_OPTIONS", "-agentlib:jdwp=something")
			})

			it("does not update JAVA_TOOL_OPTIONS", func() {
				Expect(d.Execute()).To(BeEmpty())
			})
		})

		context("$BPL_DEBUG_PORT", func() {
			it.Before(func() {
				t.Setenv("BPL_DEBUG_PORT", "8001")
			})

			it("contributes port configuration from $BPL_DEBUG_PORT", func() {
				Expect(d.Execute()).To(Equal(map[string]string{
					"JAVA_TOOL_OPTIONS": "-agentlib:jdwp=transport=dt_socket,server=y,address=*:8001,suspend=n",
				}))
			})
		})

		context("$BPL_DEBUG_SUSPEND", func() {
			it.Before(func() {
				t.Setenv("BPL_DEBUG_SUSPEND", "true")
			})

			it("contributes suspend configuration from $BPL_DEBUG_SUSPEND", func() {
				Expect(d.Execute()).To(Equal(map[string]string{
					"JAVA_TOOL_OPTIONS": "-agentlib:jdwp=transport=dt_socket,server=y,address=*:8000,suspend=y",
				}))
			})
		})

		context("$JAVA_TOOL_OPTIONS", func() {
			it.Before(func() {
				t.Setenv("JAVA_TOOL_OPTIONS", "test-java-tool-options")
			})

			it("contributes configuration appended to existing $JAVA_TOOL_OPTIONS", func() {
				Expect(d.Execute()).To(Equal(map[string]string{
					"JAVA_TOOL_OPTIONS": "test-java-tool-options -agentlib:jdwp=transport=dt_socket,server=y,address=*:8000,suspend=n",
				}))
			})
		})

		context("IPv6 is not present", func() {
			it.Before(func() {
				Expect(fakeIPv6Path).NotTo(BeEmpty())
				d1 := []byte("1\n")
				Expect(os.WriteFile(fakeIPv6Path, d1, 0600)).To(Succeed())
			})

			it.After(func() {
				Expect(fakeIPv6Path).NotTo(BeEmpty())
				d1 := []byte("0\n")
				Expect(os.WriteFile(fakeIPv6Path, d1, 0600)).To(Succeed())
			})

			it("replaces '*' host with IPv4 0.0.0.0", func() {
				Expect(d.Execute()).To(Equal(map[string]string{
					"JAVA_TOOL_OPTIONS": "-agentlib:jdwp=transport=dt_socket,server=y,address=0.0.0.0:8000,suspend=n",
				}))
			})
		})

		context("IPv6 kernel module file not there", func() {
			it.Before(func() {
				d.CustomIPv6CheckPath = "/does/not/exist"
			})

			it.After(func() {
				d.CustomIPv6CheckPath = fakeIPv6Path
			})

			it("replaces '*' host with IPv4 0.0.0.0", func() {
				Expect(d.Execute()).To(Equal(map[string]string{
					"JAVA_TOOL_OPTIONS": "-agentlib:jdwp=transport=dt_socket,server=y,address=0.0.0.0:8000,suspend=n",
				}))
			})
		})
	})
}
