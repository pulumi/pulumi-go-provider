package resource

import "github.com/pulumi/pulumi/sdk/v3/go/pulumi"

type Component interface {
	Construct(ctx *pulumi.Context) error
}
