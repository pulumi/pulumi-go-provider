package infer

import (
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/mapper"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	p "github.com/iwahbe/pulumi-go-provider"
	"github.com/iwahbe/pulumi-go-provider/internal/introspect"
	t "github.com/iwahbe/pulumi-go-provider/middleware"
	schema "github.com/iwahbe/pulumi-go-provider/middleware/schema"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

type CustomResource[I any, O any] interface {
	Create(ctx p.Context, input I, preview bool) (id string, output O, err error)
}

type CustomCheck[I any] interface {
	// Maybe oldInputs can be of type I
	Check(ctx p.Context, oldInputs presource.PropertyMap, newInputs presource.PropertyMap) (I, []p.CheckFailure, error)
}

type CustomDiff[I, O any] interface {
	// Maybe oldInputs can be of type I
	Diff(ctx p.Context, id string, olds O, news I, ignoreChanges []resource.PropertyKey) (p.DiffResponse, error)
}

type CustomUpdate[I, O any] interface {
	Update(ctx p.Context, id string, olds, news I, preview bool) (O, error)
}

type CustomRead[I, O any] interface {
	Read(ctx p.Context, id string, inputs I, state O) (canonicalId string, normalizedInputs I, normalizedState O, err error)
}

type CustomDelete[O any] interface {
	Delete(ctx p.Context, id string, props O) error
}

type Annotator interface {
	// Annotate a a struct field with a text description.
	Describe(i any, description string)

	// Annotate a a struct field with a default value. The default value must be a primitive
	// type in the pulumi type system.
	SetDefault(i any, defaultValue any)
}

type Annotated interface {
	Annotate(Annotator)
}

type InferedResource interface {
	t.CustomResource
	schema.Resource
}

func Resource[R CustomResource[I, O], I, O any]() InferedResource {
	return &derivedResourceController[R, I, O]{map[presource.URN]*R{}}
}

type derivedResourceController[R CustomResource[I, O], I, O any] struct {
	m map[presource.URN]*R
}

func (rc *derivedResourceController[R, I, O]) GetSchema() (pschema.ResourceSpec, error) {
	var r R
	v := reflect.ValueOf(r)
	for v.Type().Kind() == reflect.Ptr {
		v = v.Elem()
	}
	descriptions := map[string]string{}
	if r, ok := (interface{})(r).(Annotated); ok {
		a := introspect.NewAnnotator(r)
		r.Annotate(&a)
		descriptions = a.Descriptions
	}

	properties, required, err := propertyListFromType[O]()
	if err != nil {
		var o O
		return pschema.ResourceSpec{}, fmt.Errorf("could not serialize output type %T: %w", o, err)
	}

	inputProperties, requiredInputs, err := propertyListFromType[I]()
	if err != nil {
		var i I
		return pschema.ResourceSpec{}, fmt.Errorf("could not serialize input type %T: %w", i, err)
	}

	return pschema.ResourceSpec{
		ObjectTypeSpec: pschema.ObjectTypeSpec{
			Properties:  properties,
			Description: descriptions[""],
			Required:    required,
		},
		InputProperties: inputProperties,
		RequiredInputs:  requiredInputs,
	}, nil
}

func (rc *derivedResourceController[R, I, O]) GetToken() (tokens.Type, error) {
	var r R
	return introspect.GetToken("pkg", r)
}

func (rc *derivedResourceController[R, I, O]) getInstance(ctx p.Context, urn presource.URN, call string) *R {
	_, ok := rc.m[urn]
	if !ok {
		ctx.Log(diag.Warning, "Missing expect call to %s before create for resource %q", call, urn)
		var r R
		rc.m[urn] = &r
	}
	return rc.m[urn]
}

func (rc *derivedResourceController[R, I, O]) Check(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
	var r R
	defer func() { rc.m[req.Urn] = &r }()
	if r, ok := ((interface{})(r)).(CustomCheck[I]); ok {
		// We implement check manually, so call that
		i, failures, err := r.Check(ctx, req.Olds, req.News)
		if err != nil {
			return p.CheckResponse{}, err
		}
		m, err := mapper.New(nil).Encode(i)
		if err != nil {
			return p.CheckResponse{}, err
		}
		return p.CheckResponse{
			Inputs:   presource.NewPropertyMapFromMap(m),
			Failures: failures,
		}, nil
	}
	// We have not implemented check, so do the smart thing by default
	// We just check that we can de-serialize correctly
	var i I
	err := mapper.New(nil).Decode(req.News.Mappable(), &i)
	if err == nil {
		return p.CheckResponse{
			Inputs: req.News,
		}, nil
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
			return p.CheckResponse{
				Failures: failures,
			}, fmt.Errorf("Unknown mapper error: %s", err.Error())
		}
	}
	return p.CheckResponse{
		Inputs:   req.News,
		Failures: failures,
	}, nil
}

