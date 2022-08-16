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
	wg := new(sync.WaitGroup)
	wg.Add(3)
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
