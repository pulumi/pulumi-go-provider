// Copyright 2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package rpc allows projecting a [rpc.ResourceProviderServer] into a [p.Provider].
//
// The entry point for this package is [Provider].
package rpc

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"

	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/pulumi/pulumi-go-provider/internal/key"
	"github.com/pulumi/pulumi-go-provider/internal/putil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/protobuf/types/known/emptypb"

	p "github.com/pulumi/pulumi-go-provider"
)

// Provider projects a [rpc.ResourceProviderServer] into a [p.Provider].
//
// It is intended that Provider is used to wrap legacy native provider implementations
// while they are gradually transferred over to pulumi-go-provider based implementations.
func Provider(server rpc.ResourceProviderServer) p.Provider {

	if logFile := os.Getenv("PULUMI_GO_PROVIDER_DEBUG_GRPC"); logFile != "" {
		var err error
		server, err = newDebugServer(server, logFile)
		if err != nil {
			panic(err)
		}
	}

	var runtime runtime // the runtime configuration of the server
	return p.Provider{
		GetSchema: func(ctx context.Context, req p.GetSchemaRequest) (p.GetSchemaResponse, error) {
			if req.Version > math.MaxInt32 {
				return p.GetSchemaResponse{}, fmt.Errorf("schema version overflow: %d", req.Version)
			}
			if req.Version < math.MinInt32 {
				return p.GetSchemaResponse{}, fmt.Errorf("schema version underflow: %d", req.Version)
			}
			s, err := server.GetSchema(ctx, &rpc.GetSchemaRequest{
				//cast validated above
				//
				//nolint:gosec
				Version: int32(req.Version),
			})
			return p.GetSchemaResponse{
				Schema: s.GetSchema(),
			}, err
		},
		Cancel: func(ctx context.Context) error {
			_, err := server.Cancel(ctx, &emptypb.Empty{})
			return err
		},
		CheckConfig: func(ctx context.Context, req p.CheckRequest) (p.CheckResponse, error) {
			olds, err := runtime.propertyToRPC(req.State)
			if err != nil {
				return p.CheckResponse{}, err
			}

			news, err := runtime.propertyToRPC(req.Inputs)
			if err != nil {
				return p.CheckResponse{}, err
			}

			return checkResponse(server.CheckConfig(ctx, &rpc.CheckRequest{
				Urn:        string(req.Urn),
				Olds:       olds,
				News:       news,
				RandomSeed: req.RandomSeed,
				Name:       req.Urn.Name(),
				Type:       req.Urn.Type().String(),
			}))
		},
		DiffConfig: func(ctx context.Context, req p.DiffRequest) (p.DiffResponse, error) {
			olds, err := runtime.propertyToRPC(req.State)
			if err != nil {
				return p.DiffResponse{}, err
			}
			news, err := runtime.propertyToRPC(req.Inputs)
			if err != nil {
				return p.DiffResponse{}, err
			}

			oldInputs, err := runtime.propertyToRPC(req.OldInputs)
			if err != nil {
				return p.DiffResponse{}, err
			}

			return diffResponse(server.DiffConfig(ctx, &rpc.DiffRequest{
				Id:            req.ID,
				Urn:           string(req.Urn),
				Olds:          olds,
				News:          news,
				IgnoreChanges: req.IgnoreChanges,
				OldInputs:     oldInputs,
				Name:          req.Urn.Name(),
				Type:          req.Urn.Type().String(),
			}))
		},
		Configure: func(ctx context.Context, req p.ConfigureRequest) error {
			args, err := runtime.propertyToRPC(req.Args)
			if err != nil {
				return err
			}

			runtime.configuration, err = server.Configure(ctx, &rpc.ConfigureRequest{
				Variables:       req.Variables,
				Args:            args,
				AcceptSecrets:   true,
				AcceptResources: true,
			})
			return err
		},
		Invoke: func(ctx context.Context, req p.InvokeRequest) (p.InvokeResponse, error) {
			args, err := runtime.propertyToRPC(req.Args)
			if err != nil {
				return p.InvokeResponse{}, err
			}

			resp, err := server.Invoke(ctx, &rpc.InvokeRequest{
				Tok:  string(req.Token),
				Args: args,
			})
			ret, err := rpcToProperty(resp.GetReturn(), err)
			return p.InvokeResponse{
				Return:   ret,
				Failures: checkFailures(resp.GetFailures()),
			}, err
		},
		Check: func(ctx context.Context, req p.CheckRequest) (p.CheckResponse, error) {
			olds, err := runtime.propertyToRPC(req.State)
			if err != nil {
				return p.CheckResponse{}, err
			}

			news, err := runtime.propertyToRPC(req.Inputs)
			if err != nil {
				return p.CheckResponse{}, err
			}

			return checkResponse(server.Check(ctx, &rpc.CheckRequest{
				Urn:        string(req.Urn),
				Olds:       olds,
				News:       news,
				RandomSeed: req.RandomSeed,
				Name:       req.Urn.Name(),
				Type:       req.Urn.Type().String(),
			}))
		},
		Diff: func(ctx context.Context, req p.DiffRequest) (p.DiffResponse, error) {
			olds, err := runtime.propertyToRPC(req.State)
			if err != nil {
				return p.DiffResponse{}, err
			}

			news, err := runtime.propertyToRPC(req.Inputs)
			if err != nil {
				return p.DiffResponse{}, err
			}

			oldInputs, err := runtime.propertyToRPC(req.OldInputs)
			if err != nil {
				return p.DiffResponse{}, err
			}

			return diffResponse(server.Diff(ctx, &rpc.DiffRequest{
				Id:            req.ID,
				Urn:           string(req.Urn),
				Olds:          olds,
				News:          news,
				IgnoreChanges: req.IgnoreChanges,
				OldInputs:     oldInputs,
				Name:          req.Urn.Name(),
				Type:          req.Urn.Type().String(),
			}))
		},
		Create: func(ctx context.Context, req p.CreateRequest) (p.CreateResponse, error) {
			if req.DryRun && runtime.configuration != nil && !runtime.configuration.SupportsPreview {
				return p.CreateResponse{}, nil
			}

			inProperties, err := runtime.propertyToRPC(req.Properties)
			if err != nil {
				return p.CreateResponse{}, err
			}

			resp, err := server.Create(ctx, &rpc.CreateRequest{
				Urn:        string(req.Urn),
				Properties: inProperties,
				Timeout:    req.Timeout,
				Preview:    req.DryRun,
				Name:       req.Urn.Name(),
				Type:       req.Urn.Type().String(),
			})
			properties, err := rpcToProperty(resp.GetProperties(), err)
			return p.CreateResponse{
				ID:         resp.GetId(),
				Properties: properties,
			}, err
		},
		Read: func(ctx context.Context, req p.ReadRequest) (p.ReadResponse, error) {
			inProperties, err := runtime.propertyToRPC(req.Properties)
			if err != nil {
				return p.ReadResponse{}, err
			}
			inInputs, err := runtime.propertyToRPC(req.Inputs)
			if err != nil {
				return p.ReadResponse{}, err
			}

			resp, err := server.Read(ctx, &rpc.ReadRequest{
				Id:         req.ID,
				Urn:        string(req.Urn),
				Properties: inProperties,
				Inputs:     inInputs,
				Name:       req.Urn.Name(),
				Type:       req.Urn.Type().String(),
			})
			properties, err := rpcToProperty(resp.GetProperties(), err)
			inputs, err := rpcToProperty(resp.GetInputs(), err)
			return p.ReadResponse{
				ID:         resp.GetId(),
				Properties: properties,
				Inputs:     inputs,
			}, err
		},
		Update: func(ctx context.Context, req p.UpdateRequest) (p.UpdateResponse, error) {
			if req.DryRun && runtime.configuration != nil && !runtime.configuration.SupportsPreview {
				return p.UpdateResponse{}, nil
			}

			inOlds, err := runtime.propertyToRPC(req.State)
			if err != nil {
				return p.UpdateResponse{}, err
			}

			inNews, err := runtime.propertyToRPC(req.Inputs)
			if err != nil {
				return p.UpdateResponse{}, err
			}
			oldInputs, err := runtime.propertyToRPC(req.OldInputs)
			if err != nil {
				return p.UpdateResponse{}, err
			}

			resp, err := server.Update(ctx, &rpc.UpdateRequest{
				Id:            req.ID,
				Urn:           string(req.Urn),
				Olds:          inOlds,
				News:          inNews,
				Timeout:       req.Timeout,
				IgnoreChanges: req.IgnoreChanges,
				Preview:       req.DryRun,
				Name:          req.Urn.Name(),
				Type:          req.Urn.Type().String(),
				OldInputs:     oldInputs,
			})

			properties, err := rpcToProperty(resp.GetProperties(), err)
			return p.UpdateResponse{
				Properties: properties,
			}, err
		},
		Delete: func(ctx context.Context, req p.DeleteRequest) error {
			properties, err := runtime.propertyToRPC(req.Properties)
			if err != nil {
				return err
			}
			oldInputs, err := runtime.propertyToRPC(req.OldInputs)
			if err != nil {
				return err
			}
			_, err = server.Delete(ctx, &rpc.DeleteRequest{
				Id:         req.ID,
				Urn:        string(req.Urn),
				Properties: properties,
				Timeout:    req.Timeout,
				Name:       req.Urn.Name(),
				Type:       req.Urn.Type().String(),
				OldInputs:  oldInputs,
			})
			return err
		},
		Construct: func(ctx context.Context, req p.ConstructRequest) (p.ConstructResponse, error) {
			if req.DryRun && runtime.configuration != nil && !runtime.configuration.SupportsPreview {
				return p.ConstructResponse{}, nil
			}

			rpcReq := linkedConstructRequestToRPC(&req, runtime.propertyToRPC)
			rpcReq.AcceptsOutputValues = true

			// downgrade the input dependencies if the provider doesn't support output values,
			// in which case the [runtime.propertyToRPC] function discarded the dependency information.
			inputDependencies := map[string]*rpc.ConstructRequest_PropertyDependencies{}
			if !runtime.configuration.AcceptOutputs {
				for name, v := range req.Inputs.All {
					urns := putil.GetPropertyDependencies(v)
					if len(urns) != 0 {
						inputDependencies[name] = &rpc.ConstructRequest_PropertyDependencies{Urns: putil.FromUrns(urns)}
					}
				}
			}
			rpcReq.InputDependencies = inputDependencies

			rpcResp, err := server.Construct(ctx, rpcReq)
			if err != nil {
				return p.ConstructResponse{}, err
			}

			resp, err := linkedConstructResponseFromRPC(rpcResp)
			if err != nil {
				return p.ConstructResponse{}, err
			}

			// upgrade the state dependencies if the provider doesn't support output values,
			// in which case [rpcResp.StateDependencies] has meaningful information.
			stateDeps := make(map[string][]urn.URN, len(rpcResp.GetStateDependencies()))
			for name, deps := range rpcResp.GetStateDependencies() {
				stateDeps[name] = putil.ToUrns(deps.GetUrns())
			}
			resp.State = putil.MergePropertyDependencies(resp.State, stateDeps)

			return resp, nil
		},
		Call: func(ctx context.Context, req p.CallRequest) (p.CallResponse, error) {
			if req.DryRun && runtime.configuration != nil && !runtime.configuration.SupportsPreview {
				return p.CallResponse{}, nil
			}

			rpcReq := linkedCallRequestToRPC(&req, runtime.propertyToRPC)

			// downgrade the arg dependencies if the provider doesn't support output values,
			// in which case the [runtime.propertyToRPC] function discarded the dependency information.
			argDependencies := map[string]*rpc.CallRequest_ArgumentDependencies{}
			if !runtime.configuration.AcceptOutputs {
				for name, v := range req.Args.All {
					urns := putil.GetPropertyDependencies(v)
					if len(urns) != 0 {
						argDependencies[name] = &rpc.CallRequest_ArgumentDependencies{Urns: putil.FromUrns(urns)}
					}
				}
			}
			rpcReq.ArgDependencies = argDependencies

			rpcResp, err := server.Call(ctx, rpcReq)
			if err != nil {
				return p.CallResponse{}, err
			}

			resp, err := linkedCallResponseFromRPC(rpcResp)
			if err != nil {
				return p.CallResponse{}, err
			}

			// upgrade the return dependencies if the provider doesn't support output values,
			// in which case [rpcResp.ReturnDependencies] has meaningful information.
			returnDeps := make(map[string][]urn.URN, len(rpcResp.GetReturnDependencies()))
			for name, deps := range rpcResp.GetReturnDependencies() {
				returnDeps[name] = putil.ToUrns(deps.GetUrns())
			}
			resp.Return = putil.MergePropertyDependencies(resp.Return, returnDeps)

			return resp, nil
		},
	}
}

