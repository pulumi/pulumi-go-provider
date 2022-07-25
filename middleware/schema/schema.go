package schema

import (
	"fmt"

	p "github.com/iwahbe/pulumi-go-provider"
	t "github.com/iwahbe/pulumi-go-provider/middleware"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type Resource interface {
	GetSchema() (schema.ResourceSpec, error)
	GetToken() (tokens.Type, error)
}

type Provider struct {
	p.Provider

	resources []Resource
	schema    string
}

func Wrap(provider p.Provider) *Provider {
	if provider == nil {
		provider = &t.Scaffold{}
	}
	return &Provider{}
}

func (s *Provider) WithResources(resources ...Resource) *Provider {
	s.schema = ""
	s.resources = append(s.resources, resources...)
	return s
}

func (s *Provider) GetSchema(ctx p.Context, req p.GetSchemaRequest) (p.GetSchemaResponse, error) {
	if s.schema == "" {
		err := s.generateSchema()
		if err != nil {
			return p.GetSchemaResponse{}, err
		}
	}
	return p.GetSchemaResponse{
		Schema: s.schema,
	}, nil
}

// Generate a schema string from the currently present schema types.
func (s *Provider) generateSchema() error {
	return nil
}

// A helper function to create a token for the current module.
func MakeToken(module, name string) string {
	return fmt.Sprintf("derived-pkg:%s:%s", module, name)
}
