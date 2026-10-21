// Copyright 2026, Pulumi Corporation.
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

package tests

import (
	"context"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/integration"
)

// RawRequestRes records the raw request each of its methods sees.
type RawRequestRes struct{ seen *rawRequestsSeen }

type RawRequestArgs struct {
	Value string `pulumi:"value"`
}

type RawRequestState struct{ RawRequestArgs }

type rawRequestsSeen struct {
	check  p.CheckRequest
	diff   p.DiffRequest
	create p.CreateRequest
	read   p.ReadRequest
	update p.UpdateRequest
	delete p.DeleteRequest

	// otherTypeFound is set if a method finds a raw request of a type it isn't serving.
	otherTypeFound bool
}

func (r *RawRequestRes) Check(
	ctx context.Context, req infer.CheckRequest,
) (infer.CheckResponse[RawRequestArgs], error) {
	r.seen.check = mustGetRawRequest[p.CheckRequest](ctx)
	_, r.seen.otherTypeFound = infer.GetRawRequest[p.CreateRequest](ctx)
	args, failures, err := infer.DefaultCheck[RawRequestArgs](ctx, req.NewInputs)
	return infer.CheckResponse[RawRequestArgs]{Inputs: args, Failures: failures}, err
}

func (r *RawRequestRes) Diff(
	ctx context.Context, _ infer.DiffRequest[RawRequestArgs, RawRequestState],
) (infer.DiffResponse, error) {
	r.seen.diff = mustGetRawRequest[p.DiffRequest](ctx)
	return infer.DiffResponse{}, nil
}

func (r *RawRequestRes) Create(
	ctx context.Context, req infer.CreateRequest[RawRequestArgs],
) (infer.CreateResponse[RawRequestState], error) {
	r.seen.create = mustGetRawRequest[p.CreateRequest](ctx)
	return infer.CreateResponse[RawRequestState]{ID: "id", Output: RawRequestState{req.Inputs}}, nil
}

func (r *RawRequestRes) Read(
	ctx context.Context, req infer.ReadRequest[RawRequestArgs, RawRequestState],
) (infer.ReadResponse[RawRequestArgs, RawRequestState], error) {
	r.seen.read = mustGetRawRequest[p.ReadRequest](ctx)
	return infer.ReadResponse[RawRequestArgs, RawRequestState](req), nil
}

func (r *RawRequestRes) Update(
	ctx context.Context, req infer.UpdateRequest[RawRequestArgs, RawRequestState],
) (infer.UpdateResponse[RawRequestState], error) {
	r.seen.update = mustGetRawRequest[p.UpdateRequest](ctx)
	return infer.UpdateResponse[RawRequestState]{Output: RawRequestState{req.Inputs}}, nil
}

func (r *RawRequestRes) Delete(
	ctx context.Context, _ infer.DeleteRequest[RawRequestState],
) (infer.DeleteResponse, error) {
	r.seen.delete = mustGetRawRequest[p.DeleteRequest](ctx)
	return infer.DeleteResponse{}, nil
}

func mustGetRawRequest[T infer.RawResourceRequest](ctx context.Context) T {
	req, ok := infer.GetRawRequest[T](ctx)
	if !ok {
		panic("no raw request in context")
	}
	return req
}

func TestGetRawRequest(t *testing.T) {
	t.Parallel()

	seen := &rawRequestsSeen{}
	prov := infer.Provider(infer.Options{
		Resources: []infer.InferredResource{infer.Resource(&RawRequestRes{seen: seen})},
		ModuleMap: map[tokens.ModuleName]tokens.ModuleName{"tests": "index"},
	})
	s, err := integration.NewServer(t.Context(),
		"test", semver.MustParse("1.0.0"), integration.WithProvider(prov))
	require.NoError(t, err)

	resourceURN := urn("RawRequestRes", "test")
	// The typed inputs can't say that value is secret. The raw request can.
	olds := property.NewMap(map[string]property.Value{"value": property.New("old")})
	news := property.NewMap(map[string]property.Value{"value": property.New("new").WithSecret(true)})

	_, err = s.Check(p.CheckRequest{Urn: resourceURN, State: olds, Inputs: news})
	require.NoError(t, err)
	assert.Equal(t, news, seen.check.Inputs)
	assert.Equal(t, olds, seen.check.State)
	assert.False(t, seen.otherTypeFound, "a Check finds only a p.CheckRequest")

	_, err = s.Diff(p.DiffRequest{Urn: resourceURN, ID: "id", State: olds, OldInputs: olds, Inputs: news})
	require.NoError(t, err)
	assert.Equal(t, olds, seen.diff.OldInputs)
	assert.Equal(t, news, seen.diff.Inputs)

	_, err = s.Create(p.CreateRequest{Urn: resourceURN, Properties: news})
	require.NoError(t, err)
	assert.True(t, seen.create.Properties.Get("value").Secret())

	_, err = s.Read(p.ReadRequest{Urn: resourceURN, ID: "id", Properties: news, Inputs: news})
	require.NoError(t, err)
	assert.Equal(t, "id", seen.read.ID)
	assert.Equal(t, news, seen.read.Inputs)

	_, err = s.Update(p.UpdateRequest{Urn: resourceURN, ID: "id", State: olds, OldInputs: olds, Inputs: news})
	require.NoError(t, err)
	assert.Equal(t, olds, seen.update.OldInputs)
	assert.True(t, seen.update.Inputs.Get("value").Secret())

	err = s.Delete(p.DeleteRequest{Urn: resourceURN, ID: "id", Properties: news})
	require.NoError(t, err)
	assert.Equal(t, "id", seen.delete.ID)

	_, ok := infer.GetRawRequest[p.CreateRequest](t.Context())
	assert.False(t, ok, "a context that isn't from a request has no raw request")
}