func checkResponse(resp *rpc.CheckResponse, err error) (p.CheckResponse, error) {
	inputs, err := rpcToProperty(resp.GetInputs(), err)
	return p.CheckResponse{
		Inputs:   inputs,
		Failures: checkFailures(resp.GetFailures()),
	}, err
}

func diffResponse(resp *rpc.DiffResponse, err error) (p.DiffResponse, error) {
	detailedDiff := make(map[string]p.PropertyDiff, len(resp.GetDetailedDiff()))
	if resp.GetHasDetailedDiff() {
		for k, v := range resp.GetDetailedDiff() {
			var kind p.DiffKind
			switch v.Kind {
			case rpc.PropertyDiff_ADD:
				kind = p.Add
			case rpc.PropertyDiff_ADD_REPLACE:
				kind = p.AddReplace
			case rpc.PropertyDiff_DELETE:
				kind = p.Delete
			case rpc.PropertyDiff_DELETE_REPLACE:
				kind = p.DeleteReplace
			case rpc.PropertyDiff_UPDATE:
				kind = p.Update
			case rpc.PropertyDiff_UPDATE_REPLACE:
				kind = p.UpdateReplace
			}
			detailedDiff[k] = p.PropertyDiff{
				Kind:      kind,
				InputDiff: v.InputDiff,
			}
		}
	} else {
		// We need to emulate support for a non-detailed diff

		for _, update := range resp.GetDiffs() {
			detailedDiff[update] = p.PropertyDiff{Kind: p.Update}
		}
		for _, replace := range resp.GetReplaces() {
			detailedDiff[replace] = p.PropertyDiff{Kind: p.UpdateReplace}
		}
		detailedDiff[key.ForceNoDetailedDiff] = p.PropertyDiff{}

	}
	if len(detailedDiff) == 0 {
		detailedDiff = nil
	}
	return p.DiffResponse{
		DeleteBeforeReplace: resp.GetDeleteBeforeReplace(),
		HasChanges:          resp.GetChanges() == rpc.DiffResponse_DIFF_SOME,
		DetailedDiff:        detailedDiff,
	}, err
}

