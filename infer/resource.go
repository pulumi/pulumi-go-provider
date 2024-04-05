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

package infer

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/hashicorp/go-multierror"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/mapper"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer/internal/ende"
	"github.com/pulumi/pulumi-go-provider/internal"
	"github.com/pulumi/pulumi-go-provider/internal/introspect"
	t "github.com/pulumi/pulumi-go-provider/middleware"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
)

// A resource that understands how to create itself. This is the minimum requirement for
// defining a new custom resource.
//
// This interface should be implemented by the resource controller, with `I` the resource
// inputs and `O` the full set of resource fields. It is recommended that `O` is a
// superset of `I`, but it is not strictly required. The fields of `I` and `O` should
// consist of non-pulumi types i.e. `string` and `int` instead of `pulumi.StringInput` and
// `pulumi.IntOutput`.
//
// The behavior of a CustomResource resource can be extended by implementing any of the
// following traits:
// - CustomCheck
// - CustomDiff
// - CustomUpdate
// - CustomRead
// - CustomDelete
//
// Example:
// TODO
type CustomResource[I any, O any] interface {
	Create(ctx p.Context, name string, input I, preview bool) (id string, output O, err error)
}

// A resource that understands how to check its inputs.
//
// By default, infer handles checks by ensuring that a inputs de-serialize correctly. This
// is where you can extend that behavior. The returned input is given to subsequent calls
// to `Create` and `Update`.
//
// Example:
// TODO - Maybe a resource that has a regex. We could reject invalid regex before the up
// actually happens.
type CustomCheck[I any] interface {
	// Maybe oldInputs can be of type I
	Check(ctx p.Context, name string, oldInputs resource.PropertyMap, newInputs resource.PropertyMap) (
		I, []p.CheckFailure, error)
}

// A resource that understands how to diff itself given a new set of inputs.
//
// By default, infer handles diffs by structural equality among inputs. If CustomUpdate is
// implemented, changes will result in updates. Otherwise changes will result in replaces.
//
// Example:
// TODO - Indicate replacements for certain changes but not others.
type CustomDiff[I, O any] interface {
	// Maybe oldInputs can be of type I
	Diff(ctx p.Context, id string, olds O, news I) (p.DiffResponse, error)
}

// A resource that can adapt to new inputs with a delete and replace.
//
// There is no default behavior for CustomUpdate.
//
// Here the old state (as returned by Create or Update) as well as the new inputs are
// passed. Update should return the new state of the resource. If preview is true, then
// the update is part of `pulumi preview` and no changes should be made.
//
// Example:
// TODO
type CustomUpdate[I, O any] interface {
	Update(ctx p.Context, id string, olds O, news I, preview bool) (O, error)
}

// A resource that can recover its state from the provider.
//
// If CustomRead is not implemented, it will default to checking that the inputs and state
// fit into I and O respectively. If they do, then the values will be returned as is.
// Otherwise an error will be returned.
//
// Example:
// TODO - Probably something to do with the file system.
type CustomRead[I, O any] interface {
	// Read accepts a resource id, and a best guess of the input and output state. It returns
	// a normalized version of each, assuming it can be recovered.
	Read(ctx p.Context, id string, inputs I, state O) (
		canonicalID string, normalizedInputs I, normalizedState O, err error)
}

// A resource that knows how to delete itself.
//
// If a resource does not implement Delete, no code will be run on resource deletion.
type CustomDelete[O any] interface {
	// Delete is called before a resource is removed from pulumi state.
	Delete(ctx p.Context, id string, props O) error
}

// The methods of Annotator must be called on pointers to fields of their receivers, or on
// their receiver itself.
//
//	func (*s Struct) Annotated(a Annotator) {
//		a.Describe(&s, "A struct")            // Legal
//		a.Describe(&s.field1, "A field")      // Legal
//		a.Describe(s.field2, "A field")       // Not legal, since the pointer is missing.
//		otherS := &Struct{}
//		a.Describe(&otherS.field1, "A field") // Not legal, since describe is not called on its receiver.
//	}
type Annotator interface {
	// Annotate a struct field with a text description.
	Describe(i any, description string)

	// Annotate a struct field with a default value. The default value must be a primitive
	// type in the pulumi type system.
	SetDefault(i any, defaultValue any, env ...string)

	// Set the token of the annotated type.
	//
	// module and name should be valid Pulumi token segments. The package name will be
	// inferred from the provider.
	//
	// For example:
	//
	//	a.SetToken("mymodule", "MyResource")
	//
	// On a provider created with the name "mypkg" will have the token:
	//
	//	mypkg:mymodule:MyResource
	//
	SetToken(module, name string)
}

