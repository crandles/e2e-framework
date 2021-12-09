package resources

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/e2e-framework/klient/k8s"
)

func TestDecode(t *testing.T) {
	var testYAML string = filepath.Join("testdata", "example-configmap-1.yaml")
	f, err := os.Open(testYAML)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg := v1.ConfigMap{}
	if err := Decode(f, &cfg); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Data["foo.cfg"]; !ok {
		t.Fatal("key foo.cfg not found in decoded ConfigMap")
	}
}

func TestDecodeUnstructuredCRD(t *testing.T) {
	var testYAML string = filepath.Join("testdata", "fake-crd.yaml")
	f, err := os.Open(testYAML)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	obj, err := DecodeAny(f, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		t.Fatalf("expected unstructured.Unstructured, got %T", u)
	}

	if _, ok := u.Object["spec"]; !ok {
		t.Fatalf("spec field of CRD not found")
	}

	spec, ok := u.Object["spec"].(map[string]interface{})
	if !ok {
		t.Fatalf("spec not expected map[string]interface{}, got: %T", u.Object["spec"])
	}

	example, ok := spec["example"].(string)
	if !ok {
		t.Fatalf("spec.example not expectedstring, got: %T", spec["example"])
	}
	if example != "value" {
		t.Fatalf("spec.example not expected 'value', got %q", spec["example"])
	}
}

func TestDecodeAny(t *testing.T) {
	var testYAML string = filepath.Join("testdata", "example-configmap-3.json")
	f, err := os.Open(testYAML)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if obj, err := DecodeAny(f, nil); err != nil {
		t.Fatal(err)
	} else if cfg, ok := obj.(*v1.ConfigMap); !ok && cfg.Data["foo.cfg"] != "" {
		t.Fatal("key foo.cfg not found in decoded ConfigMap")
	} else if _, ok := cfg.Data["foo.cfg"]; !ok {
		t.Fatal("key foo.cfg not found in decoded ConfigMap")
	}
}

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

