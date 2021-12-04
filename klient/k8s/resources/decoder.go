package resources

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/e2e-framework/klient/k8s"
)

func isDirectory(dir string) bool {
	d, err := os.Stat(dir)
	if err != nil {
		return false
	}
	return d.IsDir()
}

func listResourceFiles(directory string) ([]string, error) {
	// determine if directory is a path to a directory, or if it has a filename Glob included
	var base string
	dir, err := os.Stat(directory)
	if err != nil { // if this is not a resolvable path, assume the "base" of the path is a filename-prefix
		base = filepath.Base(directory)
		directory = filepath.Dir(directory)
		if !isDirectory(directory) {
			return nil, fmt.Errorf("files with prefix %q in directory %q could not be found", base, directory)
		}
	} else if err == nil && dir.IsDir() {
		// it is a directory, continue
	} else if !dir.IsDir() { // this is not a directory, but an (unexpected) file
		return nil, fmt.Errorf("%q is not a directory. not supported", dir.Name())
	}
	yamls, err := filepath.Glob(filepath.Join(directory, fmt.Sprintf("%s*.yaml", base)))
	if err != nil {
		return nil, err
	}
	ymls, err := filepath.Glob(filepath.Join(directory, fmt.Sprintf("%s*.yml", base)))
	if err != nil {
		return nil, err
	}
	jsons, err := filepath.Glob(filepath.Join(directory, fmt.Sprintf("%s*.json", base)))
	if err != nil {
		return nil, err
	}
	filenames := append(yamls, append(ymls, jsons...)...)
	sort.Strings(filenames) // sort before returning files for a consisent order
	return filenames, nil
}

// MutateFunc is a function executed after an object is decoded to alter its state in a pre-defined way, and can be used to apply defaults.
type MutateFunc func(k8s.Object) error

// DecodeDirectory loads YAML or JSON files from the given directory into a copy of the provided object.
// Patches are applied to each object after they have been decoded.
//
// Files with the extension `.yaml`, `.yml`, and `.json` are considered.
//
// Specify an optional filename prefix in the directory string to filter directory files further, example:
// - "testdata" -- matches appropriate files in the `testdata` directory
// - "testdata/example-sa" -- matches files in the `testdata` directory that start with `example-sa` (if directory string does not resolve to a directory)
func DecodeDirectory(directory string, obj k8s.Object, patches ...MutateFunc) ([]k8s.Object, error) {
	files, err := listResourceFiles(directory)
	if err != nil {
		return nil, err
	}
	objects := []k8s.Object{}
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		o, err := DecodeObjects(f, obj, patches...)
		if err != nil {
			return nil, err
		}
		objects = append(objects, o...)
	}
	return objects, nil
}

// DecodeObjects decodes a multi-document YAML or JSON file into a copy of the provided k8s.Object. Patches are applied
// to each object after they have been decoded.
func DecodeObjects(manifest io.Reader, obj k8s.Object, patches ...MutateFunc) ([]k8s.Object, error) {
	// copy base object to preserve for each decoding iteration
	base := obj.DeepCopyObject()
	objects := []k8s.Object{}
	// start decoding documents
	decoder := yaml.NewYAMLOrJSONDecoder(manifest, 1024)
	for {
		obj := base.DeepCopyObject().(k8s.Object) // copy the base object to decode new object to
		if err := decoder.Decode(obj); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, err
		}
		for _, patch := range patches {
			patch(obj)
		}
		objects = append(objects, obj)
	}
	return objects, nil
}

type object struct {
	metav1.ObjectMeta
	runtime.Object
}

// // DecodeObjectsByGVK decodes a multi-document YAML or JSON file into a copy of the provided k8s.Object. Patches are applied
// // to each object after they have been decoded.
// func DecodeObjectsByGVK(manifest io.Reader, kind schema.GroupVersionKind, patches ...MutateFunc) ([]k8s.Object, error) {
// 	objects := []k8s.Object{}
// 	// start decoding documents
// 	decoder := yaml.NewYAMLOrJSONDecoder(manifest, 1024)
// 	for {
// 		o, err := scheme.Scheme.New(kind)
// 		if err != nil {
// 			return objects, err
// 		}
// 		obj := k8s.Object(&object{metav1.ObjectMeta{}, o})
// 		if err := decoder.Decode(obj); errors.Is(err, io.EOF) {
// 			break
// 		} else if err != nil {
// 			return nil, err
// 		}
// 		for _, patch := range patches {
// 			patch(obj)
// 		}
// 		objects = append(objects, obj)
// 	}
// 	return objects, nil
// }

