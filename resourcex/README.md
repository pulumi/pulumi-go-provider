# resourcex

The `resourcex` package extends the `github.com/pulumi/pulumi/sdk/v3/go/common/resource` package with helpers
for working with property values.

1. `Unmarshal` - Extract structured values from a property map, with tracking of unknownness and secretness.
2. `Decode` - Decode a property map into a JSON-like structure containing only values.
3. `DecodeValue` - Decode a property value into its underlying value, recursively.
4. `Traverse` - Traverse a property path, visiting each property value.

## Unmarshaling

The `Unmarshal`  function is designed to extract subsets of values from property map using structs. 
Information about the unknownness and secretness of the extracted values is provided, e.g. to annotate output properties.

Here's an example of decoding a property map into various structures, observing how unknownness and secretness varies:

```go
res1 := resource.URN("urn:pulumi:test::test::kubernetes:core/v1:Namespace::some-namespace")

props := resource.PropertyMap{
    "chart":   resource.NewStringProperty("nginx"),
    "version": resource.NewStringProperty("1.24.0"),
    "repositoryOpts": resource.NewObjectProperty(resource.PropertyMap{
        "repo":     resource.NewStringProperty("https://charts.bitnami.com/bitnami"),
        "username": resource.NewStringProperty("username"),
        "password": resource.NewSecretProperty(&resource.Secret{
            Element: resource.NewStringProperty("password"),
        }),
        "other": resource.MakeComputed(resource.NewStringProperty("")),
    }),
    "namespace": resource.NewOutputProperty(resource.Output{
        Element:      resource.NewStringProperty(""),
        Known:        false,
        Secret:       true,
        Dependencies: []resource.URN{res1},
    }),
    "args": resource.NewArrayProperty([]resource.PropertyValue{
        resource.NewObjectProperty(resource.PropertyMap{
            "name":  resource.NewStringProperty("a"),
            "value": resource.MakeSecret(resource.NewStringProperty("a")),
        }),
        resource.MakeComputed(resource.NewObjectProperty(resource.PropertyMap{})),
        resource.NewObjectProperty(resource.PropertyMap{
            "name":  resource.NewStringProperty("c"),
            "value": resource.MakeSecret(resource.NewStringProperty("c")),
        }),
    }),
}

type RepositoryOpts struct {
    // Repository where to locate the requested chart.
    Repo string `json:"repo,omitempty"`
    // The Repositories CA File
    CAFile string `json:"caFile,omitempty"`
    // The repositories cert file
    CertFile string `json:"certFile,omitempty"`
    // The repositories cert key file
    KeyFile string `json:"keyFile,omitempty"`
    // Password for HTTP basic authentication
    Password string `json:"password,omitempty"`
    // Username for HTTP basic authentication
    Username string `json:"username,omitempty"`
}

type Loader struct {
    Chart            string          `json:"chart,omitempty"`
    DependencyUpdate *bool           `json:"dependencyUpdate,omitempty"`
    Version          string          `json:"version,omitempty"`
    RepositoryOpts   *RepositoryOpts `json:"repositoryOpts,omitempty"`
}

// Example: Chart Loader
loader := &Loader{}
result, err := Unmarshal(loader, props, UnmarshalOptions{RejectUnknowns: false})
assert.NoError(t, err)
assert.Equal(t, UnmarshalResult{ContainsUnknowns: false, ContainsSecrets: true}, result)

// Example: anonymous struct (version)
var version struct {
    Version string `json:"version"`
}
result, err = Unmarshal(&version, props, UnmarshalOptions{RejectUnknowns: false})
assert.NoError(t, err)
assert.Equal(t, "1.24.0", version.Version)
assert.Equal(t, UnmarshalResult{ContainsUnknowns: false, ContainsSecrets: false}, result)

// Example: anonymous struct ("namespace")
var namespace struct {
    Namespace string `json:"namespace"`
}
result, err = Unmarshal(&namespace, props, UnmarshalOptions{RejectUnknowns: false})
assert.NoError(t, err)
assert.Equal(t, "", namespace.Namespace)
assert.Equal(t,
    UnmarshalResult{ContainsUnknowns: true, ContainsSecrets: true, Dependencies: []resource.URN{res1}}, result)

// Example: unset property ("dependencyUpdate")
var dependencyUpdate struct {
    DependencyUpdate *bool `json:"dependencyUpdate"`
}
result, err = Unmarshal(&dependencyUpdate, props, UnmarshalOptions{RejectUnknowns: false})
assert.NoError(t, err)
assert.Nil(t, dependencyUpdate.DependencyUpdate)
assert.Equal(t, UnmarshalResult{ContainsUnknowns: false, ContainsSecrets: false}, result)

// Example: arrays
type Arg struct {
    Name  string `json:"name"`
    Value string `json:"value"`
}
var args struct {
    Args []*Arg `json:"args"`
}
result, err = Unmarshal(&args, props, UnmarshalOptions{RejectUnknowns: false})
assert.NoError(t, err)
assert.Equal(t, []*Arg{{Name: "a", Value: "a"}, nil, {Name: "c", Value: "c"}}, args.Args)
assert.Equal(t, UnmarshalResult{ContainsUnknowns: true, ContainsSecrets: true}, result)

// Example: arrays (names only)
type ArgNames struct {
    Name string `json:"name"`
}
var argNames struct {
    Args []*ArgNames `json:"args"`
}
result, err = Unmarshal(&argNames, props, UnmarshalOptions{RejectUnknowns: false})
assert.NoError(t, err)
assert.Equal(t, []*ArgNames{{Name: "a"}, nil, {Name: "c"}}, argNames.Args)
assert.Equal(t, UnmarshalResult{ContainsUnknowns: true, ContainsSecrets: false}, result)
```

