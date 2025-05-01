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

package provider

import (
	_ "unsafe" // unsafe is needed to use go:linkname

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// We want to make low-level rpc functionality available to the middleware for implementation purposes.
// To achieve this, go:linkname is used by various packages to link to the below function(s).

//nolint:unused
func linkedConstructRequestToRPC(req *ConstructRequest, marshal propertyToRPC) *pulumirpc.ConstructRequest {
	return req.rpc(marshal)
}

//nolint:unused
func linkedConstructResponseFromRPC(resp *pulumirpc.ConstructResponse) (ConstructResponse, error) {
	return newConstructResponse(resp)
}

//nolint:unused
func linkedCallRequestToRPC(req *CallRequest, marshal propertyToRPC) *pulumirpc.CallRequest {
	return req.rpc(marshal)
}

//nolint:unused
func linkedCallResponseFromRPC(resp *pulumirpc.CallResponse) (CallResponse, error) {
	return newCallResponse(resp)
}