// Annotated is used to describe the fields of an object or a resource. Annotated can be
// implemented by `CustomResource`s, the input and output types for all resources and
// invokes, as well as other structs used the above.
type Annotated interface {
	Annotate(Annotator)
}

// An interface to help wire fields together.
type FieldSelector interface {
	// Create an input field. The argument to InputField must be a pointer to a field of
	// the associated input type I.
	//
	// For example:
	// ```go
	// func (r *MyResource) WireDependencies(f infer.FieldSelector, args *MyArgs, state *MyState) {
	//   f.InputField(&args.Field)
	// }
	// ```
	InputField(any) InputField
	// Create an output field. The argument to OutputField must be a pointer to a field of
	// the associated output type O.
	//
	// For example:
	// ```go
	// func (r *MyResource) WireDependencies(f infer.FieldSelector, args *MyArgs, state *MyState) {
	//   f.OutputField(&state.Field)
	// }
	// ```
	OutputField(any) OutputField
	// Seal the interface.
	isFieldSelector()
}

func (*fieldGenerator) isFieldSelector() {}

// A custom resource with the dataflow between its arguments (`I`) and outputs (`O`)
// specified. If a CustomResource implements ExplicitDependencies then WireDependencies
// will be called for each Create and Update call with `args` and `state` holding the
// values they will have for that call.
//
// If ExplicitDependencies is not implemented, it is assumed that all outputs depend on
// all inputs.
type ExplicitDependencies[I, O any] interface {
	// WireDependencies specifies the dependencies between inputs and outputs.
	WireDependencies(f FieldSelector, args *I, state *O)
}

// A field of the output (state).
type OutputField interface {
	// Specify that a state (output) field is always secret, regardless of its dependencies.
	AlwaysSecret()
	// Specify that a state (output) field is never secret, regardless of its dependencies.
	NeverSecret()
	// Specify that a state (output) field is always known, regardless of dependencies
	// or preview.
	AlwaysKnown()
	// Specify that a state (output) Field uses data from some args (input) Fields.
	DependsOn(dependencies ...InputField)

	// Seal the interface.
	isOutputField()
}

// A field of the input (args).
type InputField interface {
	// Seal the interface.
	isInputField()

	// Only wire secretness, not computedness.
	Secret() InputField

	// Only flow computedness, not secretness.
	Computed() InputField
}

type fieldGenerator struct {
	args, state  any
	argsMatcher  introspect.FieldMatcher
	stateMatcher introspect.FieldMatcher
	err          multierror.Error

	fields map[string]*field
}

func (g *fieldGenerator) getField(name string) *field {
	f, ok := g.fields[name]
	if !ok {
		f = new(field)
		g.fields[name] = f
	}
	return f
}

type field struct {
	// The set of tags that should always be secret
	alwaysSecret bool
	// The set of tags that should never be secret
	neverSecret bool
	// A map from a field to its dependencies.
	deps []dependency

	// If the output is known, regardless of other factors.
	known bool
}

type dependency struct {
	name string
	kind inputKind
}

// Check if we should apply this dependency to a kind.
func (d dependency) has(kind inputKind) bool {
	return d.kind == inputAll || kind == d.kind
}

// MarkMap mutates m to comply with the result of the fieldGenerator, applying
// computedness and secretness as appropriate.
func (g *fieldGenerator) MarkMap(isCreate, isPreview bool) func(oldInputs, inputs, m resource.PropertyMap) {
	return func(oldInputs, inputs, m resource.PropertyMap) {
		// Flow secretness and computedness
		for k, v := range m {
			m[k] = markField(g.getField(string(k)), k, v, oldInputs, inputs, isCreate, isPreview)
		}
	}
}

