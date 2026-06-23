// This example shows how to write a parameterized provider with
// [github.com/pulumi/pulumi-go-provider].
//
// The provider is parameterized by a file path:
//
//	pulumi package add ./pulumi-resource-remember ./path/to/file
//
// During parameterization the provider reads the file and remembers its contents. It
// then serves a schema named after the file (without its extension) with a single
// function called `remember`, which returns the file's contents captured
// at parameterize time.
//
// The whole file's contents are embedded (base64 encoded) into the schema as the
// parameterization parameter, so that re-parameterization from a generated SDK recovers
// the contents without reading the file again.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"

	p "github.com/pulumi/pulumi-go-provider"
)

// baseName is the name of the un-parameterized base provider.
const baseName = "remember"

// version is the version of both the base provider and every parameterization of it.
var version = semver.MustParse("0.1.0")

func main() {
	if err := p.RunProvider(context.Background(), baseName, version.String(), provider()); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
}

// remembered is the file captured during Parameterize.
//
// It is shared between Parameterize (which fills it in), GetSchema and Invoke (which
// read it back out).
type remembered struct {
	// name is the parameterized package name: the file's name without its extension.
	name string
	// contents is the full contents of the file, captured during Parameterize.
	contents []byte
}

// provider builds a parameterized provider that remembers the contents of a file.
func provider() p.Provider {
	var state remembered
	return p.Provider{
		Parameterize: state.parameterize,
		GetSchema:    state.getSchema,
		Invoke:       state.invoke,
	}
}

// parameterize captures the file that the provider will remember.
//
// It handles both initial parameterization from the CLI ([p.ParameterizeRequest.Args])
// and re-parameterization from a generated SDK ([p.ParameterizeRequest.Value]).
func (s *remembered) parameterize(
	_ context.Context, req p.ParameterizeRequest,
) (p.ParameterizeResponse, error) {
	switch {
	case req.Args != nil:
		if len(req.Args.Args) != 1 {
			return p.ParameterizeResponse{}, fmt.Errorf(
				"expected exactly one argument (the file path), got %d", len(req.Args.Args))
		}
		path := req.Args.Args[0]
		contents, err := os.ReadFile(path) //nolint:gosec // G304: reading the parameterized file is the point.
		if err != nil {
			return p.ParameterizeResponse{}, fmt.Errorf("reading %q: %w", path, err)
		}
		// Use the file's name without its extension as the package name, since a
		// Pulumi package name may not contain a '.'.
		base := filepath.Base(path)
		s.name = strings.TrimSuffix(base, filepath.Ext(base))
		s.contents = contents
	case req.Value != nil:
		// On re-parameterization the contents are recovered from the parameter that was
		// embedded into the schema during initial parameterization.
		s.name = req.Value.Name
		s.contents = req.Value.Value
	default:
		return p.ParameterizeResponse{}, fmt.Errorf("missing parameterization arguments")
	}

	return p.ParameterizeResponse{
		Name:    s.name,
		Version: version,
	}, nil
}

// rememberToken is the token of the `remember` function for a given package name.
func rememberToken(pkg string) tokens.Type {
	return tokens.NewTypeToken(tokens.NewModuleToken(tokens.Package(pkg), "index"), "remember")
}

// getSchema serves the schema for the (possibly parameterized) provider.
//
// Before parameterization it serves the bare base provider. After parameterization it
// serves a schema named after the file with the `remember` function, embedding the file
// contents (base64 encoded) as the parameterization parameter.
func (s *remembered) getSchema(
	_ context.Context, _ p.GetSchemaRequest,
) (p.GetSchemaResponse, error) {
	spec := schema.PackageSpec{
		Name:    baseName,
		Version: version.String(),
		Description: "A parameterized provider that remembers the contents of a file " +
			"captured at parameterize time.",
	}

	if s.name != "" {
		spec.Name = s.name
		spec.Parameterization = &schema.ParameterizationSpec{
			BaseProvider: schema.BaseProviderSpec{
				Name:    baseName,
				Version: version.String(),
			},
			Parameter: s.contents,
		}
		spec.Functions = map[string]schema.FunctionSpec{
			rememberToken(s.name).String(): {
				Description: "Return the contents of " + s.name +
					" as captured when the provider was parameterized.",
				Inputs: &schema.ObjectTypeSpec{Type: "object"},
				Outputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"contents": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
					Required: []string{"contents"},
				},
			},
		}
	}

	bytes, err := json.Marshal(spec)
	if err != nil {
		return p.GetSchemaResponse{}, err
	}
	return p.GetSchemaResponse{Schema: string(bytes)}, nil
}

// invoke implements the `remember` function, returning the remembered file contents.
func (s *remembered) invoke(
	_ context.Context, req p.InvokeRequest,
) (p.InvokeResponse, error) {
	if req.Token.Name() != "remember" {
		return p.InvokeResponse{}, fmt.Errorf("unknown function %q", req.Token)
	}
	return p.InvokeResponse{
		Return: property.NewMap(map[string]property.Value{
			"contents": property.New(string(s.contents)),
		}),
	}, nil
}
