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

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	comProvider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
)

const componentType = "test:index:Component"
const methodType = componentType + "/myMethod"

func main() {
	if err := p.RunProvider("test", "0.1.0", p.Provider{
		Construct: func(ctx context.Context, req p.ConstructRequest) (p.ConstructResponse, error) {
			if t := req.Urn.Type(); t != componentType {
				return p.ConstructResponse{}, fmt.Errorf("unknown component type %q", t)
			}

			return p.ProgramConstruct(ctx, req, func(
				ctx *pulumi.Context, typ, name string, inputs comProvider.ConstructInputs, opts pulumi.ResourceOption,
			) (*comProvider.ConstructResult, error) {
				r := new(testComponent)
				err := inputs.CopyTo(r)
				if err != nil {
					return nil, err
				}

				err = ctx.RegisterComponentResource("test:index:Component", "test", r, opts)
				if err != nil {
					return nil, err
				}

				pet, err := random.NewRandomPet(ctx, "pet", &random.RandomPetArgs{
					Prefix: r.MyInput,
				}, pulumi.Parent(r))
				if err != nil {
					return nil, err
				}

				r.MyOutput = pet.ID().ToStringPtrOutput()
				err = ctx.RegisterResourceOutputs(r, pulumi.ToMap(map[string]any{
					"myOutput": r.MyOutput,
				}))
				if err != nil {
					return nil, err
				}

				return comProvider.NewConstructResult(r)
			})
		},
		Call: func(ctx context.Context, req p.CallRequest) (p.CallResponse, error) {
			if req.Tok != methodType {
				return p.CallResponse{}, fmt.Errorf("unknown token %q", req.Tok)
			}

			return p.ProgramCall(ctx, req, func(ctx *pulumi.Context, tok string, args comProvider.CallArgs,
			) (*comProvider.CallResult, error) {
				pet, err := random.NewRandomPet(ctx, "call-pet", &random.RandomPetArgs{})
				if err != nil {
					return nil, err
				}

				callArgs := testCallArgs{}
				self, err := args.CopyTo(&callArgs)
				if err != nil {
					return nil, err
				}

				resp1 := pulumi.All(self.URN(), callArgs.Arg1, pet.ID()).ApplyT(func(vs []any) (string, error) {
					urn := vs[0].(pulumi.URN)
					arg1 := vs[1].(string)
					id := vs[2].(pulumi.ID)
					return arg1 + ":" + string(urn) + ":" + string(id), nil
				}).(pulumi.StringOutput)

				result := testCallResult{
					Resp1: resp1,
				}

				return comProvider.NewCallResult(&result)
			})
		},
		GetSchema: func(ctx context.Context, _ p.GetSchemaRequest) (p.GetSchemaResponse, error) {
			return p.GetSchemaResponse{
				Schema: testSchema,
			}, nil
		},
	}); err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
}

type testComponent struct {
	pulumi.ResourceState
	MyInput  pulumi.StringPtrOutput `pulumi:"myInput"`
	MyOutput pulumi.StringPtrOutput `pulumi:"myOutput"`
}

type testCallArgs struct {
	Arg1 pulumi.StringInput `pulumi:"arg1"`
}

type testCallResult struct {
	Resp1 pulumi.StringOutput `pulumi:"resp1"`
}

var testSchema = marshalSchema(schema.PackageSpec{
	Name: "test",
	Resources: map[string]schema.ResourceSpec{
		componentType: {
			IsComponent: true,
			InputProperties: map[string]schema.PropertySpec{
				"myInput": {
					TypeSpec: schema.TypeSpec{
						Type: "string",
					},
				},
			},
			ObjectTypeSpec: schema.ObjectTypeSpec{
				Properties: map[string]schema.PropertySpec{
					"myOutput": {
						TypeSpec: schema.TypeSpec{
							Type: "string",
						},
					},
				},
			},
			Methods: map[string]string{
				"myMethod": methodType,
			},
		},
	},
	Functions: map[string]schema.FunctionSpec{
		methodType: {
			Inputs: &schema.ObjectTypeSpec{
				Properties: map[string]schema.PropertySpec{
					"__self__": {
						TypeSpec: schema.TypeSpec{
							Ref: "#/resources/" + componentType,
						},
					},
					"arg1": {
						TypeSpec: schema.TypeSpec{
							Type: "string",
						},
					},
				},
			},
			Outputs: &schema.ObjectTypeSpec{
				Properties: map[string]schema.PropertySpec{
					"resp1": {
						TypeSpec: schema.TypeSpec{
							Type: "string",
						},
					},
				},
			},
		},
	},
})

func marshalSchema(s schema.PackageSpec) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	err := enc.Encode(s)
	if err != nil {
		panic(err)
	}
	return b.String()
}