func TestDecodeEachFile(t *testing.T) {
	// load `testdata/examples/example-sa*.(yaml|yml|json)`
	dir := filepath.Join("testdata", "examples", "example-sa")
	count := 0
	if err := DecodeEachFile(context.TODO(), dir, nil, func(ctx context.Context, obj k8s.Object) error {
		count++
		return nil
	}); err != nil {
		t.Fatal(err)
	} else if expected := 3; count != expected {
		t.Fatalf("expected %d objects, got: %d", expected, count)
	}
	// load `testdata/examples/*.(yaml|yml|json)`
	dir = filepath.Join("testdata", "examples")
	count = 0
	serviceAccounts := 0
	configs := 0
	if err := DecodeEachFile(context.TODO(), dir, nil, func(ctx context.Context, obj k8s.Object) error {
		count++
		if _, ok := obj.(*v1.ConfigMap); ok {
			configs++
		} else if _, ok := obj.(*v1.ServiceAccount); ok {
			serviceAccounts++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	} else if expected := 4; count != expected {
		t.Fatalf("expected %d objects, got: %d", expected, count)
	} else if expected := 3; expected != serviceAccounts {
		t.Fatalf("expected %d serviceAccounts got %d", expected, serviceAccounts)
	} else if expected := 1; expected != configs {
		t.Fatalf("expected %d configs got %d", expected, configs)
	}
}

func TestDecodeAllFiles(t *testing.T) {
	// load `testdata/examples/example-sa*.(yaml|yml|json)`
	dir := filepath.Join("testdata", "examples", "example-sa")
	if objects, err := DecodeAllFiles(dir, nil); err != nil {
		t.Fatal(err)
	} else if expected, got := 3, len(objects); got != expected {
		t.Fatalf("expected %d objects, got: %d", expected, got)
	}
	// load `testdata/examples/*.(yaml|yml|json)`
	dir = filepath.Join("testdata", "examples")
	if objects, err := DecodeAllFiles(dir, nil); err != nil {
		t.Fatal(err)
	} else if expected, got := 4, len(objects); got != expected {
		t.Fatalf("expected %d objects, got: %d", expected, got)
	}
}

func TestDecodeEach(t *testing.T) {
	var testYAML string = filepath.Join("testdata", "example-multidoc-1.yaml")
	f, err := os.Open(testYAML)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	count := 0
	err = DecodeEach(context.TODO(), f, nil, func(ctx context.Context, obj k8s.Object) error {
		count++
		switch cfg := obj.(type) {
		case *v1.ConfigMap:
			if _, ok := cfg.Data["foo"]; !ok {
				t.Fatalf("expected key 'foo' in ConfigMap.Data, got: %v", cfg.Data)
			}
		default:
			t.Fatalf("unexpected type returned not ConfigMap: %T", cfg)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	} else if count != 2 {
		t.Fatalf("expected 2 documents, got: %d", count)
	}
}

func TestDecodeAll(t *testing.T) {
	var testYAML string = filepath.Join("testdata", "example-multidoc-1.yaml")
	f, err := os.Open(testYAML)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if objects, err := DecodeAll(f, nil); err != nil {
		t.Fatal(err)
	} else if expected, got := 2, len(objects); got != expected {
		t.Fatalf("expected 2 documents, got: %d", got)
	}
}

func TestDecodersWithMutateFunc(t *testing.T) {
	t.Run("DecodeAny", func(t *testing.T) {
		var testYAML string = filepath.Join("testdata", "example-configmap-3.json")
		f, err := os.Open(testYAML)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if obj, err := DecodeAny(f, nil, MutateLabels(map[string]string{"injected": "labelvalue"})); err != nil {
			t.Fatal(err)
		} else if cfg, ok := obj.(*v1.ConfigMap); !ok && cfg.Data["foo.cfg"] != "" {
			t.Fatal("key foo.cfg not found in decoded ConfigMap")
		} else if cfg.ObjectMeta.Labels["injected"] != "labelvalue" {
			t.Fatal("injected label value not found", cfg.ObjectMeta.Labels)
		}
	})
	t.Run("DecodeEach", func(t *testing.T) {
		dir := filepath.Join("testdata", "examples", "example-sa")
		if err := DecodeEachFile(context.TODO(), dir, nil, func(ctx context.Context, obj k8s.Object) error {
			if labels := obj.GetLabels(); labels["injected"] != "labelvalue" {
				t.Fatalf("unexpected value in labels: %q", labels["injected"])
			}
			return nil
		}, MutateLabels(map[string]string{"injected": "labelvalue"})); err != nil {
			t.Fatal(err)
		}
	})
}

func TestHandlerFuncs(t *testing.T) {
	handlerNS := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "handler-test"}}
	res, err := New(cfg)
	if err != nil {
		t.Fatalf("Error creating new resources object: %v", err)
	}
	err = res.Create(context.TODO(), handlerNS)
	if err != nil {
		t.Fatalf("error while creating namespace %q: %s", handlerNS.Name, err)
	}
	dir := filepath.Join("testdata", "examples")
	patches := []MutateFunc{MutateNamespace(handlerNS.Name), MutateLabels(map[string]string{"injected": "labelvalue"})}

	t.Run("DecodeEach_Create", func(t *testing.T) {
		if err := DecodeEachFile(context.TODO(), dir, nil, CreateHandler(res), patches...); err != nil {
			t.Fatal(err)
		}
		t.Run("GetHandler", func(t *testing.T) {
			count := 0
			serviceAccounts := 0
			configs := 0
			objects, err := DecodeAllFiles(dir, nil, patches...)
			if err != nil {
				t.Fatal(err)
			}
			for i := range objects {
				if err := GetHandler(res, func(ctx context.Context, obj k8s.Object) error {
					if labels := objects[i].GetLabels(); labels["injected"] != "labelvalue" {
						t.Fatalf("unexpected value in labels: %q", labels["injected"])
					} else {
						count++
						switch cfg := obj.(type) {
						case *v1.ConfigMap:
							if _, ok := cfg.Data["foo.cfg"]; !ok {
								t.Fatalf("expected key 'foo.cfg' in ConfigMap.Data, got: %v", cfg.Data)
							}
							configs++
						case *v1.ServiceAccount:
							serviceAccounts++
						default:
							t.Fatalf("unexpected type returned not ConfigMap: %T", cfg)
						}
					}
					return nil
				})(ctx, objects[i]); err != nil {
					t.Fatal(err)
				}
			}
		})
	})
}
