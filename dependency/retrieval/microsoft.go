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

	"github.com/paketo-buildpacks/packit/v2/cargo"
)

func generateMicrosoft(id string, constraint cargo.ConfigMetadataDependencyConstraint, existing []cargo.ConfigMetadataDependency) ([]Dependency, error) {
	majorVersion, err := extractVersionFromConstraint(constraint.Constraint)
	if err != nil {
		return nil, fmt.Errorf("skipping %s version %s: %w", id, constraint.Constraint, err)
	}

	if majorVersion == 8 {
		return generateAdoptium(id, constraint, existing)
	}

	return generateFoojay(id, constraint, existing)
}
