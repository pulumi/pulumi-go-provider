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

package grpc

import (
	"context"
	"fmt"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/property"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
)

func TestReadEmptyResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		resp           p.ReadResponse
		err            error
		wantProperties bool
	}{
		{
			name:           "empty ID and no properties sends nil properties",
			resp:           p.ReadResponse{},
			wantProperties: false,
		},
		{
			name: "empty ID with properties sends nil properties",
			resp: p.ReadResponse{
				Properties: property.NewMap(map[string]property.Value{"k": property.New("v")}),
				Inputs:     property.NewMap(map[string]property.Value{"k": property.New("v")}),
			},
			wantProperties: false,
		},
		{
			name:           "ErrNotFound sends an empty response",
			resp:           p.ReadResponse{ID: "id"},
			err:            p.ErrNotFound,
			wantProperties: false,
		},
		{
			name:           "wrapped ErrNotFound sends an empty response",
			resp:           p.ReadResponse{ID: "id"},
			err:            fmt.Errorf("get id: %w", p.ErrNotFound),
			wantProperties: false,
		},
		{
			name:           "non-empty ID with no properties still sends properties",
			resp:           p.ReadResponse{ID: "id"},
			wantProperties: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s, err := p.RawServer("test", "1.0.0", p.Provider{
				Read: func(context.Context, p.ReadRequest) (p.ReadResponse, error) {
					return tt.resp, tt.err
				},
			})(nil)
			require.NoError(t, err)

			resp, err := s.Read(context.Background(), &pulumirpc.ReadRequest{
				Id:  "requested-id",
				Urn: "urn:pulumi:dev::dev::test:index:Res::r",
			})
			require.NoError(t, err)
			wantID := tt.resp.ID
			if tt.err != nil {
				wantID = ""
			}
			assert.Equal(t, wantID, resp.GetId())
			assert.Equal(t, tt.wantProperties, resp.GetProperties() != nil, "properties presence")
			assert.Equal(t, tt.wantProperties, resp.GetInputs() != nil, "inputs presence")
		})
	}
}
