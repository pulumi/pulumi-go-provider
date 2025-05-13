// This example shows using [github.com/pulumi/pulumi-go-provider] without any middleware.
//
// It defines one resource: `echo:index:Echo` with a single string value, which it repeats
// back as an output.
package main

import (
	"context"
	"fmt"
	"os"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func main() {

	echoType := tokens.NewTypeToken(tokens.NewModuleToken("echo", "index"), "Echo")

	err := p.RunProvider(context.Background(), "echo", "v0.1.0-dev", p.Provider{
		GetSchema: func(context.Context, p.GetSchemaRequest) (p.GetSchemaResponse, error) {
			// It is recommended to use [github.com/pulumi/pulumi-go-provider/middleware/schema.Wrap] for
			// structured schema generation.
			return p.GetSchemaResponse{Schema: `{
  "name": "echo",
  "version": "0.1.0-dev",
  "config": {},
  "provider": {
    "type": "object"
  },
  "resources": {
    "echo:index:Echo": {
      "properties": {
        "value": {
          "type": "string"
        }
      },
      "type": "object",
      "required": [
        "value"
      ],
      "inputProperties": {
        "value": {
          "type": "string"
        }
      }
    }
  }
}`}, nil
		},
		Cancel: func(context.Context) error {
			// Contexts will not be canceled here.
			//
			// For a correct Cancel implementation, please use
			// [github.com/pulumi/pulumi-go-provider/middleware/cancel.Wrap].
			return nil
		},
		CheckConfig: func(_ context.Context, req p.CheckRequest) (p.CheckResponse, error) {
			// Assume inputs are valid
			return p.CheckResponse{Inputs: req.Inputs}, nil
		},
		Configure: func(context.Context, p.ConfigureRequest) error {
			return nil
		},
		Check: func(_ context.Context, req p.CheckRequest) (p.CheckResponse, error) {
			if req.Urn.Type() != echoType {
				return p.CheckResponse{}, fmt.Errorf("unknown resource %q", req.Urn.Type())
			}
			return p.CheckResponse{
				// Only take "value" and ignore everything else.
				Inputs: property.NewMap(map[string]property.Value{
					"value": req.Inputs.Get("value"),
				}),
			}, nil
		},
		Diff: func(_ context.Context, req p.DiffRequest) (p.DiffResponse, error) {
			if req.Urn.Type() != echoType {
				return p.DiffResponse{}, fmt.Errorf("unknown resource %q", req.Urn.Type())
			}
			if req.Inputs.Get("value").Equals(req.State.Get("value")) {
				return p.DiffResponse{
					HasChanges: false,
				}, nil
			}
			return p.DiffResponse{
				DeleteBeforeReplace: true,
				HasChanges:          true,
				DetailedDiff: map[string]p.PropertyDiff{
					"value": {
						Kind:      p.Update,
						InputDiff: true,
					},
				},
			}, nil
		},
		Create: func(_ context.Context, req p.CreateRequest) (p.CreateResponse, error) {
			if req.Urn.Type() != echoType {
				return p.CreateResponse{}, fmt.Errorf("unknown resource %q", req.Urn.Type())
			}
			return p.CreateResponse{
				ID:         "id",
				Properties: req.Properties,
			}, nil
		},
		Read: func(_ context.Context, req p.ReadRequest) (p.ReadResponse, error) {
			if req.Urn.Type() != echoType {
				return p.ReadResponse{}, fmt.Errorf("unknown resource %q", req.Urn.Type())
			}
			// Read is a no-op.
			//
			// Leaving this unimplemented will break refresh.
			return p.ReadResponse{
				ID:         req.ID,
				Properties: req.Properties,
				Inputs:     req.Inputs,
			}, nil
		},
		Update: func(_ context.Context, req p.UpdateRequest) (p.UpdateResponse, error) {
			if req.Urn.Type() != echoType {
				return p.UpdateResponse{}, fmt.Errorf("unknown resource %q", req.Urn.Type())
			}
			// The provider assumes that all fields are valid, and just updates
			return p.UpdateResponse{Properties: req.Inputs}, nil
		},
		Delete: func(_ context.Context, req p.DeleteRequest) error {
			if req.Urn.Type() != echoType {
				return fmt.Errorf("unknown resource %q", req.Urn.Type())
			}
			return nil // Delete is a no-op for this provider
		},
	})
	if err != nil {
		os.Exit(1)
	}
}
