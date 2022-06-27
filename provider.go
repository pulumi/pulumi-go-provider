package provider

import (
	"context"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi-go-provider/internal/server"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
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
			Host: host,
		}, nil
	}
}

type options struct {
	Name       string
	Version    semver.Version
	Resources  []Resource
	Types      []interface{}
	Components []Component
}

type Options func(*options)

type Id = string

type Resource interface {
	// Create a resource.
	// Resource input properties will be applied to the resource the
	// method is called on. Output properties are set by manipulating the resource this
	// struct is called on.
	//
	// This means that implementing this method correctly requires passing the Resource
	// implementer by reference.
	Create(ctx context.Context, urn string, preview bool) (Id, error)
	Delete(ctx context.Context, id Id) error
}

type ResourceUpdate interface {
	Update(ctx context.Context, id Id, new interface{}, ignoreChanges []string, preview bool) error
}

type ResourceDiff interface {
	Diff(ctx context.Context, id Id, new interface{}, ignoreChanges []string) (DiffResponce, error)
}

type DiffResponceChangeType int32

const (
	ChangeUnknown = pulumirpc.DiffResponse_DIFF_UNKNOWN
	ChangeNone    = pulumirpc.DiffResponse_DIFF_NONE
	ChangeSome    = pulumirpc.DiffResponse_DIFF_SOME
)

// TODO: cleanup DiffResponce and remove all rpc types
type DiffResponce struct {
	Replaces            []string
	Stables             []string
	DeleteBeforeReplace bool
	Changes             DiffResponceChangeType
	Diffs               []string
	// detailedDiff is an optional field that contains map from each changed property to the type of the change.
	//
	// The keys of this map are property paths. These paths are essentially Javascript property access expressions
	// in which all elements are literals, and obey the following EBNF-ish grammar:
	//
	//   propertyName := [a-zA-Z_$] { [a-zA-Z0-9_$] }
	//   quotedPropertyName := '"' ( '\' '"' | [^"] ) { ( '\' '"' | [^"] ) } '"'
	//   arrayIndex := { [0-9] }
	//
	//   propertyIndex := '[' ( quotedPropertyName | arrayIndex ) ']'
	//   rootProperty := ( propertyName | propertyIndex )
	//   propertyAccessor := ( ( '.' propertyName ) |  propertyIndex )
	//   path := rootProperty { propertyAccessor }
	//
	// Examples of valid keys:
	// - root
	// - root.nested
	// - root["nested"]
	// - root.double.nest
	// - root["double"].nest
	// - root["double"]["nest"]
	// - root.array[0]
	// - root.array[100]
	// - root.array[0].nested
	// - root.array[0][1].nested
	// - root.nested.array[0].double[1]
	// - root["key with \"escaped\" quotes"]
	// - root["key with a ."]
	// - ["root key with \"escaped\" quotes"].nested
	// - ["root key with a ."][100]
	DetailedDiff    map[string]*pulumirpc.PropertyDiff
	HasDetailedDiff bool
}

type ResourceCheck interface {
	Check(ctx context.Context, news interface{}, sequenceNumber int32) ([]CheckFailure, error)
}

type CheckFailure struct {
	Property string // the property that failed validation.
	Reason   string // the reason that the property failed validation.
}

type ResourceRead interface {
	Read(ctx context.Context) error
}

type Component interface {
	Construct(ctx *pulumi.Context) error
}

func Resources(resources ...Resource) Options {
	return func(o *options) {
		o.Resources = append(o.Resources, resources...)
	}
}

func Types(types ...interface{}) Options {
	return func(o *options) {
		o.Types = append(o.Types, types...)
	}
}

func Components(components ...Component) Options {
	return func(o *options) {
		o.Components = append(o.Components, components...)
	}
}

// TODO: Add Invokes
