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
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

var (
	BuildContribution  = map[string]any{"build": true}
	LaunchContribution = map[string]any{"launch": true}
	NoContribution     = map[string]any{}
)

func TestUnit(t *testing.T) {
	suite := spec.New("Java Vendors", spec.Report(report.Terminal{}))
	suite("Build", testBuild)
	suite("CertificateLoader", testCertificateLoader)
	suite("Contributions", testContributions)
	suite("Detect", testDetect)
	suite("Generate", testGenerate)
	suite("JavaSecurityProperties", testJavaSecurityProperties)
	suite("JDK", testJDK)
	suite("JRE", testJRE)
	suite("NIK", testNIK)
	suite("JLink", testJLink)
	suite("NewManifest", testNewManifest)
	suite("NewManifestFromJAR", testNewManifestFromJAR)
	suite("MavenJARListing", testMavenJARListing)
	suite("SDKMAN", testSDKMAN)
	suite("Versions", testVersions)
	suite("JVMVersions", testJVMVersion)
	suite("Keystore", testKeystore)
	suite.Run(t)
}
