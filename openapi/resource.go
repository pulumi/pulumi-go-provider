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

package openapi

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"strconv"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	p "github.com/pulumi/pulumi-go-provider"
	t "github.com/pulumi/pulumi-go-provider/middleware"
	s "github.com/pulumi/pulumi-go-provider/middleware/schema"
)

// Resource represents a Pulumi resource that participates in the CRUD lifecycle.
type Resource struct {
	Token              tokens.Type
	Description        string
	DeprecationMessage string

	Create *Operation
	Read   *Operation
	Update *Operation
	Delete *Operation

	Mappings Mappings

	// Override the default diff behavior.
	//
	// If not overridden, a structured diff is used.
	Diff func(p.Context, p.DiffRequest) (p.DiffResponse, error)
	// Override the default check behavior.
	//
	// If not overridden, the type information provided by the OpenAPI schema is used.
	Check func(p.Context, p.CheckRequest) (p.CheckResponse, error)
}

type Mappings []MapPair

type MapTarget struct {
	Operation *Operation
	Property  string
}

func (m MapTarget) is(r *Resource, op *Operation) bool {
	if m.Operation == op {
		return true
	}
	switch m.Operation {
	case createOpID:
		return r.Create == op
	case updateOpID:
		return r.Update == op
	case deleteOpID:
		return r.Delete == op
	case readOpID:
		return r.Read == op
	case allOpsID:
		return true
	}
	return false
}

type MapPair struct {
	From MapTarget
	To   MapTarget
}

// targets indicates if a parameter will be supplied by another parameter, instead of the
// user.
//
// A mapping is considered to target an op if a mapping goes from
// exclusively different ops to the passed op.
func (m Mappings) targets(r *Resource, op *Operation, property string) bool {
	for _, pair := range m {
		if pair.To.is(r, op) && pair.To.Property == property && !pair.From.is(r, op) {
			return true
		}
	}
	return false
}

func (m MapTarget) To(t MapTarget) MapPair {
	return MapPair{
		From: m,
		To:   t,
	}
}

// Operation IDs to stand in for pointers to actual operations.
var (
	createOpID = new(Operation)
	updateOpID = new(Operation)
	deleteOpID = new(Operation)
	readOpID   = new(Operation)
	allOpsID   = new(Operation)
)

// Map a property in the create operation of a resource.
func MapCreate(property string) MapTarget {
	return MapTarget{createOpID, property}
}

// Map a property in the update operation of a resource.
func MapUpdate(property string) MapTarget {
	return MapTarget{updateOpID, property}
}

// Map a property in the delete operation of a resource.
func MapDelete(property string) MapTarget {
	return MapTarget{deleteOpID, property}
}

// Map a property in the read operation of a resource.
func MapRead(property string) MapTarget {
	return MapTarget{readOpID, property}
}

// Map a property in every operation of a resource.
func MapAll(property string) MapTarget {
	return MapTarget{allOpsID, property}
}

func (r *Resource) Runnable() t.CustomResource {
	if r == nil {
		return nil
	}
	return &resource{*r}
}

func (r *Resource) Schema() s.Resource {
	if r == nil {
		return nil
	}
	return &resource{*r}
}

type resource struct {
	Resource
}

// Assert interface compliance:
var _ = (s.Resource)((*resource)(nil))
var _ = (t.CustomResource)((*resource)(nil))

func (r *resource) GetSchema(reg s.RegisterDerivativeType) (schema.ResourceSpec, error) {

	errs := multierror.Error{}
	err := func(err error) bool {
		if err == nil {
			return false
		}
		errs.Errors = append(errs.Errors, err)
		return true
	}
	inputs := r.collectInputs(reg, err)
	props := properties{}
	state := properties{}

	for _, op := range []*Operation{r.Resource.Create, r.Resource.Update, r.Resource.Delete} {
		op.mapping = r.Mappings
		if op != nil {
			out, e := op.schemaOutputs(&r.Resource, reg)
			if !err(e) {
				err(props.unionWith(out))
			}
		}
	}
	if r.Resource.Read != nil {
		s, e := r.Resource.Read.schemaInputs(&r.Resource, reg)
		if !err(e) {
			state = s
		}
	}

	return schema.ResourceSpec{
		ObjectTypeSpec: schema.ObjectTypeSpec{
			Description: r.Description,
			Properties:  props.props,
			Required:    props.required.SortedValues(),
		},
		InputProperties: inputs.props,
		RequiredInputs:  inputs.required.SortedValues(),
		StateInputs: &schema.ObjectTypeSpec{
			Properties: state.props,
			Required:   state.required.SortedValues(),
		},
		DeprecationMessage: r.DeprecationMessage,
	}, errs.ErrorOrNil()
}

func (r *resource) GetToken() (tokens.Type, error) {
	return r.Token, nil
}