## Decoding

The `Decode` function decodes a property map into a JSON-like map structure containing pure values.
Unknown and computed values are decoded to `null`, both for objects and for arrays.

The following property value types are supported: `Bool`, `Number`, `String`, `Array`, `Computed`, 
`Output`, `Secret`, `Object`.

The following property value types are NOT supported: `Asset`, `Archive`, `ResourceReference`.

Here's an example:

```go
res1 := resource.URN("urn:pulumi:test::test::kubernetes:core/v1:Namespace::some-namespace")

props := resource.PropertyMap{
    "chart":   resource.NewStringProperty("nginx"),
    "version": resource.NewStringProperty("1.24.0"),
    "repositoryOpts": resource.NewObjectProperty(resource.PropertyMap{
        "repo":     resource.NewStringProperty("https://charts.bitnami.com/bitnami"),
        "username": resource.NewStringProperty("username"),
        "password": resource.NewSecretProperty(&resource.Secret{
            Element: resource.NewStringProperty("password"),
        }),
        "other": resource.MakeComputed(resource.NewStringProperty("")),
    }),
    "namespace": resource.NewOutputProperty(resource.Output{
        Element:      resource.NewStringProperty(""),
        Known:        false,
        Secret:       true,
        Dependencies: []resource.URN{res1},
    }),
    "args": resource.NewArrayProperty([]resource.PropertyValue{
        resource.NewObjectProperty(resource.PropertyMap{
            "name":  resource.NewStringProperty("a"),
            "value": resource.MakeSecret(resource.NewStringProperty("a")),
        }),
        resource.MakeComputed(resource.NewObjectProperty(resource.PropertyMap{})),
        resource.NewObjectProperty(resource.PropertyMap{
            "name":  resource.NewStringProperty("c"),
            "value": resource.MakeSecret(resource.NewStringProperty("c")),
        }),
    }),
}

decoded := Decode(props)
assert.Equal(t, map[string]any{
    "chart":   "nginx",
    "version": "1.24.0",
    "repositoryOpts": map[string]any{
        "repo":     "https://charts.bitnami.com/bitnami",
        "username": "username",
        "password": "password",
        "other":    nil,
    },
    "namespace": nil,
    "args": []any{
        map[string]any{
            "name":  "a",
            "value": "a",
        },
        nil,
        map[string]any{
            "name":  "c",
            "value": "c",
        },
    },
}, decoded)
```

Similarly, the `DecodeValue` function decodes a single property value into its pure value. Here's an example:

```go
prop := resource.NewArrayProperty([]resource.PropertyValue{
    resource.NewObjectProperty(resource.PropertyMap{
        "name":  resource.NewStringProperty("a"),
        "value": resource.MakeSecret(resource.NewStringProperty("b")),
    }),
    resource.MakeComputed(resource.NewObjectProperty(resource.PropertyMap{})),
    resource.NewObjectProperty(resource.PropertyMap{
        "name":  resource.NewStringProperty("c"),
        "value": resource.MakeSecret(resource.NewStringProperty("d")),
    }),
})

decoded := DecodeValue(prop)
assert.Equal(t, []any{
    map[string]any{
        "name":  "a",
        "value": "b",
    },
    nil,
    map[string]any{
        "name":  "c",
        "value": "d",
    },
}, decoded)
```


## Traversal

The `Traverse` function traverses a property map along the given property path, 
invoking a callback function for each property value it encounters, including the map itself.

Property paths are based on `resource.PropertyPath`.

A wildcard may be used as an array index to traverse all elements of the array.

Examples of valid paths:
 - `root`
 - `root.nested`
 - `root.double.nest`
 - `root.array[0]`
 - `root.array[100]`
 - `root.array[0].nested`
 - `root.array[0][1].nested`
 - `root.nested.array[0].double[1]`
 - `root.array[*]`
 - `root.array[*].field`

For example, given this property map:

```go
props := /* A */ resource.NewObjectProperty(resource.PropertyMap{
    "chart":   resource.NewStringProperty("nginx"),
    "version": resource.NewStringProperty("1.24.0"),
    "repositoryOpts": /* B */ resource.NewObjectProperty(resource.PropertyMap{
        "repo":     resource.NewStringProperty("https://charts.bitnami.com/bitnami"),
        "username": resource.NewStringProperty("username"),
        "password": /* C */ resource.NewSecretProperty(&resource.Secret{
            Element: /* D */ resource.NewStringProperty("password"),
        }),
    }),
})
```

Traversing the path `repositoryOpts.password` would invoke the callback function for each of the following values:
1. the root-level property value (A)
2. the "object" property value (B)
3. the "secret" property value (C)
4. the "string" property value (D)