func markComputed(
	field *field, key resource.PropertyKey, prop resource.PropertyValue,
	oldInputs, inputs resource.PropertyMap, isCreate bool,
) resource.PropertyValue {
	// If the value is already computed or if it is guaranteed to be known, we don't need to do anything
	if field.known || ende.IsComputed(prop) {
		return prop
	}

	if input, ok := inputs[key]; ok && !ende.IsComputed(prop) && ende.DeepEquals(input, prop) {
		// prop is an output during a create, but the output mirrors an
		// input in name and value. We don't make it computed.
		return prop
	}

	// If this is during a create and the value is not explicitly marked as known, we mark it computed.
	if isCreate {
		return ende.MakeComputed(prop)
	}

	// If a dependency is computed or has changed, we mark this field as computed.
	for _, k := range field.deps {
		if !k.has(inputComputed) {
			continue
		}
		k := resource.PropertyKey(k.name)

		// Not all resources embed their inputs as outputs. When they don't we are
		// unable to perform old-vs-new diffing here.
		//
		// This may lead to applies running on old information during
		// preview. This is possible anyway, if user's dependencies don't
		// accurately reflect their logic. This is not a problem for non-preview
		// updates.
		//
		// The solution is to require embedding input structs in output structs
		// (or do it for the user), ensuring that we have access to information
		// that changed..
		oldInput, hasOldInput := oldInputs[k]
		if ende.IsComputed(inputs[k]) || (hasOldInput && !ende.DeepEquals(inputs[k], oldInput)) {
			return ende.MakeComputed(prop)
		}
	}

	return prop
}

func markSecret(
	field *field, key resource.PropertyKey, prop resource.PropertyValue, inputs resource.PropertyMap,
) resource.PropertyValue {
	// If we should never return a secret, ensure that the field *is not* marked as
	// secret, then return.
	if field.neverSecret {
		return ende.MakePublic(prop)
	}

	if ende.IsSecret(prop) {
		return prop
	}

	// If we should always return a secret, ensure that the field *is* marked as secret,
	// then return.
	if field.alwaysSecret {
		return ende.MakeSecret(prop)
	}

	if input, ok := inputs[key]; ok && ende.DeepEquals(input, prop) {
		// prop might depend on a secret value, but the output mirrors a input in
		// name and value. We don't make it secret since it will either be public
		// in the state as an input, or is already a secret.
		return prop
	}

	// Otherwise secretness is derived from dependencies: any dependency that is
	// secret makes the field secret.
	for _, k := range field.deps {
		if !k.has(inputSecret) {
			continue
		}
		if inputs[resource.PropertyKey(k.name)].ContainsSecrets() {
			return ende.MakeSecret(prop)
		}
	}

	return prop
}

func markField(
	field *field, key resource.PropertyKey, prop resource.PropertyValue,
	oldInputs, inputs resource.PropertyMap, isCreate, isPreview bool,
) resource.PropertyValue {
	// Fields can only be computed during preview. They must be known by when the resource is actually created.
	if isPreview {
		prop = markComputed(field, key, prop, oldInputs, inputs, isCreate)
	}

	return markSecret(field, key, prop, inputs)

}

func (g *fieldGenerator) InputField(a any) InputField {
	if allFields, ok, err := g.argsMatcher.TargetStructFields(a); ok {
		if err != nil {
			g.err.Errors = append(g.err.Errors, err)
			return &errField{}
		}
		return &inputField{fields: allFields}
	}
	field, ok, err := g.argsMatcher.GetField(a)
	if err != nil {
		g.err.Errors = append(g.err.Errors, err)
		return &errField{}
	}
	if ok {
		return &inputField{fields: []introspect.FieldTag{field}}
	}
	// Couldn't find the field on the args, try the state
	field, ok, err = g.stateMatcher.GetField(a)
	if err == nil && ok {
		g.err.Errors = append(g.err.Errors,
			fmt.Errorf("internal error: %v (%v) is an output field, not an input field", a, field.Name))
	}

	g.err.Errors = append(g.err.Errors, fmt.Errorf("internal error: could not find the input field for value %v", a))
	return &errField{}
}

