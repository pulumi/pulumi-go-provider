package resource

import "github.com/pulumi/pulumi/sdk/v3/go/pulumi"

type Component interface {
	pulumi.ComponentResource
	Construct(name string, ctx *pulumi.Context) error
}
