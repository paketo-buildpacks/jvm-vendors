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

package jvmvendors

import (
	"fmt"
	"os"
	"slices"

	"github.com/buildpacks/libcnb/v2"
	"github.com/paketo-buildpacks/libpak/v2"
	"github.com/paketo-buildpacks/libpak/v2/log"

	"github.com/paketo-buildpacks/jvm-vendors/internal"
)

const (
	PlanEntryNativeImageBuilder = "native-image-builder"
	PlanEntryJRE                = "jre"
	PlanEntryJDK                = "jdk"
)

func Detect(context libcnb.DetectContext) (libcnb.DetectResult, error) {
	logger := log.NewPaketoLogger(os.Stdout)

	md, err := libpak.NewBuildModuleMetadata(context.Buildpack.Metadata)
	if err != nil {
		return libcnb.DetectResult{}, fmt.Errorf("unable to create build module metadata\n%w", err)
	}

	cr, err := libpak.NewConfigurationResolver(md)
	if err != nil {
		return libcnb.DetectResult{}, fmt.Errorf("unable to create configuration resolver\n%w", err)
	}

	jvmVendors, err := internal.LoadJvmVendors(&cr)
	if err != nil {
		return libcnb.DetectResult{}, fmt.Errorf("unable to load JVM vendors\n%w", err)
	}

	jvmVendor, _ := cr.Resolve("BP_JVM_VENDOR")
	if jvmVendor != "" && !slices.Contains(jvmVendors, jvmVendor) {
		logger.Bodyf("SKIPPED: buildpack does not match requested JVM vendor of [%s], buildpack supports %q", jvmVendor, jvmVendors)
		return libcnb.DetectResult{Pass: false}, nil
	}

	return libcnb.DetectResult{
		Pass: true,
		Plans: []libcnb.BuildPlan{
			{
				Provides: []libcnb.BuildPlanProvide{
					{Name: PlanEntryJDK},
					{Name: PlanEntryNativeImageBuilder},
					{Name: PlanEntryJRE},
				},
				Requires: []libcnb.BuildPlanRequire{
					{Name: PlanEntryJDK},
				},
			},
			{
				Provides: []libcnb.BuildPlanProvide{
					{Name: PlanEntryJDK},
					{Name: PlanEntryNativeImageBuilder},
				},
				Requires: []libcnb.BuildPlanRequire{
					{Name: PlanEntryJDK},
				},
			},
			{
				Provides: []libcnb.BuildPlanProvide{
					{Name: PlanEntryJDK},
					{Name: PlanEntryJRE},
				},
			},
			{
				Provides: []libcnb.BuildPlanProvide{
					{Name: PlanEntryJDK},
				},
			},
			{
				Provides: []libcnb.BuildPlanProvide{
					{Name: PlanEntryJRE},
				},
			},
		},
	}, nil
}