func (g *fieldGenerator) OutputField(a any) OutputField {
	if allFields, ok, err := g.stateMatcher.TargetStructFields(a); ok {
		if err != nil {
			g.err.Errors = append(g.err.Errors, err)
			return &errField{}
		}
		return &outputField{g, allFields}
	}
	field, ok, err := g.stateMatcher.GetField(a)
	if err != nil {
		g.err.Errors = append(g.err.Errors, err)
		return &errField{}
	}
	if ok {
		return &outputField{g, []introspect.FieldTag{field}}
	}
	// Couldn't find the field on the state, try the args
	field, ok, err = g.argsMatcher.GetField(a)
	if err == nil && ok {
		g.err.Errors = append(g.err.Errors, fmt.Errorf("%v (%v) is an input field, not an output field", a, field.Name))
	}

	g.err.Errors = append(g.err.Errors, fmt.Errorf("could not find the output field for value %v", a))
	return &errField{}
}

func newFieldGenerator(i, o any) *fieldGenerator {
	return &fieldGenerator{
		args: i, state: o,
		argsMatcher:  introspect.NewFieldMatcher(i),
		stateMatcher: introspect.NewFieldMatcher(o),
		err: multierror.Error{
			ErrorFormat: func(es []error) string {
				return "wiring error: " + multierror.ListFormatFunc(es)
			},
		},

		fields: map[string]*field{},
	}
}

// userSetKind is true if the user has set a dependency of a matching kind.
func (g *fieldGenerator) userSetKind(kind inputKind) bool {
	for _, f := range g.fields {
		for _, dep := range f.deps {
			if dep.has(kind) {
				return true
			}
		}
	}
	return false
}

// ensureDefaultComputed ensures that some computedness flow exists on the provider.
//
// If the user has not specified any flow, then we apply the default flow:
//
// Since we can't see inside the user's code to view data flow, we default to
// assuming that all inputs will be used to effect all outputs.
//
// Consider this example:
//
//	Input:  { a, b, c }
//	Output: { a, b, d }
//
// We would see this computedness flow:
//
//	Output | Input
//	-------+------
//	     a | a b c
//	     b | a b c
//	     d | a b c
func (g *fieldGenerator) ensureDefaultComputed() {
	if g.userSetKind(inputComputed) {
		// The user has specified something, so we respect that.
		return
	}
	// The user has not set a flow, so apply our own:
	//
	// Set every output to depend on each input (for computed only)
	g.OutputField(g.state).DependsOn(g.InputField(g.args).Computed())
}

// ensureDefaultSecrets that some secretness flow is explicit.
//
// If the user has not specified any flow, then we apply the default flow:
//
// Outputs that share a name with inputs have the secretness flow from input to
// output.
//
// Consider this example:
//
//	Input:  { a, b, c }
//	Output: { a, b, d }
//
// We would see this secretness flow:
//
//	Output | Input
//	-------+------
//	     a | a
//	     b | b
func (g *fieldGenerator) ensureDefaultSecrets() {
	if g.userSetKind(inputSecret) {
		// The user has specified something, so we respect that.
		return
	}

	// The user has not set a flow, so apply our own

	args, ok, err := g.argsMatcher.TargetStructFields(g.args)
	contract.Assertf(ok, "we match by construction")
	contract.AssertNoError(err)

	state, ok, err := g.stateMatcher.TargetStructFields(g.state)
	contract.Assertf(ok, "we match by construction")
	contract.AssertNoError(err)

	for _, f := range state {
		if f.Internal {
			continue
		}
		for _, a := range args {
			if f.Name != a.Name {
				continue
			}

			v := g.getField(a.Name)
			v.deps = append(v.deps, dependency{
				name: a.Name,
				kind: inputSecret,
			})
			// There will only be one field with the name f.Name, we so may
			// safely break.
			break
		}
	}
}

// The return value when an error happens. The error is reported when the errField is
// created, so this type only exists so we can return a valid instance of
// InputField/OutputField. The functions on errField do nothing.
type errField struct{}

func (*errField) AlwaysSecret()           {}
func (*errField) AlwaysKnown()            {}
func (*errField) NeverSecret()            {}
func (*errField) DependsOn(...InputField) {}
func (*errField) isInputField()           {}
func (*errField) isOutputField()          {}
func (*errField) Computed() InputField    { return &errField{} }
func (*errField) Secret() InputField      { return &errField{} }

type inputField struct {
	kind   inputKind
	fields []introspect.FieldTag
}

type inputKind int

const (
	inputAll      = 0
	inputSecret   = iota
	inputComputed = iota
)

