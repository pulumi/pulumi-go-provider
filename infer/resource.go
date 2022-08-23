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
	"sync"

	"github.com/hashicorp/go-multierror"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
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

	// The set of tags that should always be secret
	alwaysSecret map[string]bool
	// The set of tags that should never be secret
	neverSecret map[string]bool
	// A map from a field to its dependencies
	deps map[string][]string
}

// MarkMap mutates m to comply with the result of the fieldGenerator, applying
// computedness and secretness as appropriate.
func (g *fieldGenerator) MarkMap(inputs, m resource.PropertyMap) {
	// Flow secretness and computedness
	for output, inputList := range g.deps {
		output := resource.PropertyKey(output)
		for _, input := range inputList {
			input := inputs[resource.PropertyKey(input)]
			if input.IsComputed() && !m[output].IsComputed() {
				m[output] = resource.MakeComputed(m[output])
				break
			}
			if input.IsSecret() && !m[output].IsSecret() {
				m[output] = resource.MakeSecret(m[output])
			}
		}
	}

	// Create mandatory secrets
	for s := range g.alwaysSecret {
		s := resource.PropertyKey(s)
		if m[s].IsComputed() {
			break
		}
		if !m[s].IsSecret() {
			v := m[s]
			m[s] = resource.NewSecretProperty(&resource.Secret{Element: v})
		}
	}
	// Remove never secrets
	for s := range g.neverSecret {
		s := resource.PropertyKey(s)
		if m[s].IsComputed() {
			break
		}
		if m[s].IsSecret() {
			v := m[s]
			m[s] = v.SecretValue().Element
		}
	}
}

