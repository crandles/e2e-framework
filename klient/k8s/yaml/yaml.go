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

package yaml

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
)

// LoadUnstructuredYAML loads a single-document YAML or JSON file
func LoadUnstructuredYAML(filename string) (*unstructured.Unstructured, error) {
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading yaml file: %q", filename)
	}
	crd := unstructured.Unstructured{}
	if err := yaml.Unmarshal(file, &crd); err != nil {
		return nil, errors.Wrapf(err, "error decoding yaml file: %s", filename)
	}
	return &crd, nil
}

func listResourceFiles(directory string) ([]string, error) {
	yamls, err := filepath.Glob(filepath.Join(directory, "*.yaml"))
	if err != nil {
		return nil, err
	}
	ymls, err := filepath.Glob(filepath.Join(directory, "*.yml"))
	if err != nil {
		return nil, err
	}
	jsons, err := filepath.Glob(filepath.Join(directory, "*.json"))
	if err != nil {
		return nil, err
	}
	return append(yamls, append(ymls, jsons...)...), nil
}

// LoadUnstructuredYAMLDirectory loads the set of single-document YAML and JSON files (*.yaml and *.yml and *.json) at the given directory
func LoadUnstructuredYAMLDirectory(directory string) ([]*unstructured.Unstructured, error) {
	files, err := listResourceFiles(directory)
	if err != nil {
		return nil, errors.Wrapf(err, "could not list directory %q", directory)
	}

	objects := []*unstructured.Unstructured{}
	for _, file := range files {
		obj, err := LoadUnstructuredYAML(file)
		if err != nil {
			return objects, err
		}
		objects = append(objects, obj)
	}
	if len(objects) == 0 {
		return nil, fmt.Errorf("no files found in dir: %s", directory)
	}
	return objects, nil
}

// CreateResourceFromYAML loads a YAML or JSON file and creates it using the given klient.Client
func CreateResourceFromYAML(klient klient.Client, filename, namespace string, opts ...resources.CreateOption) error {
	obj, err := LoadUnstructuredYAML(filename)
	if err != nil {
		return err
	}
	obj.SetNamespace(namespace)
	err = klient.Resources(namespace).Create(context.TODO(), obj, opts...)
	return errors.Wrapf(err, "error creating unstructured object: %q; kind: %q; namespace: %q; name: %q", filename, obj.GetKind(), obj.GetNamespace(), obj.GetName())
}

// CreateResourcesFromYAMLDirectory loads YAML or JSON files from a directory and creates them using the given klient.Client
func CreateResourcesFromYAMLDirectory(klient klient.Client, directory, namespace string, opts ...resources.CreateOption) error {
	files, err := listResourceFiles(directory)
	if err != nil {
		return errors.Wrapf(err, "could not list directory %q", directory)
	}
	for _, f := range files {
		if err := CreateResourceFromYAML(klient, f, namespace, opts...); err != nil {
			return err
		}
	}
	return nil
}

// DeleteResourceFromYAML loads a YAML or JSON file and deletes it using the given klient.Client
func DeleteResourceFromYAML(klient klient.Client, filename, namespace string, opts ...resources.DeleteOption) error {
	obj, err := LoadUnstructuredYAML(filename)
	if err != nil {
		return err
	}
	obj.SetNamespace(namespace)
	err = klient.Resources(namespace).Delete(context.TODO(), obj, opts...)
	return errors.Wrapf(err, "error deleting unstructured object: %q; kind: %q; namespace: %q; name: %q", filename, obj.GetKind(), obj.GetNamespace(), obj.GetName())
}

// DeleteResourcesFromYAMLDirectory loads YAML or JSON files from a directory and deletes them using the given klient.Client
func DeleteResourcesFromYAMLDirectory(klient klient.Client, directory, namespace string, opts ...resources.DeleteOption) error {
	files, err := listResourceFiles(directory)
	if err != nil {
		return errors.Wrapf(err, "could not list directory %q", directory)
	}
	for _, f := range files {
		if err := DeleteResourceFromYAML(klient, f, namespace, opts...); err != nil {
			return err
		}
	}
	return nil
}