func (i *inputField) Computed() InputField {
	input := new(inputField)
	input.kind = inputComputed
	// Copy input fields
	input.fields = make([]introspect.FieldTag, len(i.fields))
	for i, f := range i.fields {
		input.fields[i] = f
	}
	return input
}

func (i *inputField) Secret() InputField {
	input := new(inputField)
	input.kind = inputSecret
	// Copy input fields
	input.fields = make([]introspect.FieldTag, len(i.fields))
	for i, f := range i.fields {
		input.fields[i] = f
	}
	return input
}

func (*inputField) isInputField() {}

type outputField struct {
	g      *fieldGenerator
	fields []introspect.FieldTag
}

func (f *outputField) set(set func(string, *field)) {
	for _, field := range f.fields {
		name := field.Name
		set(name, f.g.getField(name))
	}
}

func (f *outputField) AlwaysSecret() {
	f.set(func(name string, field *field) {
		field.alwaysSecret = true
		if field.neverSecret {
			f.g.err.Errors = append(f.g.err.Errors,
				fmt.Errorf("marked field %q as both always secret and never secret", name))
		}
	})
}

func (f *outputField) AlwaysKnown() { f.set(func(_ string, field *field) { field.known = true }) }

func (f *outputField) NeverSecret() {
	f.set(func(name string, field *field) {
		field.neverSecret = true
		if field.alwaysSecret {
			f.g.err.Errors = append(f.g.err.Errors,
				fmt.Errorf("marked field %q as both always secret and never secret", name))
		}
	})
}

func typeInput(i InputField) (*inputField, bool) {
	switch i := i.(type) {
	case *inputField:
		return i, true
	case *errField:
		return nil, false
	default:
		panic(fmt.Sprintf("Unknown InputField type: %T", i))
	}
}

func (f *outputField) DependsOn(deps ...InputField) {
	depNames := make([]dependency, 0, len(deps))
	for _, d := range deps {
		d, ok := typeInput(d)
		if !ok {
			// The error was already reported, so do nothing
			continue
		}
		for _, field := range d.fields {
			depNames = append(depNames, dependency{
				name: field.Name,
				kind: d.kind,
			})
		}
	}

	f.set(func(_ string, f *field) { f.deps = append(f.deps, depNames...) })
}

func (*outputField) isOutputField() {}

// A resource inferred by the Resource function.
//
// This interface cannot be implemented directly. Instead consult the Resource function.
type InferredResource interface {
	t.CustomResource
	schema.Resource

	isInferredResource()
}

// Create a new InferredResource, where `R` is the resource controller, `I` is the
// resources inputs and `O` is the resources outputs.
func Resource[R CustomResource[I, O], I, O any]() InferredResource {
	return &derivedResourceController[R, I, O]{}
}

type derivedResourceController[R CustomResource[I, O], I, O any] struct{}

func (*derivedResourceController[R, I, O]) isInferredResource() {}

func (*derivedResourceController[R, I, O]) GetSchema(reg schema.RegisterDerivativeType) (
	pschema.ResourceSpec, error) {
	if err := registerTypes[I](reg); err != nil {
		return pschema.ResourceSpec{}, err
	}
	if err := registerTypes[O](reg); err != nil {
		return pschema.ResourceSpec{}, err
	}
	r, errs := getResourceSchema[R, I, O](false)
	return r, errs.ErrorOrNil()
}

func getToken[R any](transform func(tokens.Type) tokens.Type) (tokens.Type, error) {
	var r R
	return getTokenOf(reflect.TypeOf(r), transform)
}

func getTokenOf(t reflect.Type, transform func(tokens.Type) tokens.Type) (tokens.Type, error) {
	annotator := getAnnotated(t)
	if annotator.Token != "" {
		return tokens.Type(annotator.Token), nil
	}

	tk, err := introspect.GetToken("pkg", t)
	if transform == nil || err != nil {
		return tk, err
	}

	return transform(tk), nil
}

func (*derivedResourceController[R, I, O]) GetToken() (tokens.Type, error) {
	return getToken[R](nil)
}

func (*derivedResourceController[R, I, O]) getInstance() *R {
	var r R
	return &r
}

