package infer

import (
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/mapper"

	p "github.com/iwahbe/pulumi-go-provider"
	t "github.com/iwahbe/pulumi-go-provider/middleware"
)

type CustomResource[I any, O any] interface {
	Create(ctx p.Context, input I, preview bool) (output O, err error)
}

type CustomCheck[I any] interface {
	// Maybe oldInputs can be of type I
	Check(ctx p.Context, oldInputs presource.PropertyMap, newInputs presource.PropertyMap) (I, []p.CheckFailure, error)
}

type CustomDiff[I, O any] interface {
	// Maybe oldInputs can be of type I
	Diff(ctx p.Context, id string, olds O, new I, ignoreChanges []string) (
		changes bool, replaces []string, stables []string, deleteBeforeReplace bool)
}

func Resource[R CustomResource[I, O], I, O any]() t.CustomResource {
	return &derivedResourceController[R, I, O]{map[presource.URN]R{}}
}

type derivedResourceController[R CustomResource[I, O], I, O any] struct {
	m map[presource.URN]R
}

func (rc *derivedResourceController[R, I, O]) Check(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
	var r R
	defer func() { rc.m[req.Urn] = r }()
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
	r, ok := rc.m[req.Urn]
	if !ok {
		ctx.Log(diag.Warning, "Missing expect call to Check before diff for resource %q", req.Urn)
	}
	if r, ok := ((interface{})(r)).(CustomDiff[I, O]); ok {
		// TODO: r.Diff(ctx, req.Id, olds O, new I, req.)
	}
}

func (rc *derivedResourceController[R, I, O]) Create(p.Context, p.CreateRequest) (p.CreateResponse, error) {

}

func (rc *derivedResourceController[R, I, O]) Read(p.Context, p.ReadRequest) (p.ReadResponse, error) {

}

func (rc *derivedResourceController[R, I, O]) Update(p.Context, p.UpdateRequest) (p.UpdateResponse, error) {

}

func (rc *derivedResourceController[R, I, O]) Delete(p.Context, p.DeleteRequest) error {

}
