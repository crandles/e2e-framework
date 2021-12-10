/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package features

import (
	"fmt"

	"sigs.k8s.io/e2e-framework/pkg/internal/types"
)

// Table provides a structure for table-driven tests.
// Each entry in the table represents an executable assessment.
type Table []struct {
	Name       string
	Assessment Func
}

// Build converts the defined test steps in the table
// into a FeatureBuilder which can be used to add additional attributes
// to the feature before it's exercised. Build takes an optional feature name
// if omitted will be generated.
func (table Table) Build(featureName ...string) *FeatureBuilder {
	var name string
	if len(featureName) > 0 {
		name = featureName[0]
	}
	f := New(name)
	f.feat.steps = append(f.feat.steps, table.Steps()...)
	return f
}

func (table *Table) Name() string {
	return ""
}

func (table *Table) Labels() types.Labels {
	return types.Labels{}
}

func (table *Table) Steps() []types.Step {
	steps := []types.Step{}
	for i, test := range *table {
		if test.Name == "" {
			test.Name = fmt.Sprintf("Assessment-%d", i)
		}
		if test.Assessment != nil {
			steps = append(steps, newStep(test.Name, types.LevelAssess, test.Assessment))
		}
	}
	return steps
}