func (rc *derivedResourceController[R, I, O]) Check(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
	var r R
	var i I
	encoder, err := ende.Decode(req.News, &i)
	if r, ok := ((interface{})(r)).(CustomCheck[I]); ok {
		// The user implemented check manually, so call that
		i, failures, err := r.Check(ctx, req.Urn.Name(), req.Olds, req.News)
		if err != nil {
			return p.CheckResponse{}, err
		}
		inputs, err := encoder.Encode(i)
		if err != nil {
			return p.CheckResponse{}, err
		}
		return p.CheckResponse{
			Inputs:   inputs,
			Failures: failures,
		}, nil
	}
	if err == nil {
		if err := applyDefaults(&i); err != nil {
			return p.CheckResponse{}, fmt.Errorf("unable to apply defaults: %w", err)
		}

		inputs, err := encoder.Encode(i)

		return p.CheckResponse{
			Inputs: inputs,
		}, err
	}

	failures, e := checkFailureFromMapError(err)
	if e != nil {
		return p.CheckResponse{}, e
	}

	return p.CheckResponse{
		Inputs:   req.News,
		Failures: failures,
	}, nil
}

// Ensure that `inputs` can deserialize cleanly into `I`.
func DefaultCheck[I any](inputs resource.PropertyMap) (I, []p.CheckFailure, error) {
	var i I
	_, err := ende.Decode(inputs, &i)
	if err == nil {
		return i, nil, nil
	}

	failures, e := checkFailureFromMapError(err)
	return i, failures, e
}

func checkFailureFromMapError(err mapper.MappingError) ([]p.CheckFailure, error) {
	if err == nil {
		return nil, nil
	}
	failures := []p.CheckFailure{}
	for _, err := range err.Failures() {
		switch err := err.(type) {
		case mapper.FieldError:
			failures = append(failures, p.CheckFailure{
				Property: err.Field(),
				Reason:   err.Reason(),
			})
		default:
			return failures, fmt.Errorf("unknown mapper error: %w", err)
		}
	}
	return failures, nil
}

func (rc *derivedResourceController[R, I, O]) Diff(ctx p.Context, req p.DiffRequest) (p.DiffResponse, error) {
	r := rc.getInstance()
	_, hasUpdate := ((interface{})(*r)).(CustomUpdate[I, O])
	var forceReplace func(string) bool
	if hasUpdate {
		schema, err := rc.GetSchema(func(tokens.Type, pschema.ComplexTypeSpec) bool { return false })
		if err != nil {
			return p.DiffResponse{}, err
		}
		forceReplace = func(s string) bool {
			if schema.InputProperties == nil {
				return false
			}
			return schema.InputProperties[s].ReplaceOnChanges
		}
	} else {
		// No update => every change is a replace
		forceReplace = func(string) bool { return true }
	}
	return diff[R, I, O](ctx, req, r, forceReplace)
}

// Compute a diff request.
func diff[R, I, O any](ctx p.Context, req p.DiffRequest, r *R, forceReplace func(string) bool) (p.DiffResponse, error) {

	for _, ignoredChange := range req.IgnoreChanges {
		req.News[ignoredChange] = req.Olds[ignoredChange]
	}

	if r, ok := ((interface{})(*r)).(CustomDiff[I, O]); ok {
		var olds O
		var news I
		var err error
		_, err = ende.Decode(req.Olds, &olds)
		if err != nil {
			return p.DiffResponse{}, err
		}
		_, err = ende.Decode(req.News, &news)
		if err != nil {
			return p.DiffResponse{}, err
		}
		diff, err := r.Diff(ctx, req.ID, olds, news)
		if err != nil {
			return p.DiffResponse{}, err
		}
		return diff, nil
	}

	inputProps, err := introspect.FindProperties(new(I))
	if err != nil {
		return p.DiffResponse{}, err
	}
	// Olds is an Output, but news is an Input. Output should be a superset of Input,
	// so we need to filter out fields that are in Output but not Input.
	oldInputs := resource.PropertyMap{}
	for k := range inputProps {
		key := resource.PropertyKey(k)
		oldInputs[key] = req.Olds[key]
	}
	objDiff := oldInputs.Diff(req.News)
	pluginDiff := plugin.NewDetailedDiffFromObjectDiff(objDiff, false)
	diff := map[string]p.PropertyDiff{}

	for k, v := range pluginDiff {
		set := func(kind p.DiffKind) {
			diff[k] = p.PropertyDiff{
				Kind:      kind,
				InputDiff: v.InputDiff,
			}
		}
		if forceReplace(k) {
			// We force replaces if we don't have access to updates
			v.Kind = v.Kind.AsReplace()
		}
		switch v.Kind {
		case plugin.DiffAdd:
			set(p.Add)
		case plugin.DiffAddReplace:
			set(p.AddReplace)
		case plugin.DiffDelete:
			set(p.Delete)
		case plugin.DiffDeleteReplace:
			set(p.DeleteReplace)
		case plugin.DiffUpdate:
			set(p.Update)
		case plugin.DiffUpdateReplace:
			set(p.UpdateReplace)
		}
	}
	return p.DiffResponse{
		// TODO: how shoould we set this?
		// DeleteBeforeReplace: ???,
		HasChanges:   objDiff.AnyChanges(),
		DetailedDiff: diff,
	}, nil
}

