package resources

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	sort.Strings(filenames) // sort before returning files for a consistent order
	return filenames, nil
}

// MutateFunc is a function executed after an object is decoded to alter its state in a pre-defined way, and can be used to apply defaults.
// Returning an error halts decoding of any further objects.
type MutateFunc func(obj k8s.Object) error

// HandlerFunc is a function executed after an object has been decoded and patched. If an error is returned, futher decoding is halted.
type HandlerFunc func(ctx context.Context, obj k8s.Object) error

// DecodeEachFile resolves files at the filepath (with Glob support), decoding files that match the common JSON or YAML file extensions (json, yaml, yml). Supports multi-document files.
//
// Example filepath:
// `path/to/dir` -- matches appropriate files in the directory
// `path/to/dir/file-prefix-` -- (if not also a directory) matches files in the directory that start "file-prefix-"
//
// If provided, the defaults GroupVersionKind is used to determine the k8s.Object underlying type, otherwise rely the innate typing of the scheme.
// Falls back to the unstructured.Unstructured type if a matching type cannot be found for the Kind.
//
// If handlerFn returns an error, decoding is halted.
// Patches are optional and applied after decoding and before handlerFn is executed.
//
func DecodeEachFile(ctx context.Context, filepath string, defaults *schema.GroupVersionKind, handlerFn HandlerFunc, patches ...MutateFunc) error {
	files, err := listResourceFiles(filepath)
	if err != nil {
		return err
	}
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()
		o, err := DecodeAll(f, defaults, patches...)
		if err != nil {
			return err
		}
		for _, obj := range o {
			handlerFn(ctx, obj)
		}
	}
	return nil
}

// DecodeAllFiles resolves files at the filepath (with Glob support), decoding files that match the common JSON or YAML file extensions (json, yaml, yml). Supports multi-document files.
//
// filepath may be a directory string or include an optional filename prefix, as in DecodeEachFile.
//
// If provided, the defaults GroupVersionKind is used to determine the k8s.Object underlying type, otherwise rely the innate typing of the scheme.
// Falls back to the unstructured.Unstructured type if a matching type cannot be found for the Kind.
//
// Patches are optional and applied after decoding.
func DecodeAllFiles(filepath string, defaults *schema.GroupVersionKind, patches ...MutateFunc) ([]k8s.Object, error) {
	files, err := listResourceFiles(filepath)
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
		o, err := DecodeAll(f, defaults, patches...)
		if err != nil {
			return nil, err
		}
		objects = append(objects, o...)
	}
	return objects, nil
}

// Decode a stream of documents of any Kind using either the innate typing of the scheme or the default kind, group, and version provided.
// Falls back to the unstructured.Unstructured type if a matching type cannot be found for the Kind.
// If handlerFn returns an error, decoding is halted.
// Patches are optional and applied after decoding and before handlerFn is executed.
func DecodeEach(ctx context.Context, manifest io.Reader, defaults *schema.GroupVersionKind, handlerFn HandlerFunc, patches ...MutateFunc) error {
	decoder := yaml.NewYAMLReader(bufio.NewReader(manifest))
	for {
		b, err := decoder.Read()
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return err
		}
		obj, err := DecodeAny(bytes.NewReader(b), defaults, patches...)
		if err != nil {
			return err
		}
		handlerFn(ctx, obj)
	}
	return nil
}

// Decode  a stream of  documents of any Kind using either the innate typing of the scheme or the default kind, group, and version provided.
// Falls back to the unstructured.Unstructured type if a matching type cannot be found for the Kind.
// Patches are optional and applied after decoding.
func DecodeAll(manifest io.Reader, defaults *schema.GroupVersionKind, patches ...MutateFunc) ([]k8s.Object, error) {
	decoder := yaml.NewYAMLReader(bufio.NewReader(manifest))
	objects := []k8s.Object{}
	for {
		b, err := decoder.Read()
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, err
		}
		obj, err := DecodeAny(bytes.NewReader(b), defaults, patches...)
		if err != nil {
			return nil, err
		}
		objects = append(objects, obj)
	}
	return objects, nil
}

