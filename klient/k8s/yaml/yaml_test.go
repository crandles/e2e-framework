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
	"os"
	"path/filepath"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

var (
	testenv env.Environment
)

func TestCreateResourceFromYAML(t *testing.T) {
	var testYAML string = filepath.Join("testdata", "example-configmap-1.yaml")
	createYAML := features.New("unstructured/unstructured").WithLabel("env", "dev").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if err := CreateResourceFromYAML(cfg.Client(), testYAML, cfg.Namespace()); err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Assess("Object Creation", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			config := v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "example-1", Namespace: cfg.Namespace()}}
			if err := cfg.Client().Resources(cfg.Namespace()).Get(ctx, config.GetName(), config.GetNamespace(), &config); err != nil {
				t.Fatalf("could not retrieve created object: %s", err)
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if err := DeleteResourceFromYAML(cfg.Client(), testYAML, cfg.Namespace()); err != nil {
				t.Fatal(err)
			}
			return ctx
		}).Feature()
	testenv.Test(t, createYAML)
}

func TestCreateResourceFromJSON(t *testing.T) {
	var testYAML string = filepath.Join("testdata", "example-configmap-3.json")
	createYAML := features.New("unstructured/unstructured").WithLabel("env", "dev").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if err := CreateResourceFromYAML(cfg.Client(), testYAML, cfg.Namespace()); err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Assess("Object Creation", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			config := v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "example-3", Namespace: cfg.Namespace()}}
			if err := cfg.Client().Resources(cfg.Namespace()).Get(ctx, config.GetName(), config.GetNamespace(), &config); err != nil {
				t.Fatalf("could not retrieve created object: %s", err)
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if err := DeleteResourceFromYAML(cfg.Client(), testYAML, cfg.Namespace()); err != nil {
				t.Fatal(err)
			}
			return ctx
		}).Feature()
	testenv.Test(t, createYAML)
}

func TestCreateResourcesFromYAMLDirectory(t *testing.T) {
	var testYAMLDir string = filepath.Join("testdata", "examples")
	createYAMLDir := features.New("unstructured/unstructured").WithLabel("env", "dev").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if err := CreateResourcesFromYAMLDirectory(cfg.Client(), testYAMLDir, cfg.Namespace()); err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Assess("Object Creation", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			config := v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "example-2", Namespace: cfg.Namespace()}}
			if err := cfg.Client().Resources(cfg.Namespace()).Get(ctx, config.GetName(), config.GetNamespace(), &config); err != nil {
				t.Fatalf("could not retrieve created object: %s", err)
			}
			// service account from yaml file
			sa := v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "example-1", Namespace: cfg.Namespace()}}
			if err := cfg.Client().Resources(cfg.Namespace()).Get(ctx, sa.GetName(), sa.GetNamespace(), &sa); err != nil {
				t.Fatalf("could not retrieve created object: %s", err)
			}
			// service account from yml file
			sa2 := v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "example-2", Namespace: cfg.Namespace()}}
			if err := cfg.Client().Resources(cfg.Namespace()).Get(ctx, sa2.GetName(), sa2.GetNamespace(), &sa2); err != nil {
				t.Fatalf("could not retrieve created object: %s", err)
			}
			// service account from json file
			sa3 := v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "example-3", Namespace: cfg.Namespace()}}
			if err := cfg.Client().Resources(cfg.Namespace()).Get(ctx, sa3.GetName(), sa3.GetNamespace(), &sa3); err != nil {
				t.Fatalf("could not retrieve created object: %s", err)
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if err := DeleteResourcesFromYAMLDirectory(cfg.Client(), testYAMLDir, cfg.Namespace()); err != nil {
				t.Fatal(err)
			}
			return ctx
		}).Feature()
	testenv.Test(t, createYAMLDir)
}

func TestMain(m *testing.M) {
	testenv = env.New()
	kindClusterName := envconf.RandomName("yaml", 16)
	namespace := envconf.RandomName("kind-ns-yaml", 16)
	testenv.Setup(
		envfuncs.CreateKindCluster(kindClusterName),
		envfuncs.CreateNamespace(namespace),
	)
	testenv.Finish(
		envfuncs.DeleteNamespace(namespace),
		envfuncs.DestroyKindCluster(kindClusterName),
	)
	os.Exit(testenv.Run(m))
}