func (rc *derivedResourceController[R, I, O]) Create(
	ctx p.Context, req p.CreateRequest,
) (resp p.CreateResponse, retError error) {
	r := rc.getInstance()

	var input I
	var err error
	encoder, err := ende.Decode(req.Properties, &input)
	if err != nil {
		return p.CreateResponse{}, fmt.Errorf("invalid inputs: %w", err)
	}

	id, o, err := (*r).Create(ctx, req.Urn.Name(), input, req.Preview)
	if initFailed := (ResourceInitFailedError{}); errors.As(err, &initFailed) {
		defer func(createErr error) {
			// If there was an error, it indicates a problem with serializing
			// the output.
			//
			// Failing to return full properties here will leak the created
			// resource so we should warn users.
			if retError != nil {
				retError = internal.Errorf("failed to return partial resource: %w;"+
					" %s may be leaked", retError, req.Urn)
			} else {
				// We don't want to loose information conveyed in the
				// error chain returned by the user.
				retError = createErr
			}

			resp.PartialState = &p.InitializationFailed{
				Reasons: initFailed.Reasons,
			}
		}(err)
	} else if err != nil {
		return p.CreateResponse{}, err
	}

	if id == "" && !req.Preview {
		return p.CreateResponse{}, ProviderErrorf("'%s' was created without an id", req.Urn)
	}

	m, err := encoder.AllowUnknown(req.Preview).Encode(o)
	if err != nil {
		return p.CreateResponse{}, fmt.Errorf("encoding resource properties: %w", err)
	}

	setDeps, err := getDependencies(r, &input, &o, true /* isCreate */, req.Preview)
	if err != nil {
		return p.CreateResponse{}, err
	}
	setDeps(nil, req.Properties, m)

	return p.CreateResponse{
		ID:         id,
		Properties: m,
	}, err
}

func (rc *derivedResourceController[R, I, O]) Read(
	ctx p.Context, req p.ReadRequest,
) (resp p.ReadResponse, retError error) {
	r := rc.getInstance()
	var inputs I
	var state O
	var err error
	inputEncoder, err := ende.DecodeTolerateMissing(req.Inputs, &inputs)
	if err != nil {
		return p.ReadResponse{}, err
	}
	stateEncoder, err := ende.DecodeTolerateMissing(req.Properties, &state)
	if err != nil {
		return p.ReadResponse{}, err
	}
	read, ok := ((interface{})(*r)).(CustomRead[I, O])
	if !ok {
		// Default read implementation:
		//
		// We have already confirmed that we deserialize state and properties correctly.
		// We now just return them as is.
		return p.ReadResponse{
			ID:         req.ID,
			Properties: req.Properties,
			Inputs:     req.Inputs,
		}, nil
	}
	id, inputs, state, err := read.Read(ctx, req.ID, inputs, state)
	if initFailed := (ResourceInitFailedError{}); errors.As(err, &initFailed) {
		defer func(readErr error) {
			// If there was an error, it indicates a problem with serializing
			// the output.
			//
			// Failing to return full properties here will leak the created
			// resource so we should warn users.
			if retError != nil {
				retError = internal.Errorf("failed to return partial resource: %w",
					retError)
			} else {
				// We don't want to loose information conveyed in the
				// error chain returned by the user.
				retError = readErr
			}

			resp.PartialState = &p.InitializationFailed{
				Reasons: initFailed.Reasons,
			}
		}(err)
	} else if err != nil {
		return p.ReadResponse{}, err
	}

	i, err := inputEncoder.Encode(inputs)
	if err != nil {
		return p.ReadResponse{}, err
	}
	s, err := stateEncoder.Encode(state)
	if err != nil {
		return p.ReadResponse{}, err
	}

	return p.ReadResponse{
		ID:         id,
		Properties: s,
		Inputs:     i,
	}, nil
}

