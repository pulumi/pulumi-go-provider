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

package cancel_test

import (
	"sync"
	"testing"

	"github.com/blang/semver"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi-go-provider/middleware"
	"github.com/pulumi/pulumi-go-provider/middleware/cancel"
	"github.com/stretchr/testify/assert"
)

func TestGlobalCancel(t *testing.T) {
	t.Parallel()
	wg := new(sync.WaitGroup)
	wg.Add(4)
	s := integration.NewServer("cancel", semver.MustParse("1.2.3"),
		cancel.Wrap(&middleware.Scaffold{
			CreateFn: func(ctx p.Context, req p.CreateRequest) (p.CreateResponse, error) {
				select {
				case <-ctx.Done():
					wg.Done()
					return p.CreateResponse{
						ID:         "cancled",
						Properties: req.Properties,
					}, nil
				}
			},
		}))
	go func() { _, err := s.Create(p.CreateRequest{}); assert.NoError(t, err) }()
	go func() { _, err := s.Create(p.CreateRequest{}); assert.NoError(t, err) }()
	go func() { _, err := s.Create(p.CreateRequest{}); assert.NoError(t, err) }()
	assert.NoError(t, s.Cancel())
	go func() { _, err := s.Create(p.CreateRequest{}); assert.NoError(t, err) }()
	wg.Wait()
}

func TestTimeoutApplication(t *testing.T) {
	t.Parallel()
	wg := new(sync.WaitGroup)
	wg.Add(1)
	s := integration.NewServer("cancel", semver.MustParse("1.2.3"),
		cancel.Wrap(&middleware.Scaffold{
			CreateFn: func(ctx p.Context, req p.CreateRequest) (p.CreateResponse, error) {
				select {
				case <-ctx.Done():
					wg.Done()
					return p.CreateResponse{
						ID:         "cancled",
						Properties: req.Properties,
					}, nil
				}
			},
		}))

	go func() {
		_, err := s.Create(p.CreateRequest{
			Timeout: 0.5,
		})
		assert.NoError(t, err)
	}()
	wg.Wait()
}
