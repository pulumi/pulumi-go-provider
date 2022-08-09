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
	Diff(ctx p.Context, id string, olds O, news I, ignoreChanges []resource.PropertyKey) (
		p.DiffResponse, error)
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
// There is no default behavior for CustomRead.
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
// func (*s Struct) Annotated(a Annotator) {
//  a.Describe(&s, "A struct")            // Legal
//	a.Describe(&s.field1, "A field")      // Legal
//	a.Describe(s.field2, "A field")       // Not legal, since the pointer is missing.
//	otherS := &Struct{}
//	a.Describe(&otherS.field1, "A field") // Not legal, since describe is not called on its receiver.
// }
type Annotator interface {
	// Annotate a struct field with a text description.
	Describe(i any, description string)

	// Annotate a struct field with a default value. The default value must be a primitive
	// type in the pulumi type system.
	SetDefault(i any, defaultValue any)
}

// Annotated is used to describe the fields of an object or a resource. Annotated can be
// implemented by `CustomResource`s, the input and output types for all resources and
// invokes, as well as other structs used the above.
type Annotated interface {
	Annotate(Annotator)
}

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
	return &derivedResourceController[R, I, O]{map[resource.URN]*R{}}
}

type derivedResourceController[R CustomResource[I, O], I, O any] struct {
	m map[resource.URN]*R
}

func (derivedResourceController[R, I, O]) isInferredResource() {}

func (rc *derivedResourceController[R, I, O]) GetSchema(reg schema.RegisterDerivativeType) (
	pschema.ResourceSpec, error) {
	if err := registerTypes[I](reg); err != nil {
		return pschema.ResourceSpec{}, err
	}
	if err := registerTypes[O](reg); err != nil {
		return pschema.ResourceSpec{}, err
	}
	return getResourceSchema[R, I, O](false)
}

func (rc *derivedResourceController[R, I, O]) GetToken() (tokens.Type, error) {
	var r R
	return introspect.GetToken("pkg", r)
}

