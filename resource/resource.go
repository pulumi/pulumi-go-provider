package resource

import (
	"context"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type Id = string

type Custom interface {
	// Create a resource.
	// Resource input properties will be applied to the resource the
	// method is called on. Output properties are set by manipulating the resource this
	// struct is called on.
	//
	// This means that implementing this method correctly requires passing the Resource
	// implementer by reference.
	//
	// Warning: Mutating the receiver asynchronously after Create has returned may lead to
	// invalid behavior.
	Create(ctx context.Context, preview bool) (Id, error)
	Delete(ctx context.Context, id Id) error
}

type ResourceUpdate interface {
	Update(ctx context.Context, id Id, new interface{}, ignoreChanges []string, preview bool) error
}

type ResourceDiff interface {
	Diff(ctx context.Context, id Id, new interface{}, ignoreChanges []string) (*pulumirpc.DiffResponse, error)
}

type ResourceCheck interface {
	Check(ctx context.Context, news interface{}, sequenceNumber int) ([]CheckFailure, error)
}

type CheckFailure struct {
	Property string // the property that failed validation.
	Reason   string // the reason that the property failed validation.
}

type ResourceRead interface {
	Read(ctx context.Context) error
}