func checkFailures(resp []*rpc.CheckFailure) []p.CheckFailure {
	if resp == nil {
		return nil
	}
	arr := make([]p.CheckFailure, len(resp))
	for i, v := range resp {
		arr[i] = p.CheckFailure{
			Property: v.Property,
			Reason:   v.Reason,
		}
	}
	return arr
}

type runtime struct {
	configuration *rpc.ConfigureResponse
}

func (r runtime) propertyToRPC(m property.Map) (*structpb.Struct, error) {
	if r.configuration == nil {
		r.configuration = &rpc.ConfigureResponse{}
	}
	rm := resource.ToResourcePropertyValue(property.New(m)).ObjectValue()
	s, err := plugin.MarshalProperties(rm, plugin.MarshalOptions{
		KeepUnknowns:     true,
		KeepSecrets:      r.configuration.AcceptSecrets,
		KeepResources:    r.configuration.AcceptResources,
		KeepOutputValues: r.configuration.AcceptOutputs,
	})
	return s, err
}

func rpcToProperty(s *structpb.Struct, previousError error) (property.Map, error) {
	if s == nil {
		return property.Map{}, previousError
	}
	m, err := plugin.UnmarshalProperties(s, plugin.MarshalOptions{
		SkipNulls:        false,
		KeepUnknowns:     true,
		KeepSecrets:      true,
		KeepResources:    true,
		KeepOutputValues: true,
	})
	return resource.FromResourcePropertyValue(resource.NewProperty(m)).AsMap(), errors.Join(err, previousError)
}
