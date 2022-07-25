package infer

import (
	p "github.com/iwahbe/pulumi-go-provider"
	t "github.com/iwahbe/pulumi-go-provider/middleware"
	"github.com/iwahbe/pulumi-go-provider/middleware/dispatch"
	"github.com/iwahbe/pulumi-go-provider/middleware/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

var _ p.Provider = (*Provider)(nil)

type Provider struct {
	*schema.Provider
	dispatcher *dispatch.Provider
}

func NewProvider() *Provider {
	d := dispatch.Wrap(nil)
	return &Provider{
		dispatcher: d,
		Provider:   schema.Wrap(d),
	}
}

func (prov *Provider) WithResources(resources ...InferedResource) *Provider {
	res := map[tokens.Type]t.CustomResource{}
	sRes := []schema.Resource{}
	for _, r := range resources {
		typ, err := r.GetToken()
		contract.AssertNoError(err)
		res[typ] = r
		sRes = append(sRes, r)
	}
	prov.dispatcher.WithCustomResources(res)
	prov.Provider.WithResources(sRes...)
	return prov
}
