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

package tests

import (
	"context"
	"testing"

	"github.com/blang/semver"
	p "github.com/pulumi/pulumi-go-provider"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParameterize(t *testing.T) {
	t.Parallel()

	type args struct {
		input          *pulumirpc.ParameterizeRequest
		expectedInput  p.ParameterizeRequest
		inputError     string
		output         p.ParameterizeResponse
		expectedOutput *pulumirpc.ParameterizeResponse
	}

	run := func(args args) func(t *testing.T) {
		return func(t *testing.T) {
			t.Parallel()
			server, err := p.RawServer("test", "0.0.0-dev", p.Provider{
				Parameterize: func(_ context.Context, req p.ParameterizeRequest) (p.ParameterizeResponse, error) {
					assert.Equal(t, args.expectedInput, req)
					return args.output, nil
				},
			})(nil)
			require.NoError(t, err)

			resp, err := server.Parameterize(context.Background(), args.input)
			if args.inputError == "" {
				require.NoError(t, err)
				assert.Equal(t, args.expectedOutput, resp)
			} else {
				assert.ErrorContains(t, err, args.inputError)
			}
		}
	}

	t.Run("cli args", run(args{
		input: &pulumirpc.ParameterizeRequest{
			Parameters: &pulumirpc.ParameterizeRequest_Args{
				Args: &pulumirpc.ParameterizeRequest_ParametersArgs{
					Args: []string{"arg1", "arg2"},
				},
			},
		},
		expectedInput: p.ParameterizeRequest{Args: &p.ParameterizeRequestArgs{
			Args: []string{"arg1", "arg2"},
		}}, output: p.ParameterizeResponse{
			Name:    "my-name",
			Version: semver.MustParse("1.2.3"),
		}, expectedOutput: &pulumirpc.ParameterizeResponse{
			Name:    "my-name",
			Version: "1.2.3",
		},
	}))

	t.Run("re-parameterization", run(args{
		input: &pulumirpc.ParameterizeRequest{
			Parameters: &pulumirpc.ParameterizeRequest_Value{
				Value: &pulumirpc.ParameterizeRequest_ParametersValue{
					Name:    "my-name",
					Version: "1.2.3",
					Value:   []byte("byte-slice"),
				},
			},
		},
		expectedInput: p.ParameterizeRequest{Value: &p.ParameterizeRequestValue{
			Name:    "my-name",
			Version: semver.MustParse("1.2.3"),
			Value:   []byte("byte-slice"),
		}},
		output: p.ParameterizeResponse{
			Name:    "my-name",
			Version: semver.MustParse("1.2.3"),
		}, expectedOutput: &pulumirpc.ParameterizeResponse{
			Name:    "my-name",
			Version: "1.2.3",
		},
	}))

	t.Run("err-invalid-version", run(args{
		input: &pulumirpc.ParameterizeRequest{
			Parameters: &pulumirpc.ParameterizeRequest_Value{
				Value: &pulumirpc.ParameterizeRequest_ParametersValue{
					Name:    "my-name",
					Version: "not-a-version",
					Value:   []byte("byte-slice"),
				},
			},
		},
		inputError: `invalid version "not-a-version"`,
	}))
}
