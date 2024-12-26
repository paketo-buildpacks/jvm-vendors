/*
 * Copyright 2018-2022 the original author or authors.
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

	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"

	jvmvendors "github.com/paketo-buildpacks/jvm-vendors"
)

func testSDKMAN(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		path string
	)

	it.Before(func() {
		path = t.TempDir()
	})

	it.After(func() {
		Expect(os.RemoveAll(path)).To(Succeed())
	})

	it("parses single entry sdkmanrc file", func() {
		sdkmanrcFile := filepath.Join(path, "sdkmanrc")
		Expect(os.WriteFile(sdkmanrcFile, []byte(`java=17.0.2-tem`), 0600)).To(Succeed())

		res, err := jvmvendors.ReadSDKMANRC(sdkmanrcFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal([]jvmvendors.SDKInfo{
			{Type: "java", Version: "17.0.2", Vendor: "tem"},
		}))
	})

	it("parses single entry sdkmanrc file and forces lowercase", func() {
		sdkmanrcFile := filepath.Join(path, "sdkmanrc")
		Expect(os.WriteFile(sdkmanrcFile, []byte(` jAva = 17.0.2-TEM `), 0600)).To(Succeed())

		res, err := jvmvendors.ReadSDKMANRC(sdkmanrcFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal([]jvmvendors.SDKInfo{
			{Type: "java", Version: "17.0.2", Vendor: "tem"},
		}))
	})

	it("parses multiple entry sdkmanrc and doesn't care if there's overlap", func() {
		sdkmanrcFile := filepath.Join(path, "sdkmanrc")
		Expect(os.WriteFile(sdkmanrcFile, []byte(`java=11.0.2-tem
java=17.0.2-tem`), 0600)).To(Succeed())

		res, err := jvmvendors.ReadSDKMANRC(sdkmanrcFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal([]jvmvendors.SDKInfo{
			{Type: "java", Version: "11.0.2", Vendor: "tem"},
			{Type: "java", Version: "17.0.2", Vendor: "tem"},
		}))
	})

	context("handles comments and whitespace", func() {
		it("ignores full-line comments", func() {
			sdkmanrcFile := filepath.Join(path, "sdkmanrc")
			Expect(os.WriteFile(sdkmanrcFile, []byte(`# Enable auto-env through the sdkman_auto_env config
# Add key=value pairs of SDKs to use below
java=17.0.2-tem
    # has some leading whitespace`), 0600)).To(Succeed())

			res, err := jvmvendors.ReadSDKMANRC(sdkmanrcFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal([]jvmvendors.SDKInfo{
				{Type: "java", Version: "17.0.2", Vendor: "tem"},
			}))
		})

		it("ignores trailing-line comments", func() {
			sdkmanrcFile := filepath.Join(path, "sdkmanrc")
			Expect(os.WriteFile(sdkmanrcFile, []byte(`java=17.0.2-tem # comment`), 0600)).To(Succeed())

			res, err := jvmvendors.ReadSDKMANRC(sdkmanrcFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal([]jvmvendors.SDKInfo{
				{Type: "java", Version: "17.0.2", Vendor: "tem"},
			}))
		})

		it("ignores empty lines", func() {
			sdkmanrcFile := filepath.Join(path, "sdkmanrc")
			Expect(os.WriteFile(sdkmanrcFile, []byte(`
# Enable auto-env through the sdkman_auto_env config
              
java=17.0.2-tem

`), 0600)).To(Succeed())

			res, err := jvmvendors.ReadSDKMANRC(sdkmanrcFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal([]jvmvendors.SDKInfo{
				{Type: "java", Version: "17.0.2", Vendor: "tem"},
			}))
		})
	})

	context("handles malformed key/values", func() {
		it("parses an empty value", func() {
			sdkmanrcFile := filepath.Join(path, "sdkmanrc")
			Expect(os.WriteFile(sdkmanrcFile, []byte(`java=`), 0600)).To(Succeed())

			res, err := jvmvendors.ReadSDKMANRC(sdkmanrcFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal([]jvmvendors.SDKInfo{
				{Type: "java", Version: "", Vendor: ""},
			}))
		})

		it("parses with an empty key", func() {
			sdkmanrcFile := filepath.Join(path, "sdkmanrc")
			Expect(os.WriteFile(sdkmanrcFile, []byte(`=foo-vend`), 0600)).To(Succeed())

			res, err := jvmvendors.ReadSDKMANRC(sdkmanrcFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal([]jvmvendors.SDKInfo{
				{Type: "", Version: "foo", Vendor: "vend"},
			}))
		})

		it("parses with an empty vendor", func() {
			sdkmanrcFile := filepath.Join(path, "sdkmanrc")
			Expect(os.WriteFile(sdkmanrcFile, []byte(`foo=bar`), 0600)).To(Succeed())

			res, err := jvmvendors.ReadSDKMANRC(sdkmanrcFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal([]jvmvendors.SDKInfo{
				{Type: "foo", Version: "bar", Vendor: ""},
			}))
		})
	})
}
