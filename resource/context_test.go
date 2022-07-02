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
	"testing"

	"github.com/iwahbe/pulumi-go-provider/internal/introspect"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/stretchr/testify/assert"
)

type FooResoruce struct {
	A string `pulumi:"A"`
	B *int
}

func TestMarkComputed(t *testing.T) {
	t.Parallel()
	f := &FooResoruce{}

	matcher := introspect.NewFieldMatcher(f)
	ctx := NewContext(context.Background(), &provider.HostClient{}, "urn", matcher)
	ctx.MarkComputed(&f.A)
	assert.Equal(t, []string{"A"}, ctx.markedComputed)
}
