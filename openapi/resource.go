package openapi

import (
	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

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

	// Override the default diff behavior.
	//
	// If not overridden, a structured diff is used.
	Diff func(p.Context, p.DiffRequest) (p.DiffResponse, error)
	// Override the default check behavior.
	//
	// If not overridden, the type information provided by the OpenAPI schema is used.
	Check func(p.Context, p.CheckRequest) (p.CheckResponse, error)
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
	inputs := properties{}
	props := properties{}
	state := properties{}

	for _, op := range []*Operation{r.Resource.Create, r.Resource.Update, r.Resource.Delete} {
		if op != nil {
			in, e := op.schemaInputs(reg)
			if !err(e) {
				err(inputs.unionWith(in))
			}
			out, e := op.schemaOutputs(reg)
			if !err(e) {
				err(props.unionWith(out))
			}
		}
	}
	if r.Resource.Read != nil {
		s, e := r.Resource.Read.schemaInputs(reg)
		if !err(e) {
			err(state.unionWith(s))
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

func (r *resource) Check(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
	if r.Resource.Check != nil {
		return r.Resource.Check(ctx, req)
	}
	return r.defaultCheck(ctx, req)
}

func (r *resource) defaultCheck(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
	panic("unimplemented")
}

func (r *resource) Create(ctx p.Context, req p.CreateRequest) (p.CreateResponse, error) {
	panic("unimplemented")
}

func (r *resource) Delete(ctx p.Context, req p.DeleteRequest) error {
	panic("unimplemented")
}

func (r *resource) Read(ctx p.Context, req p.ReadRequest) (p.ReadResponse, error) {
	panic("unimplemented")
}

func (r *resource) Update(ctx p.Context, req p.UpdateRequest) (p.UpdateResponse, error) {
	panic("unimplemented")
}
