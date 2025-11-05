// Copyright 2025, Pulumi Corporation.
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

package rpc

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	structpb "github.com/golang/protobuf/ptypes/struct"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDebugMiddleware verifies that the debug middleware logs RPC calls to a file
func TestDebugMiddleware(t *testing.T) {
	// Create a temporary file for debug logs
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "debug.log")

	// Set environment variable to enable debug logging
	t.Setenv("PULUMI_GO_PROVIDER_DEBUG_GRPC", logFile)

	// Set up mock server
	mock := &mockResourceProviderServer{}

	// Create provider with debug enabled via environment variable
	provider := Provider(mock)

	// Make a CheckConfig call
	mock.cannedResp = &pulumirpc.CheckResponse{
		Inputs: &structpb.Struct{Fields: map[string]*structpb.Value{
			"apiKey": {Kind: &structpb.Value_StringValue{StringValue: "test-key"}},
		}},
	}
	_, err := provider.CheckConfig(t.Context(), p.CheckRequest{
		Urn:    resource.URN("urn:pulumi:stack::project::pulumi:providers:test"),
		Inputs: property.NewMap(map[string]property.Value{"apiKey": property.New("test-key")}),
	})
	require.NoError(t, err)

	// Make a Configure call
	mock.cannedResp = &pulumirpc.ConfigureResponse{
		AcceptSecrets:   true,
		AcceptResources: true,
	}
	err = provider.Configure(t.Context(), p.ConfigureRequest{
		Args: property.NewMap(map[string]property.Value{"apiKey": property.New("test-key")}),
	})
	require.NoError(t, err)

	// Make a Cancel call
	err = provider.Cancel(t.Context())
	require.NoError(t, err)

	// Read and verify the log file contains all three entries
	logData, err := os.ReadFile(logFile) //nolint:gosec
	require.NoError(t, err)

	// Parse each line as a separate JSON entry
	var entries []logEntry
	for line := range bytes.SplitSeq(logData, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var entry logEntry
		err = json.Unmarshal(line, &entry)
		require.NoError(t, err)
		entries = append(entries, entry)
	}

	require.Len(t, entries, 3)
	assert.Equal(t, "/pulumirpc.ResourceProvider/CheckConfig", entries[0].Method)
	assert.NotEmpty(t, entries[0].Request)
	assert.NotEmpty(t, entries[0].Response)
	assert.Empty(t, entries[0].Error)

	assert.Equal(t, "/pulumirpc.ResourceProvider/Configure", entries[1].Method)
	assert.NotEmpty(t, entries[1].Request)
	assert.NotEmpty(t, entries[1].Response)
	assert.Empty(t, entries[1].Error)

	assert.Equal(t, "/pulumirpc.ResourceProvider/Cancel", entries[2].Method)
	assert.NotEmpty(t, entries[2].Request)
	assert.NotEmpty(t, entries[2].Response)
	assert.Empty(t, entries[2].Error)
}
