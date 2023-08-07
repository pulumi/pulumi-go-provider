// Copyright 2022, Pulumi Corporation.
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

package tests

import (
	"fmt"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/integration"
)

func urn(typ, name string) resource.URN {
	return resource.NewURN("stack", "proj", "",
		tokens.Type("test:index:"+typ), tokens.QName(name))
}

type Echo struct{}
type EchoInputs struct {
	String string            `pulumi:"string"`
	Int    int               `pulumi:"int"`
	Map    map[string]string `pulumi:"strMap,optional"`
}
type EchoOutputs struct {
	EchoInputs
	Name      string            `pulumi:"nameOut"`
	StringOut string            `pulumi:"stringOut"`
	IntOut    int               `pulumi:"intOut"`
	MapOut    map[string]string `pulumi:"strMapOut,optional"`
}

func (*Echo) Create(ctx p.Context, name string, inputs EchoInputs, preview bool) (string, EchoOutputs, error) {
	id := name + "-id"
	state := EchoOutputs{EchoInputs: inputs}
	if preview {
		return id, state, nil
	}
	state.Name = name
	state.StringOut = inputs.String
	state.IntOut = inputs.Int
	state.MapOut = inputs.Map
	return id, state, nil
}

func (*Echo) Update(ctx p.Context, id string, olds EchoOutputs, news EchoInputs, preview bool) (EchoOutputs, error) {
	if preview {
		return olds, nil
	}

	return EchoOutputs{
		EchoInputs: news,
		Name:       olds.Name,
		StringOut:  news.String,
		IntOut:     news.Int,
		MapOut:     news.Map,
	}, nil
}

var _ = (infer.ExplicitDependencies[WiredInputs, WiredOutputs])((*Wired)(nil))

type Wired struct{}
type WiredInputs struct {
	String string `pulumi:"string"`
	Int    int    `pulumi:"int"`
}
type WiredOutputs struct {
	Name         string `pulumi:"name"`
	StringAndInt string `pulumi:"stringAndInt"`
	StringPlus   string `pulumi:"stringPlus"`
}

func (*Wired) Create(ctx p.Context, name string, inputs WiredInputs, preview bool) (string, WiredOutputs, error) {
	id := name + "-id"
	state := WiredOutputs{Name: "(" + name + ")"}
	if preview {
		return id, state, nil
	}
	state.StringPlus = inputs.String + "+"
	state.StringAndInt = fmt.Sprintf("%s-%d", inputs.String, inputs.Int)
	return id, state, nil
}

func (*Wired) Update(
	ctx p.Context, id string, olds WiredOutputs, news WiredInputs, preview bool,
) (WiredOutputs, error) {
	return WiredOutputs{
		Name:         id,
		StringAndInt: fmt.Sprintf("%s-%d", news.String, news.Int),
		StringPlus:   news.String + "++",
	}, nil
}

func (*Wired) WireDependencies(f infer.FieldSelector, a *WiredInputs, s *WiredOutputs) {
	stringIn := f.InputField(&a.String)
	intIn := f.InputField(&a.Int)

	name := f.OutputField(&s.Name)
	stringAndInt := f.OutputField(&s.StringAndInt)
	stringOut := f.OutputField(&s.StringPlus)

	name.AlwaysKnown()            // This is based on the pulumi name, which is always known
	stringOut.DependsOn(stringIn) // Passthrough value with a mutation
	stringAndInt.DependsOn(stringIn)
	stringAndInt.DependsOn(intIn)

}

func provider() integration.Server {
	p := infer.Provider(infer.Options{
		Resources: []infer.InferredResource{
			infer.Resource[*Echo, EchoInputs, EchoOutputs](),
			infer.Resource[*Wired, WiredInputs, WiredOutputs](),
		},
		ModuleMap: map[tokens.ModuleName]tokens.ModuleName{"tests": "index"},
	})
	return integration.NewServer("test", semver.MustParse("1.0.0"), p)
}
