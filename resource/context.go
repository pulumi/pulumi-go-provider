// Copyright 2022, Pulumi Corporation.
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

package resource

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi-go-provider/internal/introspect"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type Context interface {
	context.Context

	// MarkComputed marks a resource field as computed during a preview. Marking a field
	// as computed indicates to the engine that the field will be computed during the
	// update but the result is not known during the preview.
	//
	// MarkComputed may only be called on a direct reference to a field of the resource
	// whose method Context was passed to. Calling it on another value panics.
	//
	// For example:
	// ```go
	// func (r *MyResource) Update(ctx resource.Context, _ string, newSalt any, _ []string, preview bool) error {
	//     new := newSalt.(*RandomSalt)
	//     if new.FieldInput != r.FieldInput {
	//         ctx.MarkComputed(&r.ComputedField)        // This is valid
	//         // ctx.MarkComputed(r.ComputedField)      // This is *not* valid
	//         // ctx.markedComputed(&new.ComputedField) // Neither is this
	//         if !preview {
	//             r.ComputedField = expensiveComputation(r.FieldInput)
	//         }
	//     }
	//     return nil
	// }
	// ```
	MarkComputed(field any)

	// Log logs a global message, including errors and warnings.
	Log(severity diag.Severity, msg string, args ...any) error

	// LogStatus logs a global status message, including errors and warnings. Status messages will
	// appear in the `Info` column of the progress display, but not in the final output.
	LogStatus(severity diag.Severity, msg string, args ...any) error
}

type SContext struct {
	context.Context

	matcher introspect.FieldMatcher

	// fields of the underlying type that should be marked unknown
	markedComputed []string
	urn            resource.URN
	host           *provider.HostClient
}

// See the method documentation for Context.MarkComputed.
func (c *SContext) MarkComputed(field any) {
	tag, ok, err := c.matcher.GetField(field)
	if err != nil {
		panic(fmt.Sprintf("failed to mark as computed: %s", err.Error()))
	}

	if !ok {
		panic("Cannot mark value as computed, since it is not a field reference")
	}
	c.markedComputed = append(c.markedComputed, tag.Name)
}

// Log logs a global message, including errors and warnings.
func (c *SContext) Log(severity diag.Severity, msg string, args ...any) error {
	return c.host.Log(c.Context, severity, c.urn, fmt.Sprintf(msg, args...))
}

// LogStatus logs a global status message, including errors and warnings. Status messages will
// appear in the `Info` column of the progress display, but not in the final output.
func (c *SContext) LogStatus(severity diag.Severity, msg string, args ...any) error {
	return c.host.LogStatus(c.Context, severity, c.urn, fmt.Sprintf(msg, args...))
}

func NewContext(ctx context.Context, host *provider.HostClient,
	urn resource.URN, matcher introspect.FieldMatcher) *SContext {
	contract.Assert(host != nil)

	return &SContext{
		Context: ctx,
		matcher: matcher,
		urn:     urn,
		host:    host,
	}
}

func (c *SContext) ComputedKeys() []string {
	return c.markedComputed
}
