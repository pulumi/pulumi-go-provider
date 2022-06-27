package provider

import (
	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
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

	return provider.Main(name, makeProviderfunc(opts))
}

func makeProviderfunc(opts options) func(*provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
	return func(host *provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
		return &server.Server{
			Host:   host,
			Schema: serialize(opts),
		}, nil
	}
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
