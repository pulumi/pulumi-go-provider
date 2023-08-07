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
}

type fieldGenerator struct {
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
	deps []string

	// If the output is known, regardless of other factors.
	known bool
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
	if field.known || prop.IsComputed() {
		return prop
	}

	// If this is during a create and the value is not explicitly marked as known, we mark it computed.
	if isCreate {
		if input, ok := inputs[key]; ok && !input.IsComputed() && input.DeepEquals(prop) {
			// prop is an output during a create, but the output mirrors an
			// input in name and value. We don't make it computed.
			return prop
		}
		return resource.MakeComputed(prop)
	}

	// If a dependency is computed or has changed, we mark this field as computed.
	for _, k := range field.deps {
		k := resource.PropertyKey(k)
		if inputs[k].IsComputed() || !inputs[k].DeepEquals(oldInputs[k]) {
			return resource.MakeComputed(prop)
		}
	}

	return prop
}

func markSecret(field *field, prop resource.PropertyValue, inputs resource.PropertyMap) resource.PropertyValue {
	// If we should never return a secret, ensure that the field *is not* marked as
	// secret, then return.
	if field.neverSecret {
		if prop.IsSecret() {
			prop = prop.SecretValue().Element
		}
		return prop
	}

	if prop.IsSecret() {
		return prop
	}

	// If we should always return a secret, ensure that the field *is* marked as secret,
	// then return.
	if field.alwaysSecret {
		return resource.MakeSecret(prop)
	}

	// Otherwise secretness is derived from dependencies: any dependency that is
	// secret makes the field secret.
	for _, k := range field.deps {
		if inputs[resource.PropertyKey(k)].IsSecret() {
			return resource.MakeSecret(prop)
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

	return markSecret(field, prop, inputs)

}

func (g *fieldGenerator) InputField(a any) InputField {
	if allFields, ok, err := g.argsMatcher.TargetStructFields(a); ok {
		if err != nil {
			g.err.Errors = append(g.err.Errors, err)
			return &errField{}
		}
		return &inputField{allFields}
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

func newFieldGenerator[I, O any](i *I, o *O) *fieldGenerator {
	return &fieldGenerator{
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

type inputField struct {
	fields []introspect.FieldTag
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

func (f *outputField) DependsOn(deps ...InputField) {
	depNames := make([]string, 0, len(deps))
	for _, d := range deps {
		switch d := d.(type) {
		case *inputField:
			for _, field := range d.fields {
				depNames = append(depNames, field.Name)
			}
		case *errField:
			// The error was already reported, so do nothing
		default:
			panic(fmt.Sprintf("Unknown InputField type: %T", d))
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

func (*derivedResourceController[R, I, O]) GetToken() (tokens.Type, error) {
	var r R
	return introspect.GetToken("pkg", r)
}

func (*derivedResourceController[R, I, O]) getInstance() *R {
	var r R
	return &r
}

func (rc *derivedResourceController[R, I, O]) Check(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
	var r R
	if r, ok := ((interface{})(r)).(CustomCheck[I]); ok {
		// The user implemented check manually, so call that
		i, failures, err := r.Check(ctx, req.Urn.Name().String(), req.Olds, req.News)
		if err != nil {
			return p.CheckResponse{}, err
		}
		inputs, err := encode(i, nil, false)
		if err != nil {
			return p.CheckResponse{}, err
		}
		return p.CheckResponse{
			Inputs:   inputs,
			Failures: failures,
		}, nil
	}
	// The user has not implemented check, so do the smart thing by default; We just check
	// that we can de-serialize correctly
	var i I
	_, err := decode(req.News, &i, req.News.ContainsUnknowns())
	if err == nil {
		return p.CheckResponse{
			Inputs: req.News,
		}, nil
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
	_, err := decode(inputs, &i, inputs.ContainsUnknowns())
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
		schema, err := rc.GetSchema(func(tk tokens.Type, typ pschema.ComplexTypeSpec) (unknown bool) { return false })
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
		_, err = decode(req.Olds, &olds, req.Olds.ContainsUnknowns())
		if err != nil {
			return p.DiffResponse{}, err
		}
		_, err = decode(req.News, &news, req.News.ContainsUnknowns())
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
	pluginDiff := plugin.NewDetailedDiffFromObjectDiff(objDiff)
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

func (rc *derivedResourceController[R, I, O]) Create(ctx p.Context, req p.CreateRequest) (p.CreateResponse, error) {
	r := rc.getInstance()

	var input I
	var err error
	secrets, err := decode(req.Properties, &input, req.Preview)
	if err != nil {
		return p.CreateResponse{}, fmt.Errorf("invalid inputs: %w", err)
	}

	id, o, err := (*r).Create(ctx, req.Urn.Name().String(), input, req.Preview)
	if err != nil {
		return p.CreateResponse{}, err
	}
	if id == "" && !req.Preview {
		return p.CreateResponse{}, fmt.Errorf("internal error: '%s' was created without an id", req.Urn)
	}

	m, err := encode(o, secrets, req.Preview)
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
	}, nil
}

func (rc *derivedResourceController[R, I, O]) Read(ctx p.Context, req p.ReadRequest) (p.ReadResponse, error) {
	r := rc.getInstance()
	var inputs I
	var state O
	var err error
	inputSecrets, err := decode(req.Inputs, &inputs, true)
	if err != nil {
		return p.ReadResponse{}, err
	}
	stateSecrets, err := decode(req.Properties, &state, true)
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
	if err != nil {
		return p.ReadResponse{}, err
	}
	i, err := encode(inputs, inputSecrets, false)
	if err != nil {
		return p.ReadResponse{}, err
	}
	s, err := encode(state, stateSecrets, false)
	if err != nil {
		return p.ReadResponse{}, err
	}

	return p.ReadResponse{
		ID:         id,
		Properties: s,
		Inputs:     i,
	}, nil
}

func (rc *derivedResourceController[R, I, O]) Update(ctx p.Context, req p.UpdateRequest) (p.UpdateResponse, error) {
	r := rc.getInstance()
	update, ok := ((interface{})(*r)).(CustomUpdate[I, O])
	if !ok {
		return p.UpdateResponse{}, status.Errorf(codes.Unimplemented, "Update is not implemented for resource %s", req.Urn)
	}
	var news I
	var olds O
	var err error
	for _, ignoredChange := range req.IgnoreChanges {
		req.News[ignoredChange] = req.Olds[ignoredChange]
	}
	_, err = decode(req.Olds, &olds, req.Preview)
	if err != nil {
		return p.UpdateResponse{}, err
	}
	secrets, err := decode(req.News, &news, req.Preview)
	if err != nil {
		return p.UpdateResponse{}, err
	}
	o, err := update.Update(ctx, req.ID, olds, news, req.Preview)
	if err != nil {
		return p.UpdateResponse{}, err
	}
	m, err := encode(o, secrets, req.Preview)
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
		_, err := decode(req.Properties, &olds, false)
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
func getDependencies[R, I, O any](r *R, input *I, output *O, isCreate, isPreview bool) (setDeps, error) {
	fg := newFieldGenerator(input, output)
	if r, ok := ((interface{})(*r)).(ExplicitDependencies[I, O]); ok {
		r.WireDependencies(fg, input, output)
		if err := fg.err.ErrorOrNil(); err != nil {
			return nil, err
		}
	} else {
		// We default to assuming that every output field depends on every input field.
		fg.OutputField(output).DependsOn(fg.InputField(input))
		contract.AssertNoErrorf(fg.err.ErrorOrNil(), "Default dependency wiring failed")
	}

	return fg.MarkMap(isCreate, isPreview), nil
}

func decode(m resource.PropertyMap, dst any, preview bool) (
	[]resource.PropertyPath, mapper.MappingError) {
	if m.ContainsUnknowns() {
		m = typeUnknowns(resource.NewObjectProperty(m), reflect.TypeOf(dst)).ObjectValue()
	}
	m, secrets := extractSecrets(m)
	return secrets, mapper.New(&mapper.Opts{
		IgnoreMissing: preview,
	}).Decode(m.Mappable(), dst)
}

// typeUnknowns produces a map with identical values to m module unknown values have the
// correct type to be desierialized to dst.
func typeUnknowns(m resource.PropertyValue, dst reflect.Type) resource.PropertyValue {
	for dst.Kind() == reflect.Pointer {
		dst = dst.Elem()
	}
	if m.IsSecret() {
		return resource.MakeSecret(typeUnknowns(m.SecretValue().Element, dst))
	}
	if m.IsOutput() {
		v := m.OutputValue()
		if !v.Known {
			switch dst.Kind() {
			case reflect.Struct:
				e := resource.NewObjectProperty(resource.PropertyMap{})
				v.Element = typeUnknowns(e, dst)
			case reflect.Map:
				v.Element = resource.NewObjectProperty(resource.PropertyMap{})
			case reflect.String:
				v.Element = resource.NewStringProperty("")
			case reflect.Bool:
				v.Element = resource.NewBoolProperty(false)
			case reflect.Int, reflect.Int64, reflect.Float32, reflect.Float64:
				v.Element = resource.NewNumberProperty(0)
			case reflect.Array, reflect.Slice:
				v.Element = resource.NewArrayProperty([]resource.PropertyValue{})
			}
		}
		return resource.NewOutputProperty(v)
	}

	switch dst.Kind() {
	case reflect.Array, reflect.Slice:
		var results []resource.PropertyValue
		if m.IsArray() {
			results = make([]resource.PropertyValue, len(m.ArrayValue()))
			for i, v := range m.ArrayValue() {
				results[i] = typeUnknowns(v, dst.Elem())
			}
		}
		return resource.NewArrayProperty(results)
	case reflect.Map:
		var result resource.PropertyMap
		if m.IsObject() {
			result = make(resource.PropertyMap, len(m.ObjectValue()))
			for k, v := range m.ObjectValue() {
				result[k] = typeUnknowns(v, dst.Elem())
			}
		}
		return resource.NewObjectProperty(result)
	case reflect.Struct:
		var result resource.PropertyMap
		obj := resource.PropertyMap{}
		if m.IsObject() {
			obj = m.ObjectValue()
		}
		result = make(resource.PropertyMap, len(obj))
		for _, field := range reflect.VisibleFields(dst) {
			tag, err := introspect.ParseTag(field)
			if err != nil || tag.Internal {
				continue
			}
			v, ok := obj[resource.PropertyKey(tag.Name)]
			if !ok {
				if tag.Optional {
					continue
				}
				// Create a new unknown output, which we will then type
				v = resource.NewOutputProperty(resource.Output{
					Element: resource.NewNullProperty(),
					Known:   false,
				})
			}
			result[resource.PropertyKey(tag.Name)] = typeUnknowns(v, field.Type)
		}
		return resource.NewObjectProperty(result)
	default:
		return m
	}
}

func encode(src interface{}, secrets []resource.PropertyPath, preview bool) (
	resource.PropertyMap, mapper.MappingError) {
	props, err := mapper.New(&mapper.Opts{
		IgnoreMissing: preview,
	}).Encode(src)
	if err != nil {
		return nil, err
	}
	return insertSecrets(resource.NewPropertyMapFromMap(props), secrets), nil
}

// Transform secret values into plain values, returning the new map and the list of keys
// that contained secrets.
func extractSecrets(m resource.PropertyMap) (resource.PropertyMap, []resource.PropertyPath) {
	newMap := resource.PropertyMap{}
	secrets := []resource.PropertyPath{}
	var removeSecrets func(resource.PropertyValue, resource.PropertyPath) resource.PropertyValue
	removeSecrets = func(v resource.PropertyValue, path resource.PropertyPath) resource.PropertyValue {
		switch {
		case v.IsSecret():
			// To allow full fidelity reconstructing maps, we extract nested secrets
			// first. We then extract the top level secret. We need this ordering to
			// re-embed nested secrets.
			el := removeSecrets(v.SecretValue().Element, path)
			secrets = append(secrets, path)
			return el
		case v.IsComputed():
			return removeSecrets(v.Input().Element, path)
		case v.IsOutput():
			if !v.OutputValue().Known {
				return resource.NewNullProperty()
			}
			return removeSecrets(v.OutputValue().Element, path)
		case v.IsArray():
			arr := make([]resource.PropertyValue, len(v.ArrayValue()))
			for i, e := range v.ArrayValue() {
				arr[i] = removeSecrets(e, append(path, i))
			}
			return resource.NewArrayProperty(arr)
		case v.IsObject():
			m := make(resource.PropertyMap, len(v.ObjectValue()))
			for k, v := range v.ObjectValue() {
				m[k] = removeSecrets(v, append(path, string(k)))
			}
			return resource.NewObjectProperty(m)
		default:
			return v
		}
	}
	for k, v := range m {
		newMap[k] = removeSecrets(v, resource.PropertyPath{string(k)})
	}
	contract.Assertf(!newMap.ContainsSecrets(), "%d secrets removed", len(secrets))
	return newMap, secrets
}

func insertSecrets(props resource.PropertyMap, secrets []resource.PropertyPath) resource.PropertyMap {
	m := resource.NewObjectProperty(props)
	for _, s := range secrets {
		v, ok := s.Get(m)
		if !ok {
			continue
		}
		s.Set(m, resource.MakeSecret(v))
	}
	return m.ObjectValue()
}