func (g *fieldGenerator) InputField(a any) InputField {
	field, ok, err := g.argsMatcher.GetField(a)
	if err != nil {
		g.err.Errors = append(g.err.Errors, err)
		return &errField{}
	}
	if ok {
		return &inputField{field}
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
	field, ok, err := g.stateMatcher.GetField(a)
	if err != nil {
		g.err.Errors = append(g.err.Errors, err)
		return &errField{}
	}
	if ok {
		return &outputField{g, field}
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

		alwaysSecret: map[string]bool{},
		neverSecret:  map[string]bool{},
		deps:         map[string][]string{},
	}
}

// The return value when an error happens. The error is reported when the errField is
// created, so this type only exists so we can return a valid instance of
// InputField/OutputField. The functions on errField do nothing.
type errField struct{}

func (*errField) AlwaysSecret()           {}
func (*errField) NeverSecret()            {}
func (*errField) DependsOn(...InputField) {}
func (*errField) isInputField()           {}
func (*errField) isOutputField()          {}

type inputField struct {
	field introspect.FieldTag
}

func (*inputField) isInputField() {}

type outputField struct {
	g     *fieldGenerator
	field introspect.FieldTag
}

func (f *outputField) AlwaysSecret() {
	name := f.field.Name
	f.g.alwaysSecret[name] = true
	if f.g.neverSecret[name] {
		f.g.err.Errors = append(f.g.err.Errors,
			fmt.Errorf("marked field %q as both always secret and never secret", name))
	}
}

func (f *outputField) NeverSecret() {
	name := f.field.Name
	f.g.neverSecret[name] = true
	if f.g.alwaysSecret[name] {
		f.g.err.Errors = append(f.g.err.Errors,
			fmt.Errorf("marked field %q as both always secret and never secret", name))
	}
}

func (f *outputField) DependsOn(deps ...InputField) {
	depNames := make([]string, 0, len(deps))
	for _, d := range deps {
		switch d := d.(type) {
		case *inputField:
			depNames = append(depNames, d.field.Name)
		case *errField:
			// The error was already reported, so do nothing
		default:
			panic(fmt.Sprintf("Unknown InputField type: %T", d))
		}
	}
	name := f.field.Name
	f.g.deps[name] = append(f.g.deps[name], depNames...)
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
	return &derivedResourceController[R, I, O]{m: map[resource.URN]*R{}}
}

type derivedResourceController[R CustomResource[I, O], I, O any] struct {
	m    map[resource.URN]*R
	lock sync.Mutex
}

func (*derivedResourceController[R, I, O]) isInferredResource() {}

func (rc *derivedResourceController[R, I, O]) GetSchema(reg schema.RegisterDerivativeType) (
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

func (rc *derivedResourceController[R, I, O]) GetToken() (tokens.Type, error) {
	var r R
	return introspect.GetToken("pkg", r)
}

func (rc *derivedResourceController[R, I, O]) getInstance(ctx p.Context, urn resource.URN, call string) *R {
	rc.lock.Lock()
	defer rc.lock.Unlock()
	_, ok := rc.m[urn]
	if !ok {
		if call != "Delete" && call != "Read" {
			ctx.Logf(diag.Warning, "Missing expect call to 'Check' before '%s' for resource %q", call, urn)
		}
		var r R
		rc.m[urn] = &r
	}
	return rc.m[urn]
}

func (rc *derivedResourceController[R, I, O]) Check(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
	var r R
	defer func() {
		rc.lock.Lock()
		defer rc.lock.Unlock()
		rc.m[req.Urn] = &r
	}()
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
	r := rc.getInstance(ctx, req.Urn, "Diff")
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
	objDiff := req.News.Diff(req.Olds, func(prop resource.PropertyKey) bool {
		// Olds is an Output, but news is an Input. Output should be a superset of Input,
		// so we need to filter out fields that are in Output but not Input.
		_, isInput := inputProps[string(prop)]
		return !isInput
	})
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
	r := rc.getInstance(ctx, req.Urn, "Create")

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

	if r, ok := ((interface{})(*r)).(ExplicitDependencies[I, O]); ok {
		fg := newFieldGenerator(&input, &o)
		r.WireDependencies(fg, &input, &o)
		if err = fg.err.ErrorOrNil(); err != nil {
			return p.CreateResponse{}, err
		}
		fg.MarkMap(req.Properties, m)
	} else if req.Properties.ContainsUnknowns() {
		for k, v := range m {
			if !v.IsComputed() {
				m[k] = resource.MakeComputed(v)
			}
		}
	}

	return p.CreateResponse{
		ID:         id,
		Properties: m,
	}, nil
}

func (rc *derivedResourceController[R, I, O]) Read(ctx p.Context, req p.ReadRequest) (p.ReadResponse, error) {
	r := rc.getInstance(ctx, req.Urn, "Read")
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
	r := rc.getInstance(ctx, req.Urn, "Update")
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
	if r, ok := ((interface{})(*r)).(ExplicitDependencies[I, O]); ok {
		fg := newFieldGenerator(&news, &o)
		r.WireDependencies(fg, &news, &o)
		if err = fg.err.ErrorOrNil(); err != nil {
			return p.UpdateResponse{}, err
		}
		fg.MarkMap(req.News, m)
	} else if req.News.ContainsUnknowns() {
		for k, v := range m {
			if !v.IsComputed() {
				m[k] = resource.MakeComputed(v)
			}
		}
	}

	return p.UpdateResponse{
		Properties: m,
	}, nil
}

func (rc *derivedResourceController[R, I, O]) Delete(ctx p.Context, req p.DeleteRequest) error {
	r := rc.getInstance(ctx, req.Urn, "Delete")
	del, ok := ((interface{})(*r)).(CustomDelete[O])
	if ok {
		var olds O
		_, err := decode(req.Properties, &olds, false)
		if err != nil {
			return err
		}
		return del.Delete(ctx, req.ID, olds)
	}
	rc.lock.Lock()
	defer rc.lock.Unlock()
	delete(rc.m, req.Urn)
	return nil
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
				} else {
					// Create a new unknown output, which we will then type
					v = resource.NewOutputProperty(resource.Output{
						Element: resource.NewNullProperty(),
						Known:   false,
					})
				}
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
