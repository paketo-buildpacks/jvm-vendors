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
	"fmt"
	"io"
	"os"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"

	"github.com/paketo-buildpacks/libpak/v2/log"

	"github.com/paketo-buildpacks/jvm-vendors/helper"
)

func testJVMHeapDump(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect       = NewWithT(t).Expect
		HeapDumpPath string
	)

	it.Before(func() {
		HeapDumpPath = t.TempDir()
	})

	it.After(func() {
		Expect(os.RemoveAll(HeapDumpPath)).To(Succeed())
	})

	it("does nothing without $BPL_HEAP_DUMP_PATH being set", func() {
		env, err := helper.JVMHeapDump{}.Execute()
		Expect(err).ToNot(HaveOccurred())
		Expect(env).To(BeNil())
	})

	context("$BPL_HEAP_DUMP_PATH is set", func() {
		it.Before(func() {
			Expect(os.Setenv("BPL_HEAP_DUMP_PATH", HeapDumpPath)).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("BPL_HEAP_DUMP_PATH")).To(Succeed())
		})

		context("no $JAVA_TOOL_OPTIONS", func() {
			it("enables heap dumps", func() {
				env, err := helper.JVMHeapDump{Logger: log.NewPaketoLogger(io.Discard)}.Execute()
				Expect(err).ToNot(HaveOccurred())
				Expect(env).To(HaveKeyWithValue("JAVA_TOOL_OPTIONS",
					ContainSubstring(`-XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=`)))
			})
		})

		context("unrelated $JAVA_TOOL_OPTIONS", func() {
			it.Before(func() {
				Expect(os.Setenv("JAVA_TOOL_OPTIONS", "-Xmx2G -Xss256k")).To(Succeed())
			})

			it.After(func() {
				Expect(os.Unsetenv("JAVA_TOOL_OPTIONS")).To(Succeed())
			})

			it("passes through existing options and appends heap dump options", func() {
				env, err := helper.JVMHeapDump{Logger: log.NewPaketoLogger(io.Discard)}.Execute()
				Expect(err).ToNot(HaveOccurred())
				Expect(env).To(HaveKeyWithValue("JAVA_TOOL_OPTIONS",
					ContainSubstring(`-Xmx2G -Xss256k -XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=`)))
			})
		})

		context("dump enabled already in $JAVA_TOOL_OPTIONS", func() {
			it.Before(func() {
				Expect(os.Setenv("JAVA_TOOL_OPTIONS", "-Xmx2G -Xss256k -XX:+HeapDumpOnOutOfMemoryError")).To(Succeed())
			})

			it.After(func() {
				Expect(os.Unsetenv("JAVA_TOOL_OPTIONS")).To(Succeed())
			})

			it("passes through existing options and appends heap dump path option", func() {
				env, err := helper.JVMHeapDump{Logger: log.NewPaketoLogger(io.Discard)}.Execute()
				Expect(err).ToNot(HaveOccurred())
				Expect(env).To(HaveKeyWithValue("JAVA_TOOL_OPTIONS",
					ContainSubstring(`-Xmx2G -Xss256k -XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=`)))
			})
		})

		context("dump and path enabled already in $JAVA_TOOL_OPTIONS", func() {
			var expectedArgs string

			it.Before(func() {
				expectedArgs = fmt.Sprintf("-Xmx2G -Xss256k -XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=%s", HeapDumpPath)
				Expect(os.Setenv("JAVA_TOOL_OPTIONS", expectedArgs)).To(Succeed())
			})

			it.After(func() {
				Expect(os.Unsetenv("JAVA_TOOL_OPTIONS")).To(Succeed())
			})

			it("passes through existing options and appends heap dump options", func() {
				env, err := helper.JVMHeapDump{Logger: log.NewPaketoLogger(io.Discard)}.Execute()
				Expect(err).ToNot(HaveOccurred())
				Expect(env).To(Equal(map[string]string{
					"JAVA_TOOL_OPTIONS": expectedArgs,
				}))
			})
		})
	})
}