func (rc *derivedResourceController[R, I, O]) getInstance(ctx p.Context, urn resource.URN, call string) *R {
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
	defer func() { rc.m[req.Urn] = &r }()
	if r, ok := ((interface{})(r)).(CustomCheck[I]); ok {
		// The user implemented check manually, so call that
		i, failures, err := r.Check(ctx, req.Urn.Name().String(), req.Olds, req.News)
		if err != nil {
			return p.CheckResponse{}, err
		}
		inputs, err := rc.encode(i, nil, false)
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
	_, err := rc.decode(req.News, &i, false)
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

	if r, ok := ((interface{})(*r)).(CustomDiff[I, O]); ok {
		var olds O
		var news I
		var err error
		_, err = rc.decode(req.Olds, &olds, false)
		if err != nil {
			return p.DiffResponse{}, err
		}
		_, err = rc.decode(req.Olds, &news, false)
		if err != nil {
			return p.DiffResponse{}, err
		}
		diff, err := r.Diff(ctx, req.ID, olds, news, req.IgnoreChanges)
		if err != nil {
			return p.DiffResponse{}, err
		}
		return diff, nil
	}
	ignored := map[resource.PropertyKey]struct{}{}
	for _, k := range req.IgnoreChanges {
		ignored[k] = struct{}{}
	}
	inputProps, err := introspect.FindProperties(new(I))
	if err != nil {
		return p.DiffResponse{}, err
	}
	objDiff := req.News.Diff(req.Olds, func(prop resource.PropertyKey) bool {
		// Olds is an Output, but news is an Input. Output should be a superset of Input,
		// so we need to filter out fields that are in Output but not Input.
		_, ignore := ignored[prop]
		_, isInput := inputProps[string(prop)]
		return ignore || !isInput
	})
	pluginDiff := plugin.NewDetailedDiffFromObjectDiff(objDiff)
	diff := map[string]p.PropertyDiff{}
	_, hasUpdate := ((interface{})(*r)).(CustomUpdate[I, O])
	for k, v := range pluginDiff {
		set := func(kind p.DiffKind) {
			diff[k] = p.PropertyDiff{
				Kind:      kind,
				InputDiff: v.InputDiff,
			}
		}
		if !hasUpdate {
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
		DeleteBeforeReplace: !hasUpdate,
		HasChanges:          objDiff.AnyChanges(),
		DetailedDiff:        diff,
	}, nil
}

func (rc *derivedResourceController[R, I, O]) Create(ctx p.Context, req p.CreateRequest) (p.CreateResponse, error) {
	r := rc.getInstance(ctx, req.Urn, "Create")

	var input I
	var err error
	secrets, err := rc.decode(req.Properties, &input, req.Preview)
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

	m, err := rc.encode(o, secrets, req.Preview)
	if err != nil {
		return p.CreateResponse{}, fmt.Errorf("encoding resource properties: %w", err)
	}
	return p.CreateResponse{
		ID:         id,
		Properties: m,
	}, nil
}

func (rc *derivedResourceController[R, I, O]) Read(ctx p.Context, req p.ReadRequest) (p.ReadResponse, error) {
	r := rc.getInstance(ctx, req.Urn, "Read")
	read, ok := ((interface{})(*r)).(CustomRead[I, O])
	if !ok {
		return p.ReadResponse{}, status.Errorf(codes.Unimplemented, "Read is not implemented for resource %s", req.Urn)
	}
	var inputs I
	var state O
	var err error
	inputSecrets, err := rc.decode(req.Inputs, &inputs, true)
	if err != nil {
		return p.ReadResponse{}, err
	}
	stateSecrets, err := rc.decode(req.Properties, &state, true)
	if err != nil {
		return p.ReadResponse{}, err
	}
	id, inputs, state, err := read.Read(ctx, req.ID, inputs, state)
	if err != nil {
		return p.ReadResponse{}, err
	}
	i, err := rc.encode(inputs, inputSecrets, false)
	if err != nil {
		return p.ReadResponse{}, err
	}
	s, err := rc.encode(state, stateSecrets, false)
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
	_, err = rc.decode(req.Olds, &olds, req.Preview)
	if err != nil {
		return p.UpdateResponse{}, err
	}
	secrets, err := rc.decode(req.News, &news, req.Preview)
	if err != nil {
		return p.UpdateResponse{}, err
	}
	o, err := update.Update(ctx, req.ID, olds, news, req.Preview)
	if err != nil {
		return p.UpdateResponse{}, err
	}
	m, err := rc.encode(o, secrets, req.Preview)
	if err != nil {
		return p.UpdateResponse{}, err
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
		_, err := rc.decode(req.Properties, &olds, false)
		if err != nil {
			return err
		}
		return del.Delete(ctx, req.ID, olds)
	}
	delete(rc.m, req.Urn)
	return nil
}

func (rc *derivedResourceController[R, I, O]) decode(m resource.PropertyMap, dst interface{}, preview bool) (
	[]resource.PropertyKey, mapper.MappingError) {
	m, secrets := extractSecrets(m)
	return secrets, mapper.New(&mapper.Opts{
		IgnoreMissing: preview,
	}).Decode(m.Mappable(), dst)
}

func (*derivedResourceController[R, I, O]) encode(src interface{}, secrets []resource.PropertyKey, preview bool) (
	resource.PropertyMap, mapper.MappingError) {
	props, err := mapper.New(&mapper.Opts{
		IgnoreMissing: preview,
	}).Encode(src)
	if err != nil {
		return nil, err
	}
	m := resource.NewPropertyMapFromMap(props)
	for _, s := range secrets {
		v, ok := m[s]
		if !ok {
			continue
		}
		m[s] = resource.NewSecretProperty(&resource.Secret{Element: v})
	}
	return m, nil
}

// Transform secret values into plain values, returning the new map and the list of keys
// that contained secrets.
func extractSecrets(m resource.PropertyMap) (resource.PropertyMap, []resource.PropertyKey) {
	newMap := resource.PropertyMap{}
	secrets := []resource.PropertyKey{}
	var removeSecrets func(resource.PropertyValue) resource.PropertyValue
	removeSecrets = func(v resource.PropertyValue) resource.PropertyValue {
		switch {
		case v.IsSecret():
			return v.SecretValue().Element
		case v.IsComputed():
			return removeSecrets(v.Input().Element)
		case v.IsOutput():
			if !v.OutputValue().Known {
				return resource.NewNullProperty()
			}
			return removeSecrets(v.OutputValue().Element)
		case v.IsArray():
			arr := make([]resource.PropertyValue, len(v.ArrayValue()))
			for i, e := range v.ArrayValue() {
				arr[i] = removeSecrets(e)
			}
			return resource.NewArrayProperty(arr)
		case v.IsObject():
			m := make(resource.PropertyMap, len(v.ObjectValue()))
			for k, v := range v.ObjectValue() {
				m[k] = removeSecrets(v)
			}
			return resource.NewObjectProperty(m)
		default:
			return v
		}
	}
	for k, v := range m {
		if v.ContainsSecrets() {
			secrets = append(secrets, k)
		}
		newMap[k] = removeSecrets(v)
	}
	contract.Assertf(!newMap.ContainsSecrets(), "%d secrets removed", len(secrets))
	return newMap, secrets
}
