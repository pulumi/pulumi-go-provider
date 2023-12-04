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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/integration"
)

func urn(typ, name string) resource.URN {
	return resource.NewURN("stack", "proj", "",
		tokens.Type("test:index:"+typ), name)
}

// This type helps us test the highly suspicious behavior of naming an input the same as
// an output, while giving them different values. This should never be done in practice,
// but we need to accommodate the behavior while we allow it.
type Increment struct{}
type IncrementArgs struct {
	Number int `pulumi:"int"`
	Other  int `pulumi:"other,optional"`
}

type IncrementOutput struct{ IncrementArgs }

func (*Increment) Create(ctx p.Context, name string, inputs IncrementArgs, preview bool) (string, IncrementOutput, error) {
	output := IncrementOutput{IncrementArgs: IncrementArgs{Number: inputs.Number + 1}}
	return fmt.Sprintf("id-%d", inputs.Number), output, nil
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

var _ = (infer.ExplicitDependencies[WiredInputs, WiredOutputs])((*Wired)(nil))

// Wired plus is like wired, but has its inputs embedded with its outputs.
//
// This allows it to remember old inputs when calculating which fields have changed.
type WiredPlus struct{}
type WiredPlusOutputs struct {
	WiredInputs
	WiredOutputs
}

func (*WiredPlus) Create(ctx p.Context, name string, inputs WiredInputs, preview bool) (string, WiredPlusOutputs, error) {
	r := new(Wired)
	id, out, err := r.Create(ctx, name, inputs, preview)
	return id, WiredPlusOutputs{
		WiredInputs:  inputs,
		WiredOutputs: out,
	}, err
}

func (*WiredPlus) Update(
	ctx p.Context, id string, olds WiredPlusOutputs, news WiredInputs, preview bool,
) (WiredPlusOutputs, error) {
	r := new(Wired)
	out, err := r.Update(ctx, id, olds.WiredOutputs, news, preview)
	return WiredPlusOutputs{
		WiredInputs:  news,
		WiredOutputs: out,
	}, err
}

func (*WiredPlus) WireDependencies(f infer.FieldSelector, a *WiredInputs, s *WiredPlusOutputs) {
	r := new(Wired)
	r.WireDependencies(f, a, &s.WiredOutputs)
}

// Default values are applied by the provider to facilitate integration testing and to
// backstop non-compliment SDKs.

// TODO[pulumi-go-provider#98] Remove the ,optional.

type WithDefaults struct{}
type WithDefaultsOutput struct{ WithDefaultsArgs }

var (
	_ infer.Annotated = (*WithDefaultsArgs)(nil)
	_ infer.Annotated = (*NestedDefaults)(nil)
)

type WithDefaultsArgs struct {
	// We sanity check with some primitive values, but most of this checking is in
	// NestedDefaults.
	String       string                     `pulumi:"s,optional"`
	IntPtr       *int                       `pulumi:"pi,optional"`
	Nested       NestedDefaults             `pulumi:"nested,optional"`
	NestedPtr    *NestedDefaults            `pulumi:"nestedPtr"`
	OptWithReq   *OptWithReq                `pulumi:"optWithReq,optional"`
	ArrNested    []NestedDefaults           `pulumi:"arrNested,optional"`
	ArrNestedPtr []*NestedDefaults          `pulumi:"arrNestedPtr,optional"`
	MapNested    map[string]NestedDefaults  `pulumi:"mapNested,optional"`
	MapNestedPtr map[string]*NestedDefaults `pulumi:"mapNestedPtr,optional"`

	NoDefaultsPtr *NoDefaults `pulumi:"noDefaults,optional"`
}

type OptWithReq struct {
	Required *string `pulumi:"req"`
	Optional *string `pulumi:"opt,optional"`
	Empty    *string `pulumi:"empty,optional"`
}

func (o *OptWithReq) Annotate(a infer.Annotator) {
	a.SetDefault(&o.Optional, "default-value")
}

// We want to make sure we don't effect structs or maps that don't have default values.
type NoDefaults struct {
	String string `pulumi:"s,optional"`
}

func (w *WithDefaultsArgs) Annotate(a infer.Annotator) {
	a.SetDefault(&w.String, "one")
	a.SetDefault(&w.IntPtr, 2)
}

type NestedDefaults struct {
	// Direct vars. These don't allow setting zero values.
	String string  `pulumi:"s,optional"`
	Float  float64 `pulumi:"f,optional"`
	Int    int     `pulumi:"i,optional"`
	Bool   bool    `pulumi:"b,optional"`

	// Indirect vars. These should allow setting zero values.
	StringPtr *string  `pulumi:"ps,optional"`
	FloatPtr  *float64 `pulumi:"pf,optional"`
	IntPtr    *int     `pulumi:"pi,optional"`
	BoolPtr   *bool    `pulumi:"pb,optional"`

	// A triple indirect value, included to check that we can handle arbitrary
	// indirection.
	IntPtrPtrPtr ***int `pulumi:"pppi,optional"`
}

func (w *NestedDefaults) Annotate(a infer.Annotator) {
	a.SetDefault(&w.String, "two")
	a.SetDefault(&w.Float, 4.0)
	a.SetDefault(&w.Int, 8)
	// It doesn't make much sense to have default values of bools, but we support it.
	a.SetDefault(&w.Bool, true)

	// Now indirect ptrs
	a.SetDefault(&w.StringPtr, "two")
	a.SetDefault(&w.FloatPtr, 4.0)
	a.SetDefault(&w.IntPtr, 8)
	a.SetDefault(&w.BoolPtr, true)

	a.SetDefault(&w.IntPtrPtrPtr, 64)
}

func (w *WithDefaults) Create(
	ctx p.Context, name string, inputs WithDefaultsArgs, preview bool,
) (string, WithDefaultsOutput, error) {
	return "validated", WithDefaultsOutput{inputs}, nil
}

// Test reading environmental variables as default values.
type ReadEnv struct{}
type ReadEnvArgs struct {
	String  string  `pulumi:"s,optional"`
	Int     int     `pulumi:"i,optional"`
	Float64 float64 `pulumi:"f64,optional"`
	Bool    bool    `pulumi:"b,optional"`
}
type ReadEnvOutput struct{ ReadEnvArgs }

func (w *ReadEnvArgs) Annotate(a infer.Annotator) {
	a.SetDefault(&w.String, nil, "STRING")
	a.SetDefault(&w.Int, nil, "INT")
	a.SetDefault(&w.Float64, nil, "FLOAT64")
	a.SetDefault(&w.Bool, nil, "BOOL")
}

func (w *ReadEnv) Create(
	ctx p.Context, name string, inputs ReadEnvArgs, preview bool,
) (string, ReadEnvOutput, error) {
	return "well-read", ReadEnvOutput{inputs}, nil
}

type Recursive struct{}
type RecursiveArgs struct {
	Value string         `pulumi:"value,optional"`
	Other *RecursiveArgs `pulumi:"other,optional"`
}
type RecursiveOutput struct{ RecursiveArgs }

func (w *Recursive) Create(
	ctx p.Context, name string, inputs RecursiveArgs, preview bool,
) (string, RecursiveOutput, error) {
	return "did-not-overflow-stack", RecursiveOutput{inputs}, nil
}

func (w *RecursiveArgs) Annotate(a infer.Annotator) {
	a.SetDefault(&w.Value, "default-value")
}

type Config struct {
	Value string `pulumi:"value,optional"`
}

type ReadConfig struct{}
type ReadConfigArgs struct{}
type ReadConfigOutput struct {
	Config string `pulumi:"config"`
}

func (w *ReadConfig) Create(
	ctx p.Context, name string, _ ReadConfigArgs, _ bool,
) (string, ReadConfigOutput, error) {
	c := infer.GetConfig[Config](ctx)
	bytes, err := json.Marshal(c)
	return "read", ReadConfigOutput{Config: string(bytes)}, err
}

type GetJoin struct{}
type JoinArgs struct {
	Elems []string `pulumi:"elems"`
	Sep   *string  `pulumi:"sep,optional"`
}

func (j *JoinArgs) Annotate(a infer.Annotator) {
	a.SetDefault(&j.Sep, ",")
}

type JoinResult struct {
	Result string `pulumi:"result"`
}

func (*GetJoin) Call(ctx p.Context, args JoinArgs) (JoinResult, error) {
	return JoinResult{strings.Join(args.Elems, *args.Sep)}, nil
}

type ConfigCustom struct {
	Number  *float64 `pulumi:"number,optional"`
	Squared float64
}

func (c *ConfigCustom) Configure(ctx p.Context) error {
	if c.Number == nil {
		return nil
	}
	// We can perform arbitrary data transformations in the Configure step.  These
	// transformations aren't visible in Pulumi State, but are viable in other methods
	// on the provider.
	square := func(n float64) float64 { return n * n }
	c.Squared = square(*c.Number)
	return nil
}

var _ = (infer.CustomCheck[*ConfigCustom])((*ConfigCustom)(nil))

func (*ConfigCustom) Check(ctx p.Context,
	name string, oldInputs resource.PropertyMap, newInputs resource.PropertyMap,
) (*ConfigCustom, []p.CheckFailure, error) {
	var c ConfigCustom
	if v, ok := newInputs["number"]; ok {
		number := v.NumberValue() + 0.5
		c.Number = &number
	}

	return &c, nil, nil
}

type ReadConfigCustom struct{}
type ReadConfigCustomArgs struct{}
type ReadConfigCustomOutput struct {
	Config string `pulumi:"config"`
}

func (w *ReadConfigCustom) Create(
	ctx p.Context, name string, _ ReadConfigCustomArgs, _ bool,
) (string, ReadConfigCustomOutput, error) {
	c := infer.GetConfig[ConfigCustom](ctx)
	bytes, err := json.Marshal(c)
	return "read", ReadConfigCustomOutput{Config: string(bytes)}, err
}

func providerOpts(config infer.InferredConfig) infer.Options {
	return infer.Options{
		Config: config,
		Resources: []infer.InferredResource{
			infer.Resource[*Echo, EchoInputs, EchoOutputs](),
			infer.Resource[*Wired, WiredInputs, WiredOutputs](),
			infer.Resource[*WiredPlus, WiredInputs, WiredPlusOutputs](),
			infer.Resource[*Increment, IncrementArgs, IncrementOutput](),
			infer.Resource[*WithDefaults, WithDefaultsArgs, WithDefaultsOutput](),
			infer.Resource[*ReadEnv, ReadEnvArgs, ReadEnvOutput](),
			infer.Resource[*Recursive, RecursiveArgs, RecursiveOutput](),
			infer.Resource[*ReadConfig, ReadConfigArgs, ReadConfigOutput](),
			infer.Resource[*ReadConfigCustom, ReadConfigCustomArgs, ReadConfigCustomOutput](),
		},
		Functions: []infer.InferredFunction{
			infer.Function[*GetJoin, JoinArgs, JoinResult](),
		},
		ModuleMap: map[tokens.ModuleName]tokens.ModuleName{"tests": "index"},
	}
}

func provider() integration.Server {
	p := infer.Provider(providerOpts(nil))
	return integration.NewServer("test", semver.MustParse("1.0.0"), p)
}

func providerWithConfig[T any]() integration.Server {
	p := infer.Provider(providerOpts(infer.Config[T]()))
	return integration.NewServer("test", semver.MustParse("1.0.0"), p)
}
