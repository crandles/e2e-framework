package resources

import (
	"os"
	"path/filepath"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/e2e-framework/klient/k8s"
)

func TestDecodeFile(t *testing.T) {
	var testYAML string = filepath.Join("testdata", "example-configmap-1.yaml")
	cfg := v1.ConfigMap{}
	if err := DecodeFile(testYAML, &cfg, func(o k8s.Object) error {
		obj := o.(*v1.ConfigMap)
		if obj.ObjectMeta.Labels == nil {
			obj.Labels = make(map[string]string)
		}
		obj.ObjectMeta.Labels["inject-value"] = "test123"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if cfg.ObjectMeta.Labels["inject-value"] != "test123" {
		t.Fatal("injected label value not found", cfg.ObjectMeta.Labels)
	}
	cfg = v1.ConfigMap{}
	if err := DecodeFile(testYAML, &cfg, MutateLabels(map[string]string{"injected": "labelvalue"})); err != nil {
		t.Fatal(err)
	}
	if cfg.ObjectMeta.Labels["injected"] != "labelvalue" {
		t.Fatal("injected label value not found", cfg.ObjectMeta.Labels)
	}
}

func TestDecodeObjects(t *testing.T) {
	var testYAML string = filepath.Join("testdata", "example-multidoc-1.yaml")
	f, err := os.Open(testYAML)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg := v1.ConfigMap{}
	if objects, err := DecodeObjects(f, &cfg, MutateLabels(map[string]string{"injected": "labelvalue"})); err != nil {
		t.Fatal(err)
	} else if len(objects) != 2 {
		t.Fatalf("expected 2 documents, got: %d", len(objects))
	}
}

// func TestLDecodeObjectsByGVK(t *testing.T) {
// 	var testYAML string = filepath.Join("testdata", "example-multidoc-1.yaml")
// 	f, err := os.Open(testYAML)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer f.Close()
// 	if objects, err := DecodeObjectsByGVK(f, schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}, PatchLabels(map[string]string{"injected": "labelvalue"})); err != nil {
// 		t.Fatal(err)
// 	} else if len(objects) != 2 {
// 		t.Fatalf("expected 2 documents, got: %d", len(objects))
// 	}
// }

func TestDirectory(t *testing.T) {
	// load `testdata/examples/example-sa*.(yaml|yml|json)`
	dir := filepath.Join("testdata", "examples", "example-sa")
	if objects, err := DecodeDirectory(dir, &v1.ServiceAccount{}); err != nil {
		t.Fatal(err)
	} else if got := len(objects); got != 3 {
		t.Fatalf("expected 3 objects, got: %d", got)
	}
	// load `testdata/examples/*.(yaml|yml|json)`
	dir = filepath.Join("testdata", "examples")
	if objects, err := DecodeDirectory(dir, &unstructured.Unstructured{}); err != nil {
		t.Fatal(err)
	} else if got := len(objects); got != 4 {
		t.Fatalf("expected 4 objects, got: %d", got)
	}
}

func TestDecodeListItems(t *testing.T) {
	var testYAML string = filepath.Join("testdata", "example-multidoc-1.yaml")
	f, err := os.Open(testYAML)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg := v1.ConfigMapList{}
	if err := DecodeListItems(f, &cfg, MutateLabels(map[string]string{"injected": "labelvalue"})); err != nil {
		t.Fatal(err)
	} else if len(cfg.Items) != 2 {
		t.Fatalf("expected 2 documents, got: %d", len(cfg.Items))
	}
}

func TestDecodeDocuments(t *testing.T) {
	var testYAML string = filepath.Join("testdata", "example-multidoc-1.yaml")
	f, err := os.Open(testYAML)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	configs := []*v1.ConfigMap{}
	if err := DecodeDocuments(f, &v1.ConfigMap{}, func(obj k8s.Object) {
		configs = append(configs, obj.(*v1.ConfigMap))
	}, MutateLabels(map[string]string{"injected": "labelvalue"})); err != nil {
		t.Fatal(err)
	} else if len(configs) != 2 {
		t.Fatalf("expected 2 documents, got: %d", len(configs))
	}
}

func TestDecoder(t *testing.T) {
	var testYAML string = filepath.Join("testdata", "example-multidoc-1.yaml")
	f, err := os.Open(testYAML)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	configs := []*v1.ConfigMap{}
	if err := Decoder(f, nil, func(obj k8s.Object) {
		configs = append(configs, obj.(*v1.ConfigMap))
	}, MutateLabels(map[string]string{"injected": "labelvalue"})); err != nil {
		t.Fatal(err)
	} else if len(configs) != 2 {
		t.Fatalf("expected 2 documents, got: %d", len(configs))
	}
}
