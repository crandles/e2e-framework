/*
Copyright 2026 The Kubernetes Authors.

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

// Package profile provides named conformance profiles built from CEL
// invariants. A Profile is a set of Features, each pairing a target object
// with a list of CEL assertions that must all hold.
package profile

import (
	"errors"
	"fmt"
	"strings"

	"sigs.k8s.io/e2e-framework/klient/cel"
	"sigs.k8s.io/e2e-framework/klient/k8s"
)

// Profile is a named conformance profile.
type Profile struct {
	Name     string
	Features []Feature
}

// Feature is one named invariant within a Profile: a target object paired
// with CEL assertions that must all evaluate to true.
type Feature struct {
	Name       string
	Target     k8s.Object
	Assertions []string
	// Bindings are optional extra bindings merged in after the target is
	// bound to `object` (or `self`, if the Evaluator uses EnvCRD).
	Bindings cel.Bindings
}

// Result is the outcome of evaluating a single Feature.
type Result struct {
	Feature string
	Passed  bool
	Errors  []error
}

// Results is the full report for a Profile.Run.
type Results []Result

// AllPassed reports whether every feature passed.
func (rs Results) AllPassed() bool {
	for _, r := range rs {
		if !r.Passed {
			return false
		}
	}
	return true
}

// Err returns a single joined error covering every failing feature, or nil.
// Suitable for returning directly from a features.Func on failure.
func (rs Results) Err() error {
	var errs []error
	for _, r := range rs {
		errs = append(errs, r.Errors...)
	}
	return errors.Join(errs...)
}

// Report renders the results in a human-readable form, one line per
// feature, with a summary at the top.
func (rs Results) Report() string {
	var b strings.Builder
	total := len(rs)
	passed := 0
	for _, r := range rs {
		if r.Passed {
			passed++
		}
	}
	fmt.Fprintf(&b, "profile: %d/%d features passed\n", passed, total)
	for _, r := range rs {
		status := "PASS"
		if !r.Passed {
			status = "FAIL"
		}
		fmt.Fprintf(&b, "  %s  %s\n", status, r.Feature)
		for _, err := range r.Errors {
			fmt.Fprintf(&b, "        %v\n", err)
		}
	}
	return b.String()
}

// Run evaluates every Feature and returns a Results report.
func (p Profile) Run(ev *cel.Evaluator) Results {
	out := make(Results, 0, len(p.Features))
	for _, f := range p.Features {
		out = append(out, runFeature(ev, f))
	}
	return out
}

func runFeature(ev *cel.Evaluator, f Feature) Result {
	r := Result{Feature: f.Name, Passed: true}
	bindings := cel.Bind(cel.ObjectBinding(f.Target), f.Bindings)
	for _, expr := range f.Assertions {
		if err := ev.Assert(expr, bindings); err != nil {
			r.Passed = false
			r.Errors = append(r.Errors, fmt.Errorf("%s: %w", f.Name, err))
		}
	}
	return r
}