func (rc *derivedResourceController[R, I, O]) Diff(ctx p.Context, req p.DiffRequest) (p.DiffResponse, error) {
	r := rc.getInstance(ctx, req.Urn, "Diff")

	if r, ok := ((interface{})(*r)).(CustomDiff[I, O]); ok {
		var olds O
		var news I
		var err error
		err = mapper.New(nil).Decode(req.Olds.Mappable(), &olds)
		if err != nil {
			return p.DiffResponse{}, err
		}
		err = mapper.New(nil).Decode(req.Olds.Mappable(), &news)
		if err != nil {
			return p.DiffResponse{}, err
		}
		diff, err := r.Diff(ctx, req.Id, olds, news, req.IgnoreChanges)
		if err != nil {
			return p.DiffResponse{}, err
		}
		return diff, nil
	}
	ignored := map[resource.PropertyKey]struct{}{}
	for _, k := range req.IgnoreChanges {
		ignored[k] = struct{}{}
	}
	objDiff := req.News.Diff(req.Olds, func(prop resource.PropertyKey) bool {
		_, ok := ignored[prop]
		return ok
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
	err = mapper.New(nil).Decode(req.Properties.Mappable(), &input)
	if err != nil {
		return p.CreateResponse{}, nil
	}

	id, o, err := (*r).Create(ctx, input, req.Preview)
	if err != nil {
		return p.CreateResponse{}, err
	}

	m, err := mapper.New(nil).Encode(o)
	if err != nil {
		return p.CreateResponse{}, err
	}
	return p.CreateResponse{
		Id:         id,
		Properties: presource.NewPropertyMapFromMap(m),
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
	err = mapper.New(nil).Decode(req.Inputs.Mappable(), &inputs)
	if err != nil {
		return p.ReadResponse{}, err
	}
	err = mapper.New(nil).Decode(req.Properties.Mappable(), &state)
	if err != nil {
		return p.ReadResponse{}, err
	}
	id, inputs, state, err := read.Read(ctx, req.Id, inputs, state)
	i, err := mapper.New(nil).Encode(inputs)
	if err != nil {
		return p.ReadResponse{}, err
	}
	s, err := mapper.New(nil).Encode(state)
	if err != nil {
		return p.ReadResponse{}, err
	}

	return p.ReadResponse{
		Id:         id,
		Properties: presource.NewPropertyMapFromMap(s),
		Inputs:     presource.NewPropertyMapFromMap(i),
	}, nil
}

func (rc *derivedResourceController[R, I, O]) Update(ctx p.Context, req p.UpdateRequest) (p.UpdateResponse, error) {
	r := rc.getInstance(ctx, req.Urn, "Update")
	update, ok := ((interface{})(*r)).(CustomUpdate[I, O])
	if !ok {
		return p.UpdateResponse{}, status.Errorf(codes.Unimplemented, "Update is not implemented for resource %s", req.Urn)
	}
	var olds, news I
	var err error
	err = mapper.New(nil).Decode(req.Olds.Mappable(), &olds)
	if err != nil {
		return p.UpdateResponse{}, err
	}
	err = mapper.New(nil).Decode(req.News.Mappable(), &news)
	if err != nil {
		return p.UpdateResponse{}, err
	}
	o, err := update.Update(ctx, req.Id, olds, news, req.Preview)
	if err != nil {
		return p.UpdateResponse{}, err
	}
	props, err := mapper.New(nil).Encode(o)
	if err != nil {
		return p.UpdateResponse{}, err
	}
	return p.UpdateResponse{
		Properties: presource.NewPropertyMapFromMap(props),
	}, nil
}

func (rc *derivedResourceController[R, I, O]) Delete(ctx p.Context, req p.DeleteRequest) error {
	r := rc.getInstance(ctx, req.Urn, "Delete")
	del, ok := ((interface{})(*r)).(CustomDelete[O])
	if ok {
		var olds O
		err := mapper.New(nil).Decode(req.Properties.Mappable(), &olds)
		if err != nil {
			return err
		}
		return del.Delete(ctx, req.Id, olds)
	}
	delete(rc.m, req.Urn)
	return nil
}
