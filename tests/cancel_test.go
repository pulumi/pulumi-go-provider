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
	"sync"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi-go-provider/middleware/cancel"
)

func TestGlobalCancel(t *testing.T) {
	t.Parallel()

	const testSize = 5000
	require.True(t, testSize%2 == 0)

	noWaitCounter := new(sync.WaitGroup)
	noWaitCounter.Add(testSize / 2)

	provider := integration.NewServer("cancel", semver.MustParse("1.2.3"),
		cancel.Wrap(p.Provider{
			Create: func(ctx context.Context, req p.CreateRequest) (p.CreateResponse, error) {

				// If a request is set to wait, then it pauses until it is canceled.
				if req.Properties.Get("wait").AsBool() {
					<-ctx.Done()

					return p.CreateResponse{}, ctx.Err()
				}

				noWaitCounter.Done()

				return p.CreateResponse{}, nil
			},
		}))

	finished := new(sync.WaitGroup)
	finished.Add(testSize + (testSize / 2))

	go func() {
		// Make sure that all requests that should not be canceled have already gone through.
		noWaitCounter.Wait()

		// Now cancel remaining requests.
		err := provider.Cancel()
		assert.NoError(t, err)

		// As a sanity check, send another testSize/2 requests. Check that they are immediately
		// canceled.
		for i := 0; i < testSize/2; i++ {
			go func() {
				_, err := provider.Create(p.CreateRequest{
					Properties: property.NewMap(map[string]property.Value{
						"wait": property.New(true),
					}),
				})
				assert.ErrorIs(t, err, context.Canceled)
				finished.Done()
			}()
		}
	}()

	// create testSize requests.
	//
	// Half are configured to wait, while the other half are set to return immediately.
	for i := 0; i < testSize; i++ {
		shouldWait := i%2 == 0
		go func() {
			_, err := provider.Create(p.CreateRequest{
				Properties: property.NewMap(map[string]property.Value{
					"wait": property.New(shouldWait),
				}),
			})
			if shouldWait {
				assert.ErrorIs(t, err, context.Canceled)
			} else {
				assert.NoError(t, err)
			}
			finished.Done()
		}()
	}
	finished.Wait()
}

// TestCancelCreate checks that a Cancel that occurs during a concurrent operation
// (Create) cancels the context associated with the operation.
func TestCancelCreate(t *testing.T) {
	t.Parallel()

	createCheck := make(chan bool)

	provider := integration.NewServer("cancel", semver.MustParse("1.2.3"), cancel.Wrap(p.Provider{
		Create: func(ctx context.Context, req p.CreateRequest) (p.CreateResponse, error) {
			// The context should not be canceled yes
			assert.NoError(t, ctx.Err())
			createCheck <- true
			<-createCheck

			return p.CreateResponse{}, ctx.Err()
		},
	}))

	go func() {
		<-createCheck
		assert.NoError(t, provider.Cancel())
		createCheck <- true
	}()

	_, err := provider.Create(p.CreateRequest{})
	assert.ErrorIs(t, err, context.Canceled)
}

// TestCancelTimeout checks that timeouts are applied.
//
// Note: if the timeout is not applied, the test will hang instead of fail.
func TestCancelTimeout(t *testing.T) {
	t.Parallel()

	checkDeadline := func(ctx context.Context) error {
		_, ok := ctx.Deadline()
		assert.True(t, ok)
		<-ctx.Done()
		return ctx.Err()
	}

	s := integration.NewServer("cancel", semver.MustParse("1.2.3"),
		cancel.Wrap(p.Provider{
			Create: func(ctx context.Context, _ p.CreateRequest) (p.CreateResponse, error) {
				return p.CreateResponse{}, checkDeadline(ctx)
			},
			Update: func(ctx context.Context, _ p.UpdateRequest) (p.UpdateResponse, error) {
				return p.UpdateResponse{}, checkDeadline(ctx)
			},
			Delete: func(ctx context.Context, _ p.DeleteRequest) error {
				return checkDeadline(ctx)
			},
		}))

	t.Run("create", func(t *testing.T) {
		t.Parallel()
		_, err := s.Create(p.CreateRequest{
			Timeout: 0.1,
		})
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("update", func(t *testing.T) {
		t.Parallel()
		_, err := s.Update(p.UpdateRequest{
			Timeout: 0.1,
		})
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()
		err := s.Delete(p.DeleteRequest{
			Timeout: 0.1,
		})
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})
}

// Test that a `Cancel` call will cancel in-flight operations, even if they already have a
// timeout associated with them.
//
//	Main routine                   Server routine                 Cancel routine
//	 |                              |                              |
//	 | `s.Create`-----------------> |                              |
//	 |                              | `hasCreated <- true` ------> |
//	 |                              |                              |
//	 |                              | <---------------------------- `s.Cancel`
//	 |                              |                              |
//	 |                              | `<-ctx.Done()`               |
//	 |                              |                              |
//	 | <---------------------------- `Error: context.Canceled`     |
//	 |                              |                              |
func TestCancelCreateWithTimeout(t *testing.T) {
	t.Parallel()

	// Used to block until the create call has started.
	//
	// We use this because we want to ensure that our request is in-flight when it is
	// canceled.
	hasCreated := make(chan bool)

	s := integration.NewServer("cancel", semver.MustParse("1.2.3"), cancel.Wrap(p.Provider{
		Create: func(ctx context.Context, _ p.CreateRequest) (p.CreateResponse, error) {
			hasCreated <- true
			<-ctx.Done()
			return p.CreateResponse{}, ctx.Err()
		},
	}))

	go func() {
		<-hasCreated
		err := s.Cancel()
		require.NoError(t, err)
	}()

	_, err := s.Create(p.CreateRequest{
		Timeout: 1,
	})
	assert.ErrorIs(t, err, context.Canceled)
}
