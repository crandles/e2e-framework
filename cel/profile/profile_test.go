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

package profile

import (
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/e2e-framework/cel"
)

func deployment(replicas, ready int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: ready},
	}
}

func namespace(name string, labels map[string]string) *corev1.Namespace {
	return &corev1.Namespace{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
	}
}

func TestProfile_allPass(t *testing.T) {
	ev := newEv(t)
	p := Profile{
		Name: "Deployment/baseline",
		Features: []Feature{
			{
				Name:   "replicas-fully-ready",
				Target: deployment(3, 3),
				Assertions: []string{
					"object.status.readyReplicas == object.spec.replicas",
					"object.spec.replicas >= 1",
				},
			},
			{
				Name:   "has-namespace",
				Target: deployment(1, 1),
				Assertions: []string{
					`object.metadata.namespace != ""`,
				},
			},
		},
	}
	rs := p.Run(ev)
	if !rs.AllPassed() {
		t.Fatalf("expected pass, got %v", rs.Err())
	}
	if rs.Err() != nil {
		t.Fatalf("passed result should have nil Err")
	}
}

func TestProfile_reportsFailure(t *testing.T) {
	ev := newEv(t)
	p := Profile{
		Name: "broken",
		Features: []Feature{
			{
				Name:   "readyReplicas-matches",
				Target: deployment(3, 1),
				Assertions: []string{
					"object.status.readyReplicas == object.spec.replicas",
				},
			},
		},
	}
	rs := p.Run(ev)
	if rs.AllPassed() {
		t.Fatal("expected failure")
	}
	if rs.Err() == nil {
		t.Fatal("expected joined error")
	}
	if !strings.Contains(rs.Report(), "FAIL") {
		t.Fatalf("report should mention FAIL: %q", rs.Report())
	}
}

func TestProfile_featureBindingsMerged(t *testing.T) {
	ev := newEv(t)
	p := Profile{
		Features: []Feature{
			{
				Name:     "with-request",
				Target:   deployment(1, 1),
				Bindings: cel.RequestBinding(&cel.AdmissionRequest{Operation: "CREATE"}),
				Assertions: []string{
					`request.operation == "CREATE" && object.spec.replicas == 1`,
				},
			},
		},
	}
	if !p.Run(ev).AllPassed() {
		t.Fatal("feature bindings should merge into evaluator bindings")
	}
}

func TestReport_includesCounts(t *testing.T) {
	ev := newEv(t)
	p := Profile{
		Features: []Feature{
			{Name: "a", Target: deployment(1, 1), Assertions: []string{"true"}},
			{Name: "b", Target: deployment(1, 1), Assertions: []string{"false"}},
		},
	}
	got := p.Run(ev).Report()
	if !strings.Contains(got, "1/2") {
		t.Fatalf("report should include pass count, got %q", got)
	}
}

func TestLoadProfile_yaml(t *testing.T) {
	yaml := `
name: Deployment/baseline
features:
  - name: replicas-fully-ready
    target:
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: demo
        namespace: default
      spec:
        replicas: 3
      status:
        readyReplicas: 3
    assertions:
      - "object.status.readyReplicas == object.spec.replicas"
`
	p, err := LoadProfile(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("LoadProfile: %v", err)
	}
	if p.Name != "Deployment/baseline" {
		t.Errorf("Name = %q", p.Name)
	}
	if len(p.Features) != 1 {
		t.Fatalf("want 1 feature, got %d", len(p.Features))
	}

	ev := newEv(t)
	if !p.Run(ev).AllPassed() {
		t.Fatalf("loaded profile should pass against its embedded target")
	}
}

func TestLoadProfile_usesUnstructuredTarget(t *testing.T) {
	// A Namespace target via YAML exercises the unstructured path: the
	// loader does not know about corev1.Namespace.
	yaml := `
name: ns-check
features:
  - name: has-label
    target:
      apiVersion: v1
      kind: Namespace
      metadata:
        name: demo
        labels:
          pod-security.kubernetes.io/enforce: restricted
    assertions:
      - 'has(object.metadata.labels) && "pod-security.kubernetes.io/enforce" in object.metadata.labels'
`
	p, err := LoadProfile(strings.NewReader(yaml))
	if err != nil {
		t.Fatal(err)
	}
	ev := newEv(t)
	if !p.Run(ev).AllPassed() {
		t.Fatalf("ns profile should pass: %v", p.Run(ev).Err())
	}

	// Also verify in-memory construction with a typed namespace works.
	p2 := Profile{
		Name: "ns-check",
		Features: []Feature{
			{
				Name:   "has-label",
				Target: namespace("demo", map[string]string{"pod-security.kubernetes.io/enforce": "restricted"}),
				Assertions: []string{
					`has(object.metadata.labels) && "pod-security.kubernetes.io/enforce" in object.metadata.labels`,
				},
			},
		},
	}
	if !p2.Run(ev).AllPassed() {
		t.Fatalf("typed ns profile should pass: %v", p2.Run(ev).Err())
	}
}

func newEv(t *testing.T) *cel.Evaluator {
	t.Helper()
	ev, err := cel.NewEvaluator()
	if err != nil {
		t.Fatalf("NewEvaluator: %v", err)
	}
	return ev
}
