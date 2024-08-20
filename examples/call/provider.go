// Copyright 2023-2024, Pulumi Corporation.
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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	p "github.com/pulumi/pulumi-go-provider"
	random "github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	err := p.RunProvider("test", "1.0.0", provider)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

var provider = p.Provider{
	Call: func(_ context.Context, req p.CallRequest) (p.CallResponse, error) {
		switch req.Tok {
		case callMakePetToken:
			return callMakePet(req)
		default:
			return p.CallResponse{}, fmt.Errorf("unknown token")
		}
	},
	GetSchema: func(context.Context, p.GetSchemaRequest) (p.GetSchemaResponse, error) {
		b, err := json.Marshal(pkgSchema)
		return p.GetSchemaResponse{Schema: string(b)}, err
	},
}

var pkgSchema = schema.PackageSpec{
	Name: "test",
	Provider: schema.ResourceSpec{
		Methods: map[string]string{
			"makePet": callMakePetToken,
		},
	},
	AllowedPackageNames: []string{"pulumi"}, // Allow extending the Provider resource with a method
	Functions: map[string]schema.FunctionSpec{
		callMakePetToken: {
			Inputs: &schema.ObjectTypeSpec{
				Properties: map[string]schema.PropertySpec{
					"__self__": schema.PropertySpec{
						TypeSpec: schema.TypeSpec{Type: "string"},
					},
					"prefix": schema.PropertySpec{
						TypeSpec: schema.TypeSpec{
							Type: "string",
						},
						Default: "pre-",
					},
				},
			},
			Outputs: &schema.ObjectTypeSpec{
				Properties: map[string]schema.PropertySpec{
					"success": schema.PropertySpec{
						TypeSpec: schema.TypeSpec{
							Type: "boolean",
						},
					},
				},
			},
		},
	},
}

const callMakePetToken = "pulumi:providers:test/makePet"

func callMakePet(req p.CallRequest) (p.CallResponse, error) {
	args := req.Args
	prefix, ok := args["prefix"]
	if !ok {
		return p.CallResponse{
			Failures: []p.CheckFailure{
				{Property: "prefix", Reason: "missing"},
			},
		}, nil
	}
	if !prefix.IsString() {
		return p.CallResponse{}, fmt.Errorf("unexpected type for prefix arg: %q", prefix.TypeString())
	}
	_, err := random.NewRandomPet(req.Context, "call", &random.RandomPetArgs{
		Prefix: pulumi.String(prefix.StringValue()),
	})
	if err != nil {
		return p.CallResponse{}, err
	}
	return p.CallResponse{
		Return: resource.PropertyMap{
			"success": resource.NewProperty(true),
		},
	}, nil
}
