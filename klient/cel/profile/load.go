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
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"sigs.k8s.io/e2e-framework/klient/cel"
)

// profileDoc is the on-disk YAML/JSON shape of a Profile. The Target of
// each Feature is represented as an arbitrary object map so profiles do not
// have to be tied to a compile-time Go type.
type profileDoc struct {
	Name     string       `json:"name"`
	Features []featureDoc `json:"features"`
}

type featureDoc struct {
	Name       string                 `json:"name"`
	Target     map[string]interface{} `json:"target"`
	Assertions []string               `json:"assertions"`
	Bindings   cel.Bindings           `json:"bindings,omitempty"`
}

// LoadProfile reads a Profile from a YAML or JSON stream.
//
// Example YAML:
//
//	name: Deployment/baseline
//	features:
//	  - name: replicas-fully-ready
//	    target:
//	      apiVersion: apps/v1
//	      kind: Deployment
//	      metadata: {name: demo, namespace: default}
//	      spec: {replicas: 3}
//	      status: {readyReplicas: 3}
//	    assertions:
//	      - "object.status.readyReplicas == object.spec.replicas"
func LoadProfile(r io.Reader) (Profile, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Profile{}, fmt.Errorf("profile: read: %w", err)
	}
	var doc profileDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Profile{}, fmt.Errorf("profile: parse: %w", err)
	}
	return fromDoc(doc), nil
}

func fromDoc(d profileDoc) Profile {
	p := Profile{Name: d.Name, Features: make([]Feature, 0, len(d.Features))}
	for _, fd := range d.Features {
		p.Features = append(p.Features, Feature{
			Name:       fd.Name,
			Target:     &unstructured.Unstructured{Object: fd.Target},
			Assertions: fd.Assertions,
			Bindings:   fd.Bindings,
		})
	}
	return p
}
