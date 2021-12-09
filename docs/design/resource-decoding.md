# Decoding Resources

This document proposes the design for a set of decoding functions in the resource package, intended to provide utilities for creating `k8s.Object` types from common sources of input in Go programs: files, strings, or any type that satisfies the [io.Reader](https://pkg.go.dev/io#Reader) interface.  The goal of these decoding functions is to provide an easy way for test developers to interact with Kubernetes objects in their Go tests.

## Table of Contents

1. [Motivation](#Motivation)
2. [Supported object formats](#Supported-object-formats)
3. [Goals](#Goals)
4. [Non-Goals](#Non-Goals)
5. [Design Components](#Design-Components)
    * [Patches](#Patches)
    * [Handlers](#Handlers)
    * [Decoding a single-document YAML/JSON input](#Decoding-a-single-document-YAML/JSON-input)
    * [Decoding a multi-document YAML/JSON input](#Decoding-a-multi-document-YAML/JSON-input)
        * [Decoding to a known object type](#Decoding-to-a-known-object-type)
        * [Decoding without knowing the object type](#Decoding-without-knowing-the-object-type)
6. [Decode Proposal](#Decode-Proposal)
    * [Pre-defined Decoders](#Pre-defined-Decoders)
    * [Pre-defined Helpers](#Pre-defined-Helpers)

## Motivation

When developing tests that are meant to utilize Kubernetes APIs, it is expected that you will construct a `k8s.Object` type in order to use many functions defined in the `e2e-framework` packages (as in the `klient` package).

This may be accomplished by defining Go structs, importing stdlib or third-party code.

When developing many tests, the verbosity and complexity of defining many types and understanding which packages to import may add a burden to test developers. Managing these resources as YAML or JSON has obvious benefits in regard to maintainability (and even extensibility), as they are how these resource types are traditionally represented in documentation and utilized in actual deployments.

In Go, `testdata` is a special directory that can be used to store such test fixtures, and using such a testdata directory as a source of easy-to-manage files is a common pattern associated with table-driven testing.

Finally, to help develop feature tests, it is common to need to have a set of resources created before a feature assessment begins. Similarly, deleting a set of resources may be required in a teardown step

## Supported object formats

- YAML
- JSON

## Goals

- Support decoding [single-document](https://yaml.org/spec/1.2.2/#91-documents) YAML/JSON input
- Support decoding a [multi-document](https://yaml.org/spec/1.2.2/#92-streams) YAML/JSON stream input
- Accept io.Reader interface as input

## Non-Goals

- Encoding Objects

## Design Components

### **Patches**

```go
type MutateFunc func(k8s.Object) error
```

All decoding functions accept a `patches ...MutateFunc` argument. Patches may be used to inject data after decoding is complete. If a MutateFunc returns an error, decoding is halted.

This may be done to inject dynamic data that may not be known until runtime or that may be sensitive like a locally valid credential.

Example pre-defined MutateFuncs:

```go
// apply an override set of labels to a decoded object
func MutateLabels(overrides map[string]string) MutateFunc
// apply an override set of annotations to a decoded object
func MutateAnnotations(overrides map[string]string) MutateFunc
// apply an owner annotation to a decoded object
func MutateOwnerAnnotations(owner k8s.Object) MutateFunc
```

### **Handlers**

Some decoding functions accepts a HandlerFunc, a function that is executed after decoding and the optional patches are completed per each object.

If a HandlerFunc returns an error, decoding is halted.

```go
type HandlerFunc func(context.Context, k8s.Object) error
```

Example pre-defined HandlerFuncs:

```go
// CreateHandler returns a HandlerFunc that will create objects
func CreateHandler(*Resources, opts ...CreateOption) HandlerFunc
// UpdateHandler returns a HandlerFunc that will update objects
func UpdateHandler(*Resources, opts ...UpdateOption) HandlerFunc
// DeleteHandler returns a HandlerFunc that will delete objects
func DeleteHandler(*Resources, opts ...DeleteOption) HandlerFunc

// IgnoreErrorHandler returns a HandlerFunc that will ignore an error
func IgnoreErrorHandler(HandlerFunc, error) HandlerFunc

// CreateIfNotExistsHandler returns a HandlerFunc that will create objects if they do not already exist
func CreateIfNotExistsHandler(*Resources, opts ...CreateOption) HandlerFunc
```

### Decoding a single-document YAML/JSON input

The following are alternative ideas to implementing decoding input that contain a single `k8s.Object` type. Example:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-serivce-account
  namespace: myappns
```

1. Decoding an object to a known type

```go
func Decode(manifest io.Reader, obj k8s.Object, patches ...MutateFunc) error
```

Usage:

```go
sa := v1.ServiceAccount{}
err := Decode(strings.NewReader("..."), &sa)
```

With a MutateFunc:

```go
// Decode to sa and apply the label "test" : "feature-X"
sa := v1.ServiceAccount{}
err := Decode(strings.NewReader("..."), &sa, MutateLabels(map[string]string{"test" : "feature-X"}))
```

2. Decoding an object without knowing the type

`defaults` is an optional parameter, if specified, it is a hint to the decoder to help determine the underlying Go type to use for object creation.

```go
func DecodeAny(manifest io.Reader, defaults *schema.GroupVersionKind, patches ...MutateFunc) (k8s.Object, error)
```

Usage:

```go
obj, err := DecodeAny(strings.NewReader("..."), nil)
if err != nil {
    ...
}
if sa, ok := obj.(*v1.ServiceAccount); ok {
    ...
}
```

With defaults:

```go
obj, err := DecodeAny(strings.NewReader("..."), schema.GroupVersionKind{Version: "v1", Kind: "ServiceAccount"})
if err != nil {
    ...
}
if sa, ok := obj.(*v1.ServiceAccount); ok {
    ...
}
```

### Decoding a multi-document YAML/JSON input

The following are alternative ideas to implementing decoding input that may contain multiple distinct `k8s.Object` types. Example:

```yaml
## testdata/test-setup.yaml
apiVersion: v1
kind: Namespace
name: myappns
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
  namespace: myappns
data:
  appconfig.json: |
    key: value
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-serivce-account
  namespace: myappns
```


#### **Decoding to a known object type**

The following are variations on decoding an input to a known Go type.

Example parameters: 
- `obj k8s.Object` - [*v1.ServiceAccount](https://pkg.go.dev/k8s.io/api/core/v1#ServiceAccount)
- `list k8s.ObjectList` - [*v1.ServiceAccountList](https://pkg.go.dev/k8s.io/api/core/v1#ServiceAccountList)).

If an input contains multiple Kinds of object, `Unstructured` may be used (Resource data is accessible via `map[string]interface{}`):
- `obj k8s.Object` - [`*unstructured.Unstructured`](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1/unstructured)
- `list k8s.ObjectList` - [`*unstructured.UnstructuredList`](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1/unstructuredList)

Decoding into the wrong object **will** provide unexpected results.

1. Use the provided `k8s.Object` type as a base for decoding each YAML document.

```go
func DecodeObjects(manifest io.Reader, obj k8s.Object, patches ...MutateFunc) ([]k8s.Object, error)
```

Usage:

```go
objects, err := DecodeObjects(strings.NewReader("..."), &unstructured.Unstructured{})
for _, obj := range objects {
    err := klient.Create(obj)
    ...
}
```

2. Decode into the provided `k8s.ObjectList`

```go
func DecodeList(manifest io.Reader, obj k8s.ObjectList, patches ...MutateFunc) error
```

Usage:

```go
list := &unstructured.UnstructuredList{}
err := DecodeList(strings.NewReader("..."), list)
for _, obj := range list.Items {
    err := klient.Create(obj)
    ...
}
```

3. Decode each document into the provided `k8s.Object` base and call handlerFn for each processed object. If `handlerFn` returns an error, decoding is halted.

```go
func DecodeEachDocument(manifest io.Reader, base k8s.Object, handlerFn HandlerFunc, patches ...MutateFunc) error
```

Usage:

```go
err := DecodeEachDocument(strings.NewReader("..."), &unstructured.Unstructured{}, func(ctx context.Context, obj ks8.Object) error {
    return klient.Create(obj)
})
```

Usage with pre-defined HandlerFunc:

```go
err := DecodeEachDocument(strings.NewReader("..."), &unstructured.Unstructured{}, CreateHandler(klient.Resources(namespace)))
```

Usage with pre-defined HandlerFunc and MutateFuncs:

```go
err := DecodeEachDocument(
    strings.NewReader("..."),
    &unstructured.Unstructured{},
    CreateHandler(klient.Resources(namespace)), // create each decoded object after applying the following patches
    MutateLabels(map[string]string{"test" : "feature-X"}), MutateAnnotations(map[string]string{"test" : "feature-X"}),
)
```

#### **Decoding without knowing the object type**

The following options would use the types registered with `scheme.Scheme` to help deserialize objects into a `k8s.Object` with the expected underlying API type.

`defaults` is an optional parameter, if specified, it is a hint to the decoder to help determine the underlying Go type to use for object creation.

1. Decode each document and call handlerFn for each processed object. If `handlerFn` returns an error, decoding is halted.

```go
func DecodeEach(manifest io.Reader, defaults *schema.GroupVersionKind, handlerFn HandlerFunc, patches ...MutateFunc) error
```

Usage:


```go
list := &unstructured.UnstructuredList{}
err := DecodeEach(strings.NewReader("..."), nil, func(ctx context.Context, obj ks8.Object) error {
    if cfg, ok := obj.(*v1.ConfigMap); ok {
        // obj is a ConfigMap
    } else if svc, ok := obj.(*v1.ServiceAccount); ok {
        // obj is a ServiceAccount
    }
    return klient.Create(obj)
})
```

Usage with pre-defined HandlerFunc:

```go
err := DecodeEach(strings.NewReader("..."), nil, CreateHandler(klient.Resources(namespace)))
```


2. Decode all documents.

```go
func DecodeAll(manifest io.Reader, defaults *schema.GroupVersionKind, patches ...MutateFunc) ([]k8s.Object, error)
```

Usage:

```go
objects, err := DecodeAll(strings.NewReader("..."), nil)
for _, obj := range objects {
    err := klient.Create(obj)
    ...
}
```

## Decode Proposal

The following is a final proposal on the function signatures, after considering the above options:

```go
// Decode a single-document YAML or JSON input into a known type.
// Patches are optional and applied after decoding.
func Decode(manifest io.Reader, obj k8s.Object, patches ...MutateFunc) error

// Decode any single-document YAML or JSON input using either the innate typing of the scheme or the default kind, group, and version provided.
// Patches are optional and applied after decoding.
func DecodeAny(manifest io.Reader, defaults *schema.GroupVersionKind, patches ...MutateFunc) (k8s.Object, error)

// Decode a stream of documents of any Kind using either the innate typing of the scheme or the default kind, group, and version provided. 
// If handlerFn returns an error, decoding is halted.
// Patches are optional and applied after decoding and before handlerFn is executed.
func DecodeEach(ctx context.Context, manifest io.Reader, defaults *schema.GroupVersionKind, handlerFn HandlerFunc, patches ...MutateFunc) error

// Decode  a stream of  documents of any Kind using either the innate typing of the scheme or the default kind, group, and version provided.
// Patches are optional and applied after decoding.
func DecodeAll(manifest io.Reader, defaults *schema.GroupVersionKind, patches ...MutateFunc) ([]k8s.Object, error)
```

Using a typed object when decoding multiple documents does not provide for an easy-to-use interface, so they are not being proposed at this time.

### Pre-defined Decoders

Building on the proposal, the following functions would be included that build on the base decoders:

```go
// Decode the file at the given manifest path into the provided object. Patches are optional and applied after decoding.
func DecodeFile(manifestPath string, obj k8s.Object, patches ...MutateFunc) error

// Decode the manifest string into the provided object. Patches are optional and applied after decoding.
func DecodeString(rawManifest string, obj k8s.Object, patches ...MutateFunc) error

// Decode the manifest bytes into the provided object. Patches are optional and applied after decoding.
func DecodeBytes(manifestBytes []byte, obj k8s.Object, patches ...MutateFunc) error

// DecodeEachFile resolves files at the filepath (with Glob support), decoding files that match the common JSON or YAML file extensions (json, yaml, yml). Supports multi-document files.
//
// Example filepath: 
// `path/to/dir` -- matches appropriate files in the directory
// `path/to/dir/file-prefix-` -- (if not also a directory) matches files in the directory that start "file-prefix-"
//
// If handlerFn returns an error, decoding is halted.
// Patches are optional and applied after decoding and before handlerFn is executed.
//
// If provided, the defaults GroupVersionKind is used to determine the k8s.Object underlying type, otherwise rely the innate typing of the scheme.
func DecodeEachFile(ctx context.Context, filepath string, defaults *schema.GroupVersionKind, handlerFn HandlerFunc, patches ...MutateFunc) error

// DecodeAllFiles resolves files at the filepath (with Glob support), decoding files that match the common JSON or YAML file extensions (json, yaml, yml). Supports multi-document files.
//
// filepath may be a directory string or include an optional filename prefix, as in DecodeEachFile.
//
// Patches are optional and applied after decoding.
//
// If provided, the defaults GroupVersionKind is used to determine the k8s.Object underlying type, otherwise rely the innate typing of the scheme.
func DecodeAllFiles(filepath string, defaults *schema.GroupVersionKind, patches ...MutateFunc) ([]k8s.Object, error)
```
### Pre-defined Helpers

```go
// CreateHandler returns a HandlerFunc that will create objects
func CreateHandler(r *Resources, opts ...CreateOption) HandlerFunc
// UpdateHandler returns a HandlerFunc that will update objects
func UpdateHandler(r *Resources, opts ...UpdateOption) HandlerFunc
// DeleteHandler returns a HandlerFunc that will delete objects
func DeleteHandler(r *Resources, opts ...DeleteOption) HandlerFunc

// IgnoreErrorHandler returns a HandlerFunc that will ignore the provided error
func IgnoreErrorHandler(HandlerFunc, error) HandlerFunc

// CreateIfNotExistsHandler returns a HandlerFunc that will create objects if they do not already exist
func CreateIfNotExistsHandler(r *Resources, opts ...CreateOption) HandlerFunc

// apply an override set of labels to a decoded object
func MutateLabels(overrides map[string]string) MutateFunc
// apply an override set of annotations to a decoded object
func MutateAnnotations(overrides map[string]string) MutateFunc
// apply an owner annotation to a decoded object
func MutateOwnerAnnotations(owner k8s.Object) MutateFunc
```
