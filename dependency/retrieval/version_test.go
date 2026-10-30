// Copyright 2018-2026 the original author or authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"testing"

	"github.com/Masterminds/semver/v3"
)

func TestUnitVendorVersionToSemver(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{name: "plain version", in: "21.0.13", want: "21.0.13"},
		{name: "four components", in: "21.0.13.0.11", want: "21.0.13+0.11"},
		{name: "graalvm version", in: "25.4.4.1.1", want: "25.4.4+1.1"},
		{name: "existing build metadata", in: "17.0.21+9", want: "17.0.21+9"},
		{name: "adoptium lts metadata", in: "21.0.13+9.0.LTS", want: "21.0.13+9.0.LTS"},
		{name: "ga release", in: "28+21", want: "28+21"},
		{name: "sapmachine version", in: "21.0.13.0.3", want: "21.0.13+0.3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := vendorVersionToSemver(tc.in); got != tc.want {
				t.Errorf("vendorVersionToSemver(%q) = %q, want %q", tc.in, got, tc.want)
			}
			assertValidSemver(t, tc.want)
		})
	}
}

func TestUnitCorrettoVersionToSemver(t *testing.T) {
	for _, tc := range []struct {
		name  string
		in    string
		major int
		want  string
	}{
		{name: "java 8", in: "8.512.09.1", major: 8, want: "8.0.512+9.1"},
		{name: "java 11", in: "11.0.33.0.10", major: 11, want: "11.0.33+0.10"},
		{name: "java 21", in: "21.0.13.0.11", major: 21, want: "21.0.13+0.11"},
		{name: "java 27", in: "27.0.1.0.11", major: 27, want: "27.0.1+0.11"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := correttoVersionToSemver(tc.in, tc.major)
			if got != tc.want {
				t.Errorf("correttoVersionToSemver(%q, %d) = %q, want %q", tc.in, tc.major, got, tc.want)
			}
			assertValidSemver(t, tc.want)
		})
	}
}

func TestUnitNormalizeLibericaVersion(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{name: "java 8", in: "8u513+9", want: "8.0.513+9"},
		{name: "java 11", in: "11.0.33.0.3+7", want: "11.0.33+0.3.7"},
		{name: "java 21", in: "21.0.13.0.3+5", want: "21.0.13+0.3.5"},
		{name: "nik component", in: "21.0.12.1+2", want: "21.0.12+1.2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeLibericaVersion(tc.in)
			if got != tc.want {
				t.Errorf("normalizeLibericaVersion(%q) = %q, want %q", tc.in, got, tc.want)
			}
			assertValidSemver(t, tc.want)
		})
	}
}

func TestUnitNormalizeSemeruVersion(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{name: "java 8 new style", in: "8.0.504.5", want: "8.0.504+5"},
		{name: "java 8 legacy style", in: "8u472b08", want: "8.0.472+8"},
		{name: "java 21", in: "21.0.12.15", want: "21.0.12+15"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeSemeruVersion(tc.in)
			if got != tc.want {
				t.Errorf("normalizeSemeruVersion(%q) = %q, want %q", tc.in, got, tc.want)
			}
			assertValidSemver(t, tc.want)
		})
	}
}

func TestUnitAdoptiumVersionToSemver(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{name: "java 8", in: "8.0.504+1", want: "8.0.504+1"},
		{name: "java 8 legacy", in: "8u432-b06", want: "8.0.432"},
		{name: "java 17", in: "17.0.21+9", want: "17.0.21+9"},
		{name: "java 21", in: "21.0.13+9.0.LTS", want: "21.0.13+9.0.LTS"},
		{name: "dashed build", in: "27.0.1-8", want: "27.0.1+8"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := adoptiumVersionToSemver(tc.in)
			if got != tc.want {
				t.Errorf("adoptiumVersionToSemver(%q) = %q, want %q", tc.in, got, tc.want)
			}
			assertValidSemver(t, tc.want)
		})
	}
}

func TestUnitDragonwellVersion(t *testing.T) {
	for _, tc := range []struct {
		name  string
		tag   string
		major int
		java  string
		want  string
	}{
		{
			name:  "java 8",
			tag:   "dragonwell-standard-8.30.29_jdk8u502-ga",
			major: 8,
			java:  "8.0.502",
			want:  "8.0.502+30.29",
		},
		{
			name:  "java 21",
			tag:   "dragonwell-standard-21.0.12.0.12+8_jdk-21.0.12-ga",
			major: 21,
			java:  "21.0.12",
			want:  "21.0.12+0.12.8",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := dragonwellVersion(tc.tag, tc.major, tc.java)
			if got != tc.want {
				t.Errorf("dragonwellVersion(%q, %d, %q) = %q, want %q", tc.tag, tc.major, tc.java, got, tc.want)
			}
			assertValidSemver(t, tc.want)
		})
	}
}

func TestUnitParseTarget(t *testing.T) {
	for _, tc := range []struct {
		target   string
		wantOS   string
		wantArch string
		wantErr  bool
	}{
		{target: "linux-amd64", wantOS: "linux", wantArch: "amd64"},
		{target: "linux-arm64", wantOS: "linux", wantArch: "arm64"},
		{target: "linux-ppc64le", wantOS: "linux", wantArch: "ppc64le"},
		{target: "linux-ppc", wantErr: true},
		{target: "linux-riscv", wantErr: true},
		{target: "", wantErr: true},
	} {
		t.Run(tc.target, func(t *testing.T) {
			os, arch, err := parseTarget(tc.target)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseTarget(%q) expected an error, got %q/%q", tc.target, os, arch)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseTarget(%q) unexpected error: %v", tc.target, err)
			}
			if os != tc.wantOS || arch != tc.wantArch {
				t.Errorf("parseTarget(%q) = %q/%q, want %q/%q", tc.target, os, arch, tc.wantOS, tc.wantArch)
			}
		})
	}
}

func TestUnitFindAdoptiumBinary(t *testing.T) {
	binaries := []AdoptiumBinary{
		{ImageType: "jre", OS: "linux", Architecture: "x64"},
		{ImageType: "staticlibs", OS: "linux", Architecture: "x64"},
		{ImageType: "jdk", OS: "linux", Architecture: "x64"},
		{ImageType: "jdk", OS: "mac", Architecture: "x64"},
	}

	binary, ok := findAdoptiumBinary(binaries, "jdk", "linux", "x64")
	if !ok {
		t.Fatal("expected to find a jdk/linux/x64 binary")
	}
	if binary.ImageType != "jdk" || binary.OS != "linux" {
		t.Errorf("findAdoptiumBinary returned the wrong binary: %+v", binary)
	}

	if _, ok := findAdoptiumBinary(binaries, "jdk", "linux", "aarch64"); ok {
		t.Error("expected no match for jdk/linux/aarch64")
	}
}

func assertValidSemver(t *testing.T, version string) {
	t.Helper()

	v, err := semver.NewVersion(version)
	if err != nil {
		t.Fatalf("%q is not valid semver: %v", version, err)
	}

	constraint, err := semver.NewConstraint(fmt.Sprintf("^%d", v.Major()))
	if err != nil {
		t.Fatalf("unable to build constraint for %q: %v", version, err)
	}
	if !constraint.Check(v) {
		t.Errorf("%q does not satisfy its major constraint ^%d", version, v.Major())
	}
}
