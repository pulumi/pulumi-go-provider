# resourcex

The `resourcex` package extends the `github.com/pulumi/pulumi/sdk/v3/go/common/resource` package with helpers
for working with property values.

1. `Extract` - Extract structured values from a property map, with tracking of unknownness and secretness.
2. `DecodeValues` - Decode a property map into a JSON-like structure containing only values.
3. `Traverse` - Traverse a property path, visiting each property value.

## Extraction

The `Extract`  function is designed to extract subsets of values from property map using structs. 
Information about the unknownness and secretness of the extracted values is provided,  e.g. to annotate output properties.

Here's a real-world example:

```go

func Test_Extract_Example(t *testing.T) {

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
			resource.MakeComputed(resource.NewObjectProperty(resource.PropertyMap{})),
			resource.NewObjectProperty(resource.PropertyMap{
				"name":  resource.NewStringProperty("a"),
				"value": resource.MakeComputed(resource.NewStringProperty("")),
			}),
			resource.NewObjectProperty(resource.PropertyMap{
				"name":  resource.NewStringProperty("b"),
				"value": resource.MakeSecret(resource.NewStringProperty("b")),
			}),
		}),
	}

	type RepositoryOpts struct {
		// Repository where to locate the requested chart. If is a URL the chart is installed without installing the repository.
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

	type Arg struct {
		Name  string `json:"name,omitempty"`
		Value string `json:"value,omitempty"`
	}

	type Loader struct {
		Chart            string          `json:"chart,omitempty"`
		DependencyUpdate *bool           `json:"dependencyUpdate,omitempty"`
		Version          string          `json:"version,omitempty"`
		RepositoryOpts   *RepositoryOpts `json:"repositoryOpts,omitempty"`
	}

	// EXAMPLE: Chart Loader
	loader := &Loader{}
	result, err := Extract(loader, props, ExtractOptions{RejectUnknowns: false})
	assert.NoError(t, err)
	assert.Equal(t, ExtractResult{ContainsUnknowns: false, ContainsSecrets: true}, result)
	t.Logf("\n%+v", result)

	// EXAMPLE: anonymous struct (version)
	version := struct {
		Version string `json:"version"`
	}{}
	result, err = Extract(&version, props, ExtractOptions{RejectUnknowns: false})
	assert.NoError(t, err)
	assert.Equal(t, "1.24.0", version.Version)
	assert.Equal(t, ExtractResult{ContainsUnknowns: false, ContainsSecrets: false}, result)
	t.Logf("\n%+v\n%+v", version, result)

	// EXAMPLE: anonymous struct (namespace)
	namespace := struct {
		Namespace string `json:"namespace"`
	}{}
	result, err = Extract(&namespace, props, ExtractOptions{RejectUnknowns: false})
	assert.NoError(t, err)
	assert.Equal(t, "", namespace.Namespace)
	assert.Equal(t, ExtractResult{ContainsUnknowns: true, ContainsSecrets: true, Dependencies: []resource.URN{res1}}, result)
	t.Logf("\n%+v\n%+v", namespace, result)

	// EXAMPLE: not present (dependencyUpdate, optional)
	dependencyUpdate := struct {
		DependencyUpdate *bool `json:"dependencyUpdate"`
	}{}
	result, err = Extract(&dependencyUpdate, props, ExtractOptions{RejectUnknowns: false})
	assert.NoError(t, err)
	assert.Nil(t, dependencyUpdate.DependencyUpdate)
	assert.Equal(t, ExtractResult{ContainsUnknowns: false, ContainsSecrets: false}, result)
	t.Logf("\n%+v\n%+v", dependencyUpdate, result)

	// EXAMPLE: arrays
	args := struct {
		Args []Arg `json:"args"`
	}{}
	result, err = Extract(&args, props, ExtractOptions{RejectUnknowns: false})
	assert.NoError(t, err)
	assert.Equal(t, []Arg{{Name: "", Value: ""}, {Name: "a", Value: ""}, {Name: "b", Value: "b"}}, args.Args)
	assert.Equal(t, ExtractResult{ContainsUnknowns: true, ContainsSecrets: true}, result)
	t.Logf("\n%+v\n%+v", args, result)

}
```