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
	"context"
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
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer/internal/ende"
	"github.com/pulumi/pulumi-go-provider/internal"
	"github.com/pulumi/pulumi-go-provider/internal/introspect"
	"github.com/pulumi/pulumi-go-provider/internal/putil"
	t "github.com/pulumi/pulumi-go-provider/middleware"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
)

// CustomResource is a [custom resource](https://www.pulumi.com/docs/concepts/resources/)
// inferred from code. This is the minimum requirement for defining a new custom resource.
//
// This interface should be implemented by the resource controller, with `I` the resource
// inputs and `O` the full set of resource fields. It is recommended that `O` is a
// superset of `I`, but it is not strictly required. The fields of `I` and `O` should
// consist of non-pulumi types i.e. `string` and `int` instead of `pulumi.StringInput` and
// `pulumi.IntOutput`.
//
// The behavior of a CustomResource resource can be extended by implementing any of the
// following interfaces on the resource controller:
//
// - [CustomCheck]
// - [CustomDiff]
// - [CustomUpdate]
// - [CustomRead]
// - [CustomDelete]
// - [CustomStateMigrations]
// - [Annotated]
//
// Example:
//
//	type MyResource struct{}
//
//	type MyResourceInputs struct {
//		MyString string `pulumi:"myString"`
//		OptionalInt *int `pulumi:"myInt,optional"`
//	}
//
//	type MyResourceOutputs struct {
//		MyResourceInputs
//		Result string `pulumi:"result"`
//	}
//
//	func (*MyResource) Create(
//		ctx context.Context, req infer.CreateRequest[MyResourceInputs],
//	) (infer.CreateResponse[MyResourceOutputs], error) {
//		id := req.Inputs.MyString + ".id"
//		if req.Preview {
//			return infer.CreateResponse[MyResourceOutputs]{
//				ID: id,
//				Output: MyResourceOutputs{MyResourceInputs: inputs},
//			}, nil
//		}
//
//		result := req.Inputs.MyString
//		if req.Inputs.OptionalInt != nil {
//			result = fmt.Sprintf("%s.%d", result, *req.Inputs.OptionalInt)
//		}
//
//		return infer.CreateResponse[MyResourceOutputs]{
//					ID: id,
//					Output: MyResourceOutputs{inputs, result},
//				}, nil
//	}
type CustomResource[I, O any] interface {
	// All custom resources must be able to be created.
	CustomCreate[I, O]
}

// CreateRequest contains all the parameters for a Create operation
type CreateRequest[I any] struct {
	// The resource name.
	Name string
	// The resource inputs.
	Inputs I
	// Whether this is a preview operation.
	Preview bool
}

// CreateResponse contains all the results from a Create operation
type CreateResponse[O any] struct {
	// The provider assigned unique ID of the created resource.
	ID string
	// The output state of the resource to checkpoint.
	Output O
}

type CustomCreate[I, O any] interface {
	Create(ctx context.Context, req CreateRequest[I]) (CreateResponse[O], error)
}

// CheckRequest contains all the parameters for a Check operation.
type CheckRequest struct {
	// The resource name.
	Name string
	// The old resource inputs.
	OldInputs property.Map
	// The new resource inputs.
	NewInputs property.Map
}

// CheckResponse contains all the results from a Check operation
type CheckResponse[I any] struct {
	// The validated inputs.
	Inputs I
	// Any validation failures encountered.
	Failures []p.CheckFailure
}

// CustomCheck describes a resource that understands how to check its inputs.
//
// By default, infer handles checks by ensuring that a inputs de-serialize correctly,
// applying default values and secrets. You can wrap the default behavior of Check by
// calling [DefaultCheck] inside of your custom Check implementation.
//
// This is where you can extend that behavior. The
// returned input is given to subsequent calls to `Create` and `Update`.
//
// Example:
// TODO - Maybe a resource that has a regex. We could reject invalid regex before the up
// actually happens.
// CheckRequest contains all the parameters for a Check operation
type CustomCheck[I any] interface {
	// Check validates the inputs for a resource.
	Check(ctx context.Context, req CheckRequest) (CheckResponse[I], error)
}

// DiffRequest contains all the parameters for a Diff operation
type DiffRequest[I, O any] struct {
	// The resource ID.
	ID string
	// The old resource state.
	Olds O
	// The new resource inputs.
	News I
}

// DiffResponse contains all the results from a Diff operation.
type DiffResponse = p.DiffResponse

// CustomDiff describes a resource that understands how to diff itself given a new set of
// inputs.
//
// By default, infer handles diffs by structural equality among inputs. If CustomUpdate is
// implemented, changes will result in updates. Otherwise changes will result in replaces.
//
// Example:
// TODO - Indicate replacements for certain changes but not others.
type CustomDiff[I, O any] interface {
	Diff(ctx context.Context, req DiffRequest[I, O]) (DiffResponse, error)
}

// UpdateRequest contains all the parameters for an Update operation
type UpdateRequest[I, O any] struct {
	// The resource ID.
	ID string
	// The old resource state.
	Olds O
	// The new resource inputs.
	News I
	// Whether this is a preview operation.
	Preview bool
}

// UpdateResponse contains all the results from an Update operation
type UpdateResponse[O any] struct {
	// The output state of the resource to checkpoint.
	Output O
}

// CustomUpdate descibes a resource that can adapt to new inputs with a delete and
// replace.
//
// There is no default behavior for CustomUpdate.
//
// Here the old state (as returned by Create or Update) as well as the new inputs are
// passed. Update should return the new state of the resource. If preview is true, then
// the update is part of `pulumi preview` and no changes should be made.
//
// Example:
//
//	TODO
type CustomUpdate[I, O any] interface {
	Update(ctx context.Context, req UpdateRequest[I, O]) (UpdateResponse[O], error)
}

// ReadRequest contains all the parameters for a Read operation
type ReadRequest[I, O any] struct {
	// The resource ID.
	ID string
	// The resource inputs.
	Inputs I
	// The current resource state.
	State O
}

// ReadResponse contains all the results from a Read operation
type ReadResponse[I, O any] struct {
	// The canonical ID of the resource.
	ID string
	// The normalized inputs.
	Inputs I
	// The normalized state.
	State O
}

// CustomRead describes resource that can recover its state from the provider.
//
// If CustomRead is not implemented, it will default to checking that the inputs and state
// fit into I and O respectively. If they do, then the values will be returned as is.
// Otherwise an error will be returned.
//
// Example:
// TODO - Probably something to do with the file system.
type CustomRead[I, O any] interface {
	// Read accepts a resource's current state and returns a normalized version.
	// It can be used to sync the Pulumi state with the actual resource state.

	Read(ctx context.Context, req ReadRequest[I, O]) (ReadResponse[I, O], error)
}

// DeleteRequest contains all the parameters for a Delete operation
type DeleteRequest[O any] struct {
	// The resource ID.
	ID string
	// The current resource state.
	State O
}

// DeleteResponse contains all the results from a Delete operation
type DeleteResponse struct {
	// Currently empty, but may be extended in the future to return additional
	// information.
}

// CustomDelete describes a resource that knows how to delete itself.
//
// If a resource does not implement Delete, no code will be run on resource deletion.
type CustomDelete[O any] interface {
	// Delete is called before a resource is removed from pulumi state.
	Delete(ctx context.Context, req DeleteRequest[O]) (DeleteResponse, error)
}

// StateMigrationFunc represents a stateless mapping from an old state shape to a new
// state shape. Each StateMigrationFunc is parameterized by the shape of the type it
// produces, ensuring that all successful migrations end up in a valid state.
//
// To create a StateMigrationFunc, use [StateMigration].
type StateMigrationFunc[New any] interface {
	isStateMigrationFunc()

	oldShape() reflect.Type
	newShape() reflect.Type
	migrateFunc() reflect.Value
}

// StateMigration creates a mapping from an old state shape (type Old) to a new state
// shape (type New).
//
// If Old = [resource.PropertyMap], then the migration is always run.
//
// Example:
//
//	type MyResource struct{}
//
//	type MyInput struct{}
//
//	type MyStateV1 struct {
//		SomeInt *int `pulumi:"someInt,optional"`
//	}
//
//	type MyStateV2 struct {
//		AString string `pulumi:"aString"`
//		AInt    *int   `pulumi:"aInt,optional"`
//	}
//
//	func migrateFromV1(ctx context.Context, v1 StateV1) (infer.MigrationResult[MigrateStateV2], error) {
//		return infer.MigrationResult[MigrateStateV2]{
//			Result: &MigrateStateV2{
//				AString: "default-string", // Add a new required field
//				AInt: v1.SomeInt, // Rename an existing field
//			},
//		}, nil
//	}
//
//	// Associate your migration with the resource it encapsulates.
//	func (*MyResource) StateMigrations(context.Context) []infer.StateMigrationFunc[MigrateStateV2] {
//		return []infer.StateMigrationFunc[MigrateStateV2]{
//			infer.StateMigration(migrateFromV1),
//		}
//	}
func StateMigration[Old, New any, F func(context.Context, Old) (MigrationResult[New], error)](
	f F,
) StateMigrationFunc[New] {
	return stateMigrationFunc[Old, New, F]{f}
}

// MigrationResult represents the result of a migration.
type MigrationResult[T any] struct {
	// Result is the result of the migration.
	//
	// If Result is nil, then the migration is considered to have been unnecessary.
	//
	// If Result is non-nil, then the migration is considered to have completed and
	// the new value state value will be *Result.
	Result *T
}

type stateMigrationFunc[Old, New any, F func(context.Context, Old) (MigrationResult[New], error)] struct{ f F }

func (stateMigrationFunc[O, N, F]) isStateMigrationFunc()        {}
func (stateMigrationFunc[O, N, F]) oldShape() reflect.Type       { return reflect.TypeFor[O]() }
func (stateMigrationFunc[O, N, F]) newShape() reflect.Type       { return reflect.TypeFor[N]() }
func (m stateMigrationFunc[O, N, F]) migrateFunc() reflect.Value { return reflect.ValueOf(m.f) }

type CustomStateMigrations[O any] interface {
	// StateMigrations is the list of know migrations.
	//
	// Each migration should return a valid State object.
	//
	// The first migration to return a non-nil Result will be used.
	StateMigrations(ctx context.Context) []StateMigrationFunc[O]
}

// Annotator is used as part of [Annotated] to describe schema metadata for a resource or
// type.
//
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
	SetToken(module tokens.ModuleName, name tokens.TypeName)

	// Add a type [alias](https://www.pulumi.com/docs/using-pulumi/pulumi-packages/schema/#alias) for
	// this resource.
	//
	// The module and the name will be assembled into a type specifier of the form
	// `mypkg:mymodule:MyResource`, in the same way `SetToken` does.
	AddAlias(module tokens.ModuleName, name tokens.TypeName)

	// Set a deprecation message for a struct field, which officially marks it as deprecated.
	//
	// For example:
	//
	//	func (*s Struct) Annotated(a Annotator) {
	//		a.Deprecate(&s.Field, "field is deprecated")
	//	}
	//
	// To deprecate a resource, object or function, call Deprecate on the struct itself:
	//
	//	func (*s Struct) Annotated(a Annotator) {
	//		a.Deprecate(&s, "Struct is deprecated")
	//	}
	Deprecate(i any, message string)
}

// Annotated is used to describe the fields of an object or a resource. Annotated can be
// implemented by `CustomResource`s, the input and output types for all resources and
// invokes, as well as other structs used the above.
type Annotated interface {
	Annotate(Annotator)
}

// FieldSelector is used to describe the relationship between fields.
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

// ExplicitDependencies describes a custom resource with the dataflow between its
// arguments (`I`) and outputs (`O`) specified. If a CustomResource implements
// ExplicitDependencies then WireDependencies will be called for each Create and Update
// call with `args` and `state` holding the values they will have for that call.
//
// If ExplicitDependencies is not implemented, it is assumed that all outputs depend on
// all inputs.
type ExplicitDependencies[I, O any] interface {
	// WireDependencies specifies the dependencies between inputs and outputs.
	WireDependencies(f FieldSelector, args *I, state *O)
}

// OutputField represents an output/state field to apply metadata to.
//
// See [FieldSelector] for details on usage.
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

// InputField represents an argument/input field to apply metadata to.
//
// See [FieldSelector] for details on usage.
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
	if field.known || putil.IsComputed(prop) {
		return prop
	}

	if input, ok := inputs[key]; ok && !putil.IsComputed(prop) && putil.DeepEquals(input, prop) {
		// prop is an output during a create, but the output mirrors an
		// input in name and value. We don't make it computed.
		return prop
	}

	// If this is during a create and the value is not explicitly marked as known, we mark it computed.
	if isCreate {
		return putil.MakeComputed(prop)
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
		if putil.IsComputed(inputs[k]) || (hasOldInput && !putil.DeepEquals(inputs[k], oldInput)) {
			return putil.MakeComputed(prop)
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
		return putil.MakePublic(prop)
	}

	if putil.IsSecret(prop) {
		return prop
	}

	// If we should always return a secret, ensure that the field *is* marked as secret,
	// then return.
	if field.alwaysSecret {
		return putil.MakeSecret(prop)
	}

	if input, ok := inputs[key]; ok && putil.DeepEquals(input, prop) {
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
			return putil.MakeSecret(prop)
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
	contract.AssertNoErrorf(err, "TargetStructFields on %v", g.args)

	state, ok, err := g.stateMatcher.TargetStructFields(g.state)
	contract.Assertf(ok, "we match by construction")
	contract.AssertNoErrorf(err, "TargetStructFields on %v", g.state)

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
	copy(input.fields, i.fields)
	return input
}

func (i *inputField) Secret() InputField {
	input := new(inputField)
	input.kind = inputSecret
	// Copy input fields
	input.fields = make([]introspect.FieldTag, len(i.fields))
	copy(input.fields, i.fields)
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

// InferredResource is a resource inferred by the Resource function.
//
// This interface cannot be implemented directly. Instead consult the Resource function.
type InferredResource interface {
	t.CustomResource
	schema.Resource

	isInferredResource()
}

// Resource creates a new InferredResource, where `R` is the resource controller, `I` is
// the resources inputs and `O` is the resources outputs.
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

func (rc *derivedResourceController[R, I, O]) Check(ctx context.Context, req p.CheckRequest) (p.CheckResponse, error) {
	var r R
	if r, ok := ((interface{})(r)).(CustomCheck[I]); ok {
		// The user implemented check manually, so call that.
		//
		// We do not apply defaults if the user has implemented Check
		// themselves. Defaults are applied by [DefaultCheck].
		encoder, i, failures, err := callCustomCheck(ctx, r, req.Urn.Name(), req.Olds, req.News)
		if err != nil {
			return p.CheckResponse{}, err
		}

		// callCustomCheck will have an encoder if and only if the custom check
		// calls [DefaultCheck].
		//
		// If it doesn't have an encoder, but no error was returned, we do our
		// best to recover secrets, unknowns, etc by calling
		// decodeCheckingMapErrors to re-derive an encoder to use.
		//
		// There isn't any guaranteed relationship between the shape of `req.News`
		// and `I`, so we are not guaranteed that `decodeCheckingMapErrors` won't
		// produce errors.
		if encoder == nil {
			backupEncoder, _, _, _ := decodeCheckingMapErrors[I](req.News)
			encoder = &backupEncoder
		}

		inputs, err := encoder.Encode(i)
		return p.CheckResponse{
			Inputs:   applySecrets[I](inputs),
			Failures: failures,
		}, err
	}

	encoder, i, failures, err := decodeCheckingMapErrors[I](req.News)
	if err != nil {
		return p.CheckResponse{}, err
	}
	if len(failures) > 0 {
		return p.CheckResponse{
			// If we failed to decode, we apply secrets pro-actively to ensure
			// that they don't leak into previews.
			Inputs:   applySecrets[I](resource.ToResourcePropertyValue(property.New(req.News)).ObjectValue()),
			Failures: failures,
		}, nil
	}

	if i, err = defaultCheck(i); err != nil {
		return p.CheckResponse{}, fmt.Errorf("unable to apply defaults: %w", err)
	}

	inputs, err := encoder.Encode(i)

	return p.CheckResponse{Inputs: applySecrets[I](inputs)}, err
}

// This (key,value) pair provide a mechanism for [DefaultCheck] to silently return the
// encoder it derives.
type (
	defaultCheckEncoderKey   struct{}
	defaultCheckEncoderValue struct{ enc *ende.Encoder }
)

// callCustomCheck should be used to call [CustomCheck.Check].
//
// callCustomCheck facilitates extracting the encoder created with [DefaultCheck].
func callCustomCheck[T any](
	ctx context.Context, r CustomCheck[T], name string, olds, news property.Map,
) (*ende.Encoder, T, []p.CheckFailure, error) {
	defaultCheckEncoder := new(defaultCheckEncoderValue)
	ctx = context.WithValue(ctx, defaultCheckEncoderKey{}, defaultCheckEncoder)
	resp, err := r.Check(ctx, CheckRequest{
		Name:      name,
		OldInputs: olds,
		NewInputs: news,
	})
	return defaultCheckEncoder.enc, resp.Inputs, resp.Failures, err
}

// DefaultCheck verifies that inputs can deserialize cleanly into I. This is the default
// validation that is performed when leaving Check unimplemented.
//
// It also adds defaults to inputs as necessary, as defined by [Annotator.SetDefault].
func DefaultCheck[I any](ctx context.Context, inputs property.Map) (I, []p.CheckFailure, error) {
	enc, i, failures, err := decodeCheckingMapErrors[I](inputs)

	if v, ok := ctx.Value(defaultCheckEncoderKey{}).(*defaultCheckEncoderValue); ok {
		v.enc = &enc
	}

	if err != nil || len(failures) > 0 {
		return i, failures, err
	}

	i, err = defaultCheck(i)
	return i, nil, err
}

func defaultCheck[I any](i I) (I, error) {
	if err := applyDefaults(&i); err != nil {
		return i, fmt.Errorf("unable to apply defaults: %w", err)
	}
	return i, nil
}

func decodeCheckingMapErrors[I any](inputs property.Map) (ende.Encoder, I, []p.CheckFailure, error) {
	encoder, i, err := ende.Decode[I](inputs)
	if err != nil {
		failures, e := checkFailureFromMapError(err)
		return encoder, i, failures, e
	}

	return encoder, i, nil, nil
}

// checkFailureFromMapError converts from a [mapper.MappingError] to a [p.CheckFailure]:
//
//	err is nil -> nil, nil
//	all err.Failures are FieldErrors -> []p.CheckFailure, nil
//	otherwise -> unspecified, err
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

func (rc *derivedResourceController[R, I, O]) Diff(ctx context.Context, req p.DiffRequest) (p.DiffResponse, error) {
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
func diff[R, I, O any](
	ctx context.Context, req p.DiffRequest, r *R, forceReplace func(string) bool,
) (p.DiffResponse, error) {

	for _, ignoredChange := range req.IgnoreChanges {
		v, ok := req.Olds.GetOk(ignoredChange)
		if ok {
			req.News = req.News.Set(ignoredChange, v)
		}
	}

	if r, ok := ((interface{})(*r)).(CustomDiff[I, O]); ok {
		_, olds, err := hydrateFromState[R, I, O](ctx, req.Olds) // TODO
		if err != nil {
			return p.DiffResponse{}, err
		}
		_, news, err := ende.Decode[I](req.News)
		if err != nil {
			return p.DiffResponse{}, err
		}
		resp, err := r.Diff(ctx, DiffRequest[I, O]{
			ID:   req.ID,
			Olds: olds,
			News: news,
		})
		if err != nil {
			return p.DiffResponse{}, err
		}
		return resp, nil
	}

	inputProps, err := introspect.FindProperties(reflect.TypeFor[I]())
	if err != nil {
		return p.DiffResponse{}, err
	}
	// Olds is an Output, but news is an Input. Output should be a superset of Input,
	// so we need to filter out fields that are in Output but not Input.
	oldInputs := map[string]property.Value{}
	for k := range inputProps {
		oldInputs[k] = req.Olds.Get(k)
	}
	objDiff := resource.ToResourcePropertyValue(property.New(oldInputs)).ObjectValue().Diff(
		resource.ToResourcePropertyValue(property.New(req.News)).ObjectValue(),
	)
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
	ctx context.Context, req p.CreateRequest,
) (resp p.CreateResponse, retError error) {
	r := rc.getInstance()

	var err error
	encoder, input, err := ende.Decode[I](req.Properties)
	if err != nil {
		return p.CreateResponse{}, fmt.Errorf("invalid inputs: %w", err)
	}

	inferResp, err := (*r).Create(ctx, CreateRequest[I]{
		Name:    req.Urn.Name(),
		Inputs:  input,
		Preview: req.Preview,
	})
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

	if inferResp.ID == "" && !req.Preview {
		return p.CreateResponse{}, ProviderErrorf("'%s' was created without an id", req.Urn)
	}

	m, err := encoder.AllowUnknown(req.Preview).Encode(inferResp.Output)
	if err != nil {
		return p.CreateResponse{}, fmt.Errorf("encoding resource properties: %w", err)
	}

	setDeps, err := getDependencies(r, &input, &inferResp.Output, true /* isCreate */, req.Preview)
	if err != nil {
		return p.CreateResponse{}, err
	}
	setDeps(nil, resource.ToResourcePropertyValue(property.New(req.Properties)).ObjectValue(), m)

	return p.CreateResponse{
		ID:         inferResp.ID,
		Properties: resource.FromResourcePropertyValue(resource.NewProperty(m)).AsMap(),
	}, err
}

func (rc *derivedResourceController[R, I, O]) Read(
	ctx context.Context, req p.ReadRequest,
) (resp p.ReadResponse, retError error) {
	r := rc.getInstance()
	var inputs I
	var err error
	inputEncoder, err := ende.DecodeTolerateMissing(req.Inputs, &inputs)
	if err != nil {
		return p.ReadResponse{}, err
	}

	// We decode the resource state.
	//
	// The state can come from 2 places:
	//
	// 1. From the actual stack state.
	//
	// 2. From the state field for an import.
	//
	// Unfortunately, we are unable to distinguish between (1) and (2). We try (1), which has stricter
	// requirements, then try (2).
	var stateEncoder ende.Encoder
	var state O

	// If (1), then we expect that the state is complete and may need to be upgraded.
	if enc, s, err := hydrateFromState[R, I, O](ctx, req.Properties); err == nil {
		stateEncoder = enc
		state = s
	} else {
		// That didn't work, so maybe we can get by decoding without state migration but by tolerating
		// missing fields.
		stateEncoder, err = ende.DecodeTolerateMissing(req.Properties, &state)
		if err != nil {
			return p.ReadResponse{}, err
		}
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
	inferResp, err := read.Read(ctx, ReadRequest[I, O]{
		ID:     req.ID,
		Inputs: inputs,
		State:  state,
	})
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

	i, err := inputEncoder.Encode(inferResp.Inputs)
	if err != nil {
		return p.ReadResponse{}, err
	}
	s, err := stateEncoder.Encode(inferResp.State)
	if err != nil {
		return p.ReadResponse{}, err
	}

	return p.ReadResponse{
		ID:         inferResp.ID,
		Properties: resource.FromResourcePropertyValue(resource.NewProperty(s)).AsMap(),
		Inputs:     resource.FromResourcePropertyValue(resource.NewProperty(i)).AsMap(),
	}, nil
}

func (rc *derivedResourceController[R, I, O]) Update(
	ctx context.Context, req p.UpdateRequest,
) (resp p.UpdateResponse, retError error) {
	r := rc.getInstance()
	update, ok := ((interface{})(*r)).(CustomUpdate[I, O])
	if !ok {
		return p.UpdateResponse{}, status.Errorf(codes.Unimplemented,
			"Update is not implemented for resource %s", req.Urn)
	}
	for _, ignoredChange := range req.IgnoreChanges {
		v, ok := req.Olds.GetOk(ignoredChange)
		if ok {
			req.News = req.News.Set(ignoredChange, v)
		}
	}

	_, olds, err := hydrateFromState[R, I, O](ctx, req.Olds)
	if err != nil {
		return p.UpdateResponse{}, err
	}
	encoder, news, err := ende.Decode[I](req.News)
	if err != nil {
		return p.UpdateResponse{}, err
	}
	inferResp, err := update.Update(ctx, UpdateRequest[I, O]{
		ID:      req.ID,
		Olds:    olds,
		News:    news,
		Preview: req.Preview,
	})
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
	m, err := encoder.AllowUnknown(req.Preview).Encode(inferResp.Output)
	if err != nil {
		return p.UpdateResponse{}, err
	}
	setDeps, err := getDependencies(r, &news, &inferResp.Output, false /* isCreate */, req.Preview)
	if err != nil {
		return p.UpdateResponse{}, err
	}
	setDeps(
		resource.ToResourcePropertyValue(property.New(req.Olds)).ObjectValue(),
		resource.ToResourcePropertyValue(property.New(req.News)).ObjectValue(),
		m,
	)

	return p.UpdateResponse{
		Properties: resource.FromResourcePropertyValue(resource.NewProperty(m)).AsMap(),
	}, nil
}

func (rc *derivedResourceController[R, I, O]) Delete(ctx context.Context, req p.DeleteRequest) error {
	r := rc.getInstance()
	del, ok := ((interface{})(*r)).(CustomDelete[O])
	if ok {
		_, olds, err := hydrateFromState[R, I, O](ctx, req.Properties)
		if err != nil {
			return err
		}
		_, err = del.Delete(ctx, DeleteRequest[O]{
			ID:    req.ID,
			State: olds,
		})
		return err
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

// hydrateFromState takes a blob from state and hydrates it for user consumption, running any relevant state
// migrations.
func hydrateFromState[R, I, O any](
	ctx context.Context, state property.Map,
) (ende.Encoder, O, error) {
	var r R
	if r, ok := ((interface{})(r)).(CustomStateMigrations[O]); ok {
		enc, newState, didMigrate, err := migrateState(ctx, r, state)
		if err != nil || didMigrate {
			return enc, newState, err
		}
	}

	return ende.Decode[O](state)
}

func migrateState[O any](
	ctx context.Context, r CustomStateMigrations[O], state property.Map,
) (ende.Encoder, O, bool, error) {
	var o O
	for _, upgrader := range r.StateMigrations(ctx) {
		oldType := upgrader.oldShape()
		f := upgrader.migrateFunc()

		// If the old type is a resource.PropertyMap, we always run the migration
		// func.

		var results []reflect.Value
		var enc ende.Encoder
		if oldType == reflect.TypeOf(property.Map{}) {
			results = f.Call([]reflect.Value{
				reflect.ValueOf(ctx), reflect.ValueOf(state),
			})
		} else {
			oldValue := reflect.New(oldType)

			var err error
			enc, err = ende.DecodeAny(state, oldValue.Interface())
			if err != nil {
				// If we couldn't encode cleanly, then state doesn't fit into the migrator.
				continue
			}

			results = f.Call([]reflect.Value{reflect.ValueOf(ctx), oldValue.Elem()})

		}

		contract.Assertf(len(results) == 2,
			"upgrader.migrateFunc() returned an invalid value %#v (%[1]T)", f,
		)

		contract.Assertf(f.Type().Out(1).Name() == "error",
			"The signature guarantees of f mandate the second argument is an error, found %s",
			f.Type().Out(1))
		err, _ := results[1].Interface().(error)
		if err != nil {
			return ende.Encoder{}, o, true, err
		}
		result, ok := results[0].Interface().(MigrationResult[O])
		contract.Assertf(ok,
			"The signature guarantees of f mandate the second argument is an %T, found %T",
			result, results[0].Interface())

		if result.Result == nil {
			continue
		}

		// The migration succeeded, so we are done
		//
		// NOTE: This design throws away all secrets from the state when deserializing from raw, and
		// doesn't move secrets with path changes when deserializing from non-raw.
		//
		// Should we warn if state contains secrets.
		//
		// Without a richer value representation
		// (https://github.com/pulumi/pulumi-go-provider/issues/212), this is inevitable for any
		// strongly typed design.
		//
		// We could allow an escape hatch by allowing MigrationResult[O] to be a union of O and
		// resource.PropertyMap where resource.PropertyMap guarantees that it encodes into O safely.
		return enc, *result.Result, true, nil
	}

	// No migration was run
	return ende.Encoder{}, o, false, nil
}
