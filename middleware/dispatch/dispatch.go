package dispatch

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	p "github.com/iwahbe/pulumi-go-provider"
	t "github.com/iwahbe/pulumi-go-provider/middleware"
)

type Dispatcher struct {
	p.Provider
	customs    map[string]t.CustomResource
	components map[string]t.ComponentResource
	invokes    map[string]t.Invoke
}

func Wrap(provider p.Provider) Dispatcher {
	if provider == nil {
		provider = &t.Scaffold{}
	}
	return Dispatcher{
		Provider:   provider,
		customs:    map[string]t.CustomResource{},
		components: map[string]t.ComponentResource{},
		invokes:    map[string]t.Invoke{},
	}
}

func normalize(tk tokens.Type) string {
	return tk.Module().Name().String() + tokens.TokenDelimiter + tk.Name().String()
}

func fixupError(err error) error {
	if status.Code(err) == codes.Unimplemented {
		err = status.Error(codes.NotFound, "Token not found")
	}
	return err
}

func (d Dispatcher) WithCustomResources(resources map[tokens.Type]t.CustomResource) Dispatcher {
	for k, v := range resources {
		d.customs[normalize(k)] = v
	}
	return d
}

func (d Dispatcher) WithComponentResources(components map[tokens.Type]t.ComponentResource) Dispatcher {
	for k, v := range components {
		d.components[normalize(k)] = v
	}
	return d
}

func (d Dispatcher) WithInvokes(invokes map[tokens.Type]t.Invoke) Dispatcher {
	for k, v := range invokes {
		d.invokes[normalize(k)] = v
	}
	return d
}

func (d *Dispatcher) Invoke(ctx p.Context, req p.InvokeRequest) (p.InvokeResponse, error) {
	inv, ok := d.invokes[normalize(req.Token)]
	if ok {
		return inv.Invoke(ctx, req)
	}
	r, err := d.Provider.Invoke(ctx, req)
	return r, fixupError(err)
}

func (d *Dispatcher) Check(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
	r, ok := d.customs[normalize(req.Urn.Type())]
	if ok {
		return r.Check(ctx, req)
	}
	c, err := d.Provider.Check(ctx, req)
	return c, fixupError(err)
}

func (d *Dispatcher) Diff(ctx p.Context, req p.DiffRequest) (p.DiffResponse, error) {
	r, ok := d.customs[normalize(req.Urn.Type())]
	if ok {
		return r.Diff(ctx, req)
	}
	diff, err := d.Provider.Diff(ctx, req)
	return diff, fixupError(err)

}

func (d *Dispatcher) Create(ctx p.Context, req p.CreateRequest) (p.CreateResponse, error) {
	r, ok := d.customs[normalize(req.Urn.Type())]
	if ok {
		return r.Create(ctx, req)
	}
	c, err := d.Provider.Create(ctx, req)
	return c, fixupError(err)
}

func (d *Dispatcher) Read(ctx p.Context, req p.ReadRequest) (p.ReadResponse, error) {
	r, ok := d.customs[normalize(req.Urn.Type())]
	if ok {
		return r.Read(ctx, req)
	}
	read, err := d.Provider.Read(ctx, req)
	return read, fixupError(err)
}

func (d *Dispatcher) Update(ctx p.Context, req p.UpdateRequest) (p.UpdateResponse, error) {
	r, ok := d.customs[normalize(req.Urn.Type())]
	if ok {
		return r.Update(ctx, req)
	}
	up, err := d.Provider.Update(ctx, req)
	return up, fixupError(err)
}

func (d *Dispatcher) Delete(ctx p.Context, req p.DeleteRequest) error {
	r, ok := d.customs[normalize(req.Urn.Type())]
	if ok {
		return r.Delete(ctx, req)
	}
	return fixupError(d.Provider.Delete(ctx, req))
}

func (d *Dispatcher) Construct(pctx p.Context, typ string, name string, ctx *pulumi.Context, inputs pulumi.Map, opts pulumi.ResourceOption) (pulumi.ComponentResource, error) {
	r, ok := d.components[typ]
	if ok {
		return r.Construct(pctx, typ, name, ctx, inputs, opts)
	}
	con, err := d.Provider.Construct(pctx, typ, name, ctx, inputs, opts)
	return con, fixupError(err)
}
