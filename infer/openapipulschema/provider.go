// Copyright 2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package openapipulschema

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	pulschema "github.com/cloudy-sky-software/pulschema/pkg"
	"github.com/getkin/kin-openapi/openapi3"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

const packageName = "openapi"

func Wrap(provider p.Provider, oaURL url.URL, metadata schema.Metadata) (p.Provider, error) {
	openAPILoader := openapi3.NewLoader()
	openAPILoader.IsExternalRefsAllowed = true

	openAPIDoc, err := openAPILoader.LoadFromURI(&oaURL)
	if err != nil {
		return provider, fmt.Errorf("loading OpenAPI spec at %v: %w", oaURL, err)
	}
	openAPIDoc.InternalizeRefs(context.Background(), nil)

	pkg := pschema.PackageSpec{
		Name: packageName,

		Config: pschema.ConfigSpec{
			Variables: map[string]pschema.PropertySpec{},
		},

		Provider: pschema.ResourceSpec{
			ObjectTypeSpec: pschema.ObjectTypeSpec{
				Description: fmt.Sprintf("The provider type for the %s package.", packageName),
				Type:        "object",
			},
			InputProperties: map[string]pschema.PropertySpec{},
		},

		// Will be populated when we read the OpenAPI spec.

		Types:     map[string]pschema.ComplexTypeSpec{},
		Resources: map[string]pschema.ResourceSpec{},
		Functions: map[string]pschema.FunctionSpec{},
		Language:  map[string]pschema.RawMessage{},
	}

	metadata.PopulatePackageSpec(&pkg)

	// OpenAPIContext combines the OpenAPI spec with the Pulumi package schema for pulschema.
	openAPICtx := &pulschema.OpenAPIContext{
		Doc: *openAPIDoc,
		Pkg: &pkg,
	}

	csharpNamespaces := map[string]string{}

	// populates pkg indirectly through openAPICtx
	_, _, err = openAPICtx.GatherResourcesFromAPI(csharpNamespaces)
	if err != nil {
		return provider, fmt.Errorf("generating resources from OpenAPI spec at %v: %w", oaURL, err)
	}

	schemaBytes, err := json.MarshalIndent(openAPICtx.Pkg, "", "  ")
	if err != nil {
		return provider, err
	}
	schemaString := string(schemaBytes)

	provider.GetSchema = func(_ p.Context, _ p.GetSchemaRequest) (p.GetSchemaResponse, error) {
		return p.GetSchemaResponse{
			Schema: schemaString,
		}, err
	}

	return provider, nil
}
