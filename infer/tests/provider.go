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
	"reflect"

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

type WithDefaults struct{}
type WithDefaultsOutput struct{ WithDefaultsArgs }

var (
	_ infer.Annotated = (*WithDefaultsArgs)(nil)
	_ infer.Annotated = (*NestedDefaults)(nil)
)

type WithDefaultsArgs struct {
	// We sanity check with some primitive values, but most of this checking is in
	// NestedDefaults.
	String       string                     `pulumi:"s"`
	Int          *int                       `pulumi:"i"`
	Nested       NestedDefaults             `pulumi:"nested"`
	NestedPtr    *NestedDefaults            `pulumi:"nestedPtr"`
	ArrNested    []NestedDefaults           `pulumi:"arrNested"`
	ArrNestedPtr []*NestedDefaults          `pulumi:"arrNestedPtr"`
	MapNested    map[string]NestedDefaults  `pulumi:"mapNested"`
	MapNestedPtr map[string]*NestedDefaults `pulumi:"mapNestedPtr"`
}

func (w *WithDefaultsArgs) Annotate(a infer.Annotator) {
	a.SetDefault(&w.String, "one")
	a.SetDefault(&w.Int, 2)
}

type NestedDefaults struct {
	// Direct vars. These don't allow setting zero values.
	String string  `pulumi:"s"`
	Float  float64 `pulumi:"f"`
	Int    int     `pulumi:"i"`
	Bool   bool    `pulumi:"b"`

	// Indirect vars. These should allow setting zero values.
	StringPtr *string  `pulumi:"ps"`
	FloatPtr  *float64 `pulumi:"pf"`
	IntPtr    *int     `pulumi:"pi"`
	BoolPtr   *bool    `pulumi:"pb"`

	// A triple indirect value, included to check that we can handle arbitrary
	// indirection.
	IntPtrPtrPtr ***int `pulumi:"pppi"`
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

func (w *WithDefaultsArgs) validate(check func(value any)) {
	check(w.String)
	check(w.Int)
	w.Nested.validate(check)
	w.NestedPtr.validate(check)

	for _, v := range w.ArrNested {
		v.validate(check)
	}
	for _, v := range w.ArrNestedPtr {
		v.validate(check)
	}
	for _, v := range w.MapNested {
		v.validate(check)
	}
	for _, v := range w.MapNestedPtr {
		v.validate(check)
	}
}

// Check that all values with default values are non-zero.
func (w *NestedDefaults) validate(check func(value any)) {
	// direct values
	check(w.String)
	check(w.Float)
	check(w.Int)
	check(w.Bool)

	// indirect values
	check(w.StringPtr)
	check(w.FloatPtr)
	check(w.IntPtr)
	check(w.BoolPtr)

	// triple-indirect values.
	check(w.IntPtrPtrPtr)
}

func (*WithDefaults) check(value any) {
	v := reflect.ValueOf(value)
	for v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.IsZero() {
		panic("Default value not applied")
	}
}

func (w *WithDefaults) Create(ctx p.Context, name string, inputs WithDefaultsArgs, preview bool) (string, WithDefaultsOutput, error) {
	inputs.validate(w.check)
	return "validated", WithDefaultsOutput{inputs}, nil
}

func (w *WithDefaults) Update(
	ctx p.Context, id string, olds WithDefaultsOutput, news WithDefaultsArgs, preview bool,
) (WithDefaultsOutput, error) {
	news.validate(w.check)
	return WithDefaultsOutput{news}, nil
}

func provider() integration.Server {
	p := infer.Provider(infer.Options{
		Resources: []infer.InferredResource{
			infer.Resource[*Echo, EchoInputs, EchoOutputs](),
			infer.Resource[*Wired, WiredInputs, WiredOutputs](),
			infer.Resource[*WiredPlus, WiredInputs, WiredPlusOutputs](),
			infer.Resource[*Increment, IncrementArgs, IncrementOutput](),
			infer.Resource[*WithDefaults, WithDefaultsArgs, WithDefaultsOutput](),
		},
		ModuleMap: map[tokens.ModuleName]tokens.ModuleName{"tests": "index"},
	})
	return integration.NewServer("test", semver.MustParse("1.0.0"), p)
}
