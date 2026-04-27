// Copyright 2016-2025, Pulumi Corporation.
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
	"runtime/debug"
	"sync"
)

const (
	// frameworkStateKeyName is the key name used to store the framework version in
	// a program's state.
	frameworkStateKeyName = "__pulumi-go-provider-version"

	// frameworkModulePath is the Go module path of this framework. It is used to
	// look up the framework's resolved version in the consuming binary's build
	// info.
	frameworkModulePath = "github.com/pulumi/pulumi-go-provider"

	// devVersion is the placeholder reported when the Go module system has no
	// resolved version for the framework. This happens when the framework is
	// built as the main module (e.g. during its own tests) or when a consumer
	// uses a local `replace` directive.
	devVersion = "(devel)"
)

var (
	frameworkVersionOnce sync.Once
	frameworkVersionStr  string
)

// Version reports the resolved version of the pulumi-go-provider framework as
// recorded in the consuming binary's Go build information. For tagged releases
// it returns a value like "v1.2.3"; for untagged commits it returns a
// pseudo-version; for local development builds it returns "(devel)".
func Version() string {
	frameworkVersionOnce.Do(func() {
		frameworkVersionStr = readFrameworkVersion()
	})
	return frameworkVersionStr
}

func readFrameworkVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return devVersion
	}
	if info.Main.Path == frameworkModulePath {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
		return devVersion
	}
	for _, dep := range info.Deps {
		if dep.Path != frameworkModulePath {
			continue
		}
		if dep.Replace != nil && dep.Replace.Version != "" {
			return dep.Replace.Version
		}
		if dep.Version != "" {
			return dep.Version
		}
		return devVersion
	}
	return devVersion
}
