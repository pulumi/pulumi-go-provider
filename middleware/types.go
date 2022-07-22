package middleware

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	p "github.com/iwahbe/pulumi-go-provider"
)

type CustomResource interface {
	Check(p.Context, p.CheckRequest) (p.CheckResponse, error)
	Diff(p.Context, p.DiffRequest) (p.DiffResponse, error)
	Create(p.Context, p.CreateRequest) (p.CreateResponse, error)
	Read(p.Context, p.ReadRequest) (p.ReadResponse, error)
	Update(p.Context, p.UpdateRequest) (p.UpdateResponse, error)
	Delete(p.Context, p.DeleteRequest) error
}

type ComponentResource interface {
	Construct(pctx p.Context, typ string, name string, ctx *pulumi.Context, inputs pulumi.Map, opts pulumi.ResourceOption) (pulumi.ComponentResource, error)
}

type Invoke interface {
	Invoke(p.Context, p.InvokeRequest) (p.InvokeResponse, error)
}