// Decode any single-document YAML or JSON input using either the innate typing of the scheme or the default kind, group, and version provided.
// Falls back to the unstructured.Unstructured type if a matching type cannot be found for the Kind.
// Patches are optional and applied after decoding.
func DecodeAny(manifest io.Reader, defaults *schema.GroupVersionKind, patches ...MutateFunc) (k8s.Object, error) {
	k8sDecoder := serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer().Decode
	b, err := io.ReadAll(manifest)
	if err != nil {
		return nil, err
	}
	runtimeObj, _, err := k8sDecoder(b, defaults, nil)
	if runtime.IsNotRegisteredError(err) {
		// fallback to the unstructured.Unstructured type if a type is not registered for the Object to be decoded
		runtimeObj = &unstructured.Unstructured{}
		if err := yaml.Unmarshal(b, runtimeObj); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	obj, ok := runtimeObj.(k8s.Object)
	if !ok {
		return nil, err
	}
	for _, patch := range patches {
		patch(obj)
	}
	return obj, nil
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

// MutateNamespace can be used to patch objects with the given namespace name after being decoded
func MutateNamespace(namespace string) MutateFunc {
	return func(obj k8s.Object) error {
		obj.SetNamespace(namespace)
		return nil
	}
}

// CreateHandler returns a HandlerFunc that will create objects
func CreateHandler(r *Resources, opts ...CreateOption) HandlerFunc {
	return func(ctx context.Context, obj k8s.Object) error {
		return r.Create(ctx, obj, opts...)
	}
}

// GetHandler returns a HandlerFunc that will replace objects by performing a Get for the objects of the same Kind/Name
// and then calling handler to utilize the retrieved object
func GetHandler(r *Resources, handler HandlerFunc) HandlerFunc {
	return func(ctx context.Context, obj k8s.Object) error {
		name := obj.GetName()
		namespace := obj.GetNamespace()
		// use scheme.Scheme to generate a new, empty object to use as a base for decoding into
		gvk := obj.GetObjectKind().GroupVersionKind()
		o, err := scheme.Scheme.New(gvk)
		if err != nil {
			return fmt.Errorf("resource: GroupVersionKind not found in scheme: %s", gvk.String())
		}
		obj, ok := o.(k8s.Object)
		if !ok {
			return fmt.Errorf("resource: unexpected type %T in list, does not satisfy k8s.Object", obj)
		}
		if err := r.Get(ctx, name, namespace, obj); err != nil {
			return err
		}
		return handler(ctx, obj)
	}
}

// UpdateHandler returns a HandlerFunc that will update objects
func UpdateHandler(r *Resources, opts ...UpdateOption) HandlerFunc {
	return func(ctx context.Context, obj k8s.Object) error {
		return r.Update(ctx, obj, opts...)
	}
}

// DeleteHandler returns a HandlerFunc that will delete objects
func DeleteHandler(r *Resources, opts ...DeleteOption) HandlerFunc {
	return func(ctx context.Context, obj k8s.Object) error {
		return r.Delete(ctx, obj, opts...)
	}
}

// IgnoreErrorHandler returns a HandlerFunc that will ignore the provided error if the errorMatcher returns true
func IgnoreErrorHandler(handler HandlerFunc, errorMatcher func(err error) bool) HandlerFunc {
	return func(ctx context.Context, obj k8s.Object) error {
		if err := handler(ctx, obj); err != nil && !errorMatcher(err) {
			return err
		}
		return nil
	}
}

// NoopHandler returns a Handler func that only returns nil
func NoopHandler(r *Resources, opts ...DeleteOption) HandlerFunc {
	return func(ctx context.Context, obj k8s.Object) error {
		return nil
	}
}

// CreateIgnoreAlreadyExists returns a HandlerFunc that will create objects if they do not already exist
func CreateIgnoreAlreadyExists(r *Resources, opts ...CreateOption) HandlerFunc {
	return func(ctx context.Context, obj k8s.Object) error {
		return IgnoreErrorHandler(CreateHandler(r, opts...), apierrors.IsAlreadyExists)(ctx, obj)
	}
}

// DeleteIgnoreNotFound returns a HandlerFunc that will delete objects if they do not already exist
func DeleteIgnoreNotFound(r *Resources, opts ...CreateOption) HandlerFunc {
	return func(ctx context.Context, obj k8s.Object) error {
		return IgnoreErrorHandler(CreateHandler(r, opts...), apierrors.IsNotFound)(ctx, obj)
	}
}