func (rc *derivedResourceController[R, I, O]) Update(
	ctx p.Context, req p.UpdateRequest,
) (resp p.UpdateResponse, retError error) {
	r := rc.getInstance()
	update, ok := ((interface{})(*r)).(CustomUpdate[I, O])
	if !ok {
		return p.UpdateResponse{}, status.Errorf(codes.Unimplemented,
			"Update is not implemented for resource %s", req.Urn)
	}
	var news I
	var olds O
	var err error
	for _, ignoredChange := range req.IgnoreChanges {
		req.News[ignoredChange] = req.Olds[ignoredChange]
	}
	_, err = ende.Decode(req.Olds, &olds)
	if err != nil {
		return p.UpdateResponse{}, err
	}
	encoder, err := ende.Decode(req.News, &news)
	if err != nil {
		return p.UpdateResponse{}, err
	}
	o, err := update.Update(ctx, req.ID, olds, news, req.Preview)
	if initFailed := (ResourceInitFailedError{}); errors.As(err, &initFailed) {
		defer func(updateErr error) {
			// If there was an error, it indicates a problem with serializing
			// the output.
			//
			// Failing to return full properties here will leak the created
			// resource so we should warn users.
			if retError != nil {
				retError = internal.Errorf("failed to return partial resource: %w",
					retError)
			} else {
				// We don't want to loose information conveyed in the
				// error chain returned by the user.
				retError = updateErr
			}

			resp.PartialState = &p.InitializationFailed{
				Reasons: initFailed.Reasons,
			}
		}(err)
		err = nil
	}
	if err != nil {
		return p.UpdateResponse{}, err
	}
	m, err := encoder.AllowUnknown(req.Preview).Encode(o)
	if err != nil {
		return p.UpdateResponse{}, err
	}
	setDeps, err := getDependencies(r, &news, &o, false /* isCreate */, req.Preview)
	if err != nil {
		return p.UpdateResponse{}, err
	}
	setDeps(req.Olds, req.News, m)

	return p.UpdateResponse{
		Properties: m,
	}, nil
}

func (rc *derivedResourceController[R, I, O]) Delete(ctx p.Context, req p.DeleteRequest) error {
	r := rc.getInstance()
	del, ok := ((interface{})(*r)).(CustomDelete[O])
	if ok {
		var olds O
		_, err := ende.Decode(req.Properties, &olds)
		if err != nil {
			return err
		}
		return del.Delete(ctx, req.ID, olds)
	}
	return nil
}

// Apply dependencies to a property map, flowing secretness and computedness from input to
// output.
type setDeps func(oldInputs, input, output resource.PropertyMap)

// Get the decency mapping between inputs and outputs of a resource.
func getDependencies[R, I, O any](
	r *R, input *I, output *O, isCreate, isPreview bool,
) (setDeps, error) {
	var wire func(FieldSelector)

	if r, ok := ((interface{})(*r)).(ExplicitDependencies[I, O]); ok {
		wire = func(fg FieldSelector) {
			r.WireDependencies(fg, input, output)
		}
	}
	return getDependenciesRaw(input, output, wire, isCreate, isPreview)
}

// getDependenciesRaw is the untyped implementation of getDependencies.
func getDependenciesRaw(
	input, output any, wire func(FieldSelector), isCreate, isPreview bool,
) (setDeps, error) {
	fg := newFieldGenerator(input, output)
	if wire != nil {
		wire(fg)
		if err := fg.err.ErrorOrNil(); err != nil {
			return nil, err
		}

	}

	fg.ensureDefaultSecrets()
	fg.ensureDefaultComputed()

	// If the user code returned an error, we would have returned it by now. An
	// error here means that our code set an error.
	contract.AssertNoErrorf(fg.err.ErrorOrNil(), "Default dependency wiring failed")

	return fg.MarkMap(isCreate, isPreview), nil
}