func (r *resource) Diff(ctx p.Context, req p.DiffRequest) (p.DiffResponse, error) {
	if r.Resource.Diff != nil {
		return r.Resource.Diff(ctx, req)
	}
	return r.defaultDiff(ctx, req)
}

func (r *resource) defaultDiff(ctx p.Context, req p.DiffRequest) (p.DiffResponse, error) {
	// This default diff is copied from infer.resource. We should generalize this
	// solution.
	objDiff := req.News.Diff(req.Olds)
	pluginDiff := plugin.NewDetailedDiffFromObjectDiff(objDiff)
	diff := map[string]p.PropertyDiff{}
	for k, v := range pluginDiff {
		set := func(kind p.DiffKind) {
			diff[k] = p.PropertyDiff{
				Kind:      kind,
				InputDiff: v.InputDiff,
			}
		}
		if r.Resource.Update == nil {
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
		HasChanges:   objDiff.AnyChanges(),
		DetailedDiff: diff,
	}, nil
}

func (r *resource) collectInputs(reg s.RegisterDerivativeType, err func(error) bool) properties {
	inputs := properties{}
	for _, op := range []*Operation{r.Resource.Create, r.Resource.Update, r.Resource.Delete} {
		op.mapping = r.Mappings
		if op != nil {
			in, e := op.schemaInputs(&r.Resource, reg)
			if !err(e) {
				err(inputs.unionWith(in))
			}
		}
	}
	return inputs
}

func (r *resource) Check(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
	// We allow the user to mutate the inputs, but we still confirm that they are correct
	// on our own.
	if r.Resource.Check != nil {
		ret, err := r.Resource.Check(ctx, req)
		if err != nil || len(ret.Failures) > 0 {
			return ret, err
		}
		req.News = ret.Inputs
	}
	return r.schemaCheck(ctx, req)
}

// Verify that the provided inputs match the schema inputs.
//
// No effort is made to adjust the provided inputs to match the schema inputs.
func (r *resource) schemaCheck(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
	errs := multierror.Error{}
	addError := func(err error) bool {
		if err == nil {
			return false
		}
		errs.Errors = append(errs.Errors, err)
		return true
	}
	seen := map[tokens.Type]struct{}{}

	inputProps := r.collectInputs(func(tk tokens.Type, typ schema.ComplexTypeSpec) (unknown bool) {
		_, ok := seen[tk]
		if ok {
			seen[tk] = struct{}{}
		}
		return !ok
	}, addError)
	inputs := inputProps.rawTypes

	failures := []p.CheckFailure{}

	// Verify that news is a subset of inputs
	for k := range req.News {
		_, ok := inputs[string(k)]
		if !ok {
			failures = append(failures, p.CheckFailure{
				Property: string(k),
				Reason:   "unknown field",
			})
		}
	}

	// Verify that we are not missing a required input
	for k := range inputs {
		_, ok := req.News[presource.PropertyKey(k)]
		if !ok && inputProps.required.Has(k) {
			failures = append(failures, p.CheckFailure{
				Property: k,
				Reason:   "Missing required property",
			})
		}
	}

	// We have now verified that
	// 1. Each req.New property has a corresponding openapi3.Schema type
	// 2. Missing req.New properties are ok or accounted for.
	//
	// The only thing remaining is to assert that the inputs match the openapi3.Schema:
	for k, v := range req.News {
		schema, ok := inputs[string(k)]
		if !ok {
			continue
		}
		path, message, ok, err := checkTypes(string(k), v, schema)
		if err != nil {
			return p.CheckResponse{}, err
		}
		if !ok {
			failures = append(failures, p.CheckFailure{
				Property: path,
				Reason:   message,
			})
		}
	}

	return p.CheckResponse{
		Inputs:   req.News,
		Failures: failures,
	}, errs.ErrorOrNil()
}

func checkTypes(path string, val presource.PropertyValue, typ *openapi3.Schema) (string, string, bool, error) {
	ok := func() (string, string, bool, error) {
		return "", "", true, nil
	}
	failf := func(message string, a ...any) (string, string, bool, error) {
		return path, fmt.Sprintf(message, a...), false, nil
	}
	check := func(err error) (string, string, bool, error) {
		if err == nil {
			return ok()
		}
		return failf(err.Error())
	}
	expected := func(t string) (string, string, bool, error) {
		return failf("expected %s, found %s", typ.Type, t)
	}
	switch {

	// Types that are never allowed, since they are inexpressible in the OpenAPI schema:

	case val.IsArchive():
		return expected("archive")
	case val.IsAsset():
		return expected("asset")
	case val.IsResourceReference():
		// OpenAPI can't express resource reference's, so this will always error.
		return expected("resourceReference")

	// Basic types:

	case val.IsString():
		return check(typ.VisitJSONString(val.StringValue()))
	case val.IsBool():
		return check(typ.VisitJSONBoolean(val.BoolValue()))
	case val.IsNull():
		if typ.Nullable {
			return ok()
		}
		return failf("null is not an allowed value here")
	case val.IsNumber():
		return check(typ.VisitJSONNumber(val.NumberValue()))

	// Compound types:

	case val.IsArray():
		v := val.ArrayValue()
		// If the underlying type is an array, we check each element.
		if typ.Type == openapi3.TypeArray {
			for i, v := range v {
				// TODO: Submit multiple error messages in a check here.
				path, message, ok, err := checkTypes(fmt.Sprintf("%s[%d]", path, i), v, typ.Items.Value)
				if err != nil || !ok {
					return path, message, ok, err
				}
			}
		}
		// We perform a check of the array properties themselves, but we don't check the
		// elements, since openapi3 doesn't know how to do that.
		untypedArray := make([]any, len(v))
		for i, v := range v {
			untypedArray[i] = v
		}
		return check(typ.WithItems(&openapi3.Schema{}).VisitJSONArray(untypedArray))
	case val.IsObject():
		obj := val.ObjectValue()
		if len(typ.Properties) == 0 && len(obj) > 0 {
			return expected("object")
		}

		// Check that present values are what they should be.
		for k, v := range obj {
			path := fmt.Sprintf("%s.%s", path, string(k))
			typ, ok := typ.Properties[string(k)]
			if !ok {
				return path, "unexpected field", false, nil
			}
			path, message, ok, err := checkTypes(path, v, typ.Value)
			if !ok || err != nil {
				return path, message, ok, err
			}
		}

		// Check for missing values.
		for k, v := range typ.Properties {
			path := fmt.Sprintf("%s.%s", path, k)
			_, ok := obj[presource.PropertyKey(k)]
			if !ok && !v.Value.Nullable {
				return path, "missing required key", false, nil
			}
		}

		return ok()

	// Passthrough Pulumi types:

	case val.IsOutput():
		v := val.OutputValue()
		if v.Known {
			return checkTypes(path, v.Element, typ)
		}
		// We just accept unknown values. They are not generally typed correctly so it
		// isn't worth type checking them.
		return ok()
	case val.IsSecret():
		return checkTypes(path, val.SecretValue().Element, typ)

	default:
		return "", "", false, fmt.Errorf("unexpected property value kind: %T", val.V)
	}
}

func unwrapPValue(v presource.PropertyValue) presource.PropertyValue {
	for {
		switch {
		case v.IsSecret():
			v = v.SecretValue().Element
		case v.IsOutput():
			contract.Assert(v.OutputValue().Known)
			v = v.OutputValue().Element
		default:
			return v
		}
	}
}

func stringifyPValue(v presource.PropertyValue) (string, error) {
	v = unwrapPValue(v)
	switch {
	case v.IsString():
		return v.StringValue(), nil
	case v.IsNumber():
		return strconv.FormatFloat(v.NumberValue(), 'f', -1, 64), nil
	case v.IsBool():
		if v.BoolValue() {
			return "true", nil
		}
		return "false", nil
	default:
		return "", fmt.Errorf("could not stringify value of type %s", v.TypeString())
	}
}

func (r *resource) Create(ctx p.Context, req p.CreateRequest) (p.CreateResponse, error) {
	id := id(req.Urn)
	if req.Preview {
		return p.CreateResponse{
			ID: id,
		}, nil
	}

	result, err := runOp(ctx, &r.Resource, r.Resource.Create, req.Properties)
	return p.CreateResponse{
		ID:         id,
		Properties: result,
	}, err
}

func id(urn presource.URN) string {
	hasher := fnv.New64()
	_, err := hasher.Write([]byte(string(urn)))
	contract.AssertNoError(err)
	rand := rand.New(rand.NewSource(int64(hasher.Sum64()))) // nolint: gosec
	post := rand.Int() % 999_999
	return fmt.Sprintf("%s-%d", urn.Name().Name(), post)
}

func (r *resource) Delete(ctx p.Context, req p.DeleteRequest) error {
	_, err := runOp(ctx, &r.Resource, r.Resource.Delete, req.Properties)
	return err
}

func (r *resource) Read(ctx p.Context, req p.ReadRequest) (p.ReadResponse, error) {
	// Resource needs to layer ID, inputs and state together for this request.
	//
	// That will require some more work.
	panic("unimplemented")
}

func (r *resource) Update(ctx p.Context, req p.UpdateRequest) (p.UpdateResponse, error) {
	inputs := req.News.Copy()
	for _, c := range req.IgnoreChanges {
		inputs[c] = req.Olds[c]
	}
	if req.Preview {
		return p.UpdateResponse{
			Properties: req.Olds,
		}, nil
	}
	props, err := runOp(ctx, &r.Resource, r.Resource.Update, inputs)
	return p.UpdateResponse{
		Properties: props,
	}, err
}