// DecodeListItems decodes a multi-document YAML or JSON file into items on the provided k8s.ObjectList. Patches are applied
// to each object after they have been decoded.
func DecodeListItems(manifest io.Reader, obj k8s.ObjectList, patches ...MutateFunc) error {
	// get a pointer to the list's Items slice
	itemsPtr, err := meta.GetItemsPtr(obj)
	if err != nil {
		return err
	}
	// convert to pointer so we can append items
	items, err := conversion.EnforcePtr(itemsPtr)
	if err != nil {
		return err
	}
	// determine the type of the slice Items and create a new reference object as a base to decode into for each document
	base := reflect.New(items.Type().Elem()).Interface().(k8s.Object)
	// start decoding documents
	decoder := yaml.NewYAMLOrJSONDecoder(manifest, 1024)
	for {
		obj := base.DeepCopyObject().(k8s.Object) // copy the base object to decode new object to
		if err := decoder.Decode(obj); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return err
		}
		for _, patch := range patches {
			patch(obj)
		}
		items.Set(reflect.Append(items, reflect.ValueOf(obj).Elem()))
	}
	return nil
}

// DecodeDocuments decodes a multi-document YAML or JSON file into a copy of the provided k8s.Object and invokes
// fn on each decoded object, after patches have been applied.
func DecodeDocuments(manifest io.Reader, base k8s.Object, fn func(k8s.Object), patches ...MutateFunc) error {
	// start decoding documents
	decoder := yaml.NewYAMLOrJSONDecoder(manifest, 1024)
	for {
		obj := base.DeepCopyObject().(k8s.Object) // copy the base object to decode new object to
		if err := decoder.Decode(obj); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return err
		}
		for _, patch := range patches {
			patch(obj)
		}
		fn(obj)
	}
	return nil
}

// Decoder decodes a multi-document YAML or JSON file into Go objects for any types that have been registered with scheme.Scheme.
// If the GroupVersionKind defaults is provided, it is used when determining the object Kind.
func Decoder(manifest io.Reader, defaults *schema.GroupVersionKind, fn func(k8s.Object), patches ...MutateFunc) error {
	k8sDecoder := serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer().Decode
	decoder := yaml.NewYAMLOrJSONDecoder(manifest, 1024)
	for {
		// using decoder to split documents, incurring second decode
		var raw runtime.RawExtension
		if err := decoder.Decode(&raw); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return err
		}
		var obj k8s.Object
		runtimeObj, _, err := k8sDecoder(raw.Raw, defaults, nil)
		if err != nil {
			return err
		}
		obj = runtimeObj.(k8s.Object)
		for _, patch := range patches {
			patch(obj)
		}
		fn(obj)
	}
	return nil
}

// Decode a single-document YAML or JSON file into the provided object. Patches are applied
// after decoding to the object to update the loaded resource.
func Decode(manifest io.Reader, obj k8s.Object, patches ...MutateFunc) error {
	if err := yaml.NewYAMLOrJSONDecoder(manifest, 1024).Decode(obj); err != nil {
		return err
	}
	for _, patch := range patches {
		patch(obj)
	}
	return nil
}

// DecodeFile decodes a single-document YAML or JSON file into the provided object. Patches are applied
// after decoding to the object to update the loaded resource.
func DecodeFile(manifestPath string, obj k8s.Object, patches ...MutateFunc) error {
	f, err := os.Open(manifestPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return Decode(f, obj, patches...)
}

// DecodeString decodes a single-document YAML or JSON string into the provided object. Patches are applied
// after decoding to the object to update the loaded resource.
func DecodeString(rawManifest string, obj k8s.Object, patches ...MutateFunc) error {
	return Decode(strings.NewReader(rawManifest), obj, patches...)
}

// MutateLabels can be used to patch an objects metadata.labels after being decoded
func MutateLabels(overrides map[string]string) MutateFunc {
	return func(obj k8s.Object) error {
		labels := obj.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
			obj.SetLabels(labels)
		}
		for key, value := range overrides {
			labels[key] = value
		}
		return nil
	}
}

// MutateAnnotations can be used to patch an objects metadata.annotations after being decoded
func MutateAnnotations(overrides map[string]string) MutateFunc {
	return func(obj k8s.Object) error {
		annotations := obj.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
			obj.SetLabels(annotations)
		}
		for key, value := range overrides {
			annotations[key] = value
		}
		return nil
	}
}

// MutateOwnerAnnotations can be used to patch objects using the given owner object after being decoded
func MutateOwnerAnnotations(owner k8s.Object) MutateFunc {
	return func(obj k8s.Object) error {
		return controllerutil.SetOwnerReference(owner, obj, scheme.Scheme)
	}
}
