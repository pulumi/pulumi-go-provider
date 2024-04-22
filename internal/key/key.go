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

// key provides an internal set of keys for use with [context.WithValue] and
// [context.Context.Value] that can be shared across packages source.
//
// Each key has a private type (a `struct{}`) and a public instance of that type.
package key

type (
	runtimeInfoType struct{}
	logType         struct{}
	urnType         struct{}
)

var (
	RuntimeInfo = runtimeInfoType{}
	Logger      = logType{}
	URN         = urnType{}
)
