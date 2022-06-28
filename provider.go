package provider

import (
	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"github.com/pulumi/pulumi-go-provider/internal/server"
	"github.com/pulumi/pulumi-go-provider/resource"
)

func Run(name string, version semver.Version, providerOptions ...Options) error {
	opts := options{
		Name:    name,
		Version: version,
	}
	for _, o := range providerOptions {
		o(&opts)
	}
	makeProviderfunc, err := prepareProvider(opts)
	if err != nil {
		return err
	}
	return provider.Main(name, makeProviderfunc)
}

func prepareProvider(opts options) (func(*provider.HostClient) (pulumirpc.ResourceProviderServer, error), error) {

	pkg := tokens.NewPackageToken(tokens.PackageName(opts.Name))
	components, err := server.NewComponentResources(pkg, opts.Components)
	if err != nil {
		return nil, err
	}
	customs, err := server.NewCustomResources(pkg, opts.Resources)
	if err != nil {
		return nil, err
	}
	return func(host *provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
		return &server.Server{
			Host:    host,
			Version: opts.Version,

			Components: components,
			Customs:    customs,
		}, nil
	}, nil
}

type options struct {
	Name       string
	Version    semver.Version
	Resources  []resource.Custom
	Types      []interface{}
	Components []resource.Component
}

type Options func(*options)

func Resources(resources ...resource.Custom) Options {
	return func(o *options) {
		o.Resources = append(o.Resources, resources...)
	}
}

func Types(types ...interface{}) Options {
	return func(o *options) {
		o.Types = append(o.Types, types...)
	}
}

func Components(components ...resource.Component) Options {
	return func(o *options) {
		o.Components = append(o.Components, components...)
	}
}

// TODO: Add Invokes
