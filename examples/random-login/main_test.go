package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/blang/semver"
	p "github.com/pulumi/pulumi-go-provider"
	integration "github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const schema = `{
  "name": "random-login",
  "version": "0.1.0",
  "language": {
    "go": {
      "importBasePath": "github.com/pulumi/pulumi-go-provider/examples/random-login/sdk/go/randomlogin"
    }
  },
  "config": {
    "variables": {
      "itsasecret": {
        "type": "boolean"
      }
    }
  },
  "provider": {
    "properties": {
      "itsasecret": {
        "type": "boolean"
      }
    },
    "inputProperties": {
      "itsasecret": {
        "type": "boolean"
      }
    }
  },
  "resources": {
    "random-login:index:MoreRandomPassword": {
      "description": "Generate a random password.",
      "properties": {
        "length": {
          "type": "integer"
        },
        "password": {
          "type": "string"
        }
      },
      "required": [
        "length",
        "password"
      ],
      "inputProperties": {
        "length": {
          "type": "integer",
          "description": "The desired password length."
        }
      },
      "requiredInputs": [
        "length"
      ],
      "isComponent": true
    },
    "random-login:index:RandomLogin": {
      "description": "Generate a random login.",
      "properties": {
        "password": {
          "type": "string",
          "description": "The generated password."
        },
        "petName": {
          "type": "boolean",
          "plain": true,
          "description": "Whether to use a memorable pet name or a random string for the Username."
        },
        "username": {
          "type": "string",
          "description": "The generated username."
        }
      },
      "required": [
        "petName",
        "username",
        "password"
      ],
      "inputProperties": {
        "petName": {
          "type": "boolean",
          "plain": true,
          "description": "Whether to use a memorable pet name or a random string for the Username."
        }
      },
      "requiredInputs": [
        "petName"
      ],
      "aliases": [
        {
          "type": "pkg:other:RandomLogin"
        }
      ],
      "isComponent": true
    },
    "random-login:index:RandomSalt": {
      "properties": {
        "password": {
          "type": "string"
        },
        "salt": {
          "type": "string"
        },
        "saltedLength": {
          "type": "integer"
        },
        "saltedPassword": {
          "type": "string"
        }
      },
      "required": [
        "salt",
        "saltedPassword",
        "password"
      ],
      "inputProperties": {
        "password": {
          "type": "string"
        },
        "saltedLength": {
          "type": "integer"
        }
      },
      "requiredInputs": [
        "password"
      ]
    }
  }
}`

func TestSchema(t *testing.T) {
	server, err := integration.NewServer(t.Context(),
		"random-login",
		semver.Version{Minor: 1},
		integration.WithProvider(provider()),
	)
	require.NoError(t, err)

	s, err := server.GetSchema(p.GetSchemaRequest{})
	require.NoError(t, err)
	blob := json.RawMessage{}
	err = json.Unmarshal([]byte(s.Schema), &blob)
	assert.NoError(t, err)
	encoded, err := json.MarshalIndent(blob, "", "  ")
	assert.NoError(t, err)
	assert.Equal(t, schema, string(encoded))
}

func TestRandomSalt(t *testing.T) {
	server, err := integration.NewServer(t.Context(),
		"random-login",
		semver.Version{Minor: 1},
		integration.WithProvider(provider()),
	)
	require.NoError(t, err)

	integration.LifeCycleTest{
		Resource: "random-login:index:RandomSalt",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"password":     property.New("foo"),
				"saltedLength": property.New(3.0),
			}),
			Hook: func(inputs, output property.Map) {
				t.Logf("Outputs: %v", output)
				saltedPassword := output.Get("saltedPassword").AsString()
				assert.True(t, strings.HasSuffix(saltedPassword, "foo"), "password wrong")
				assert.Len(t, saltedPassword, 6)
			},
		},
		Updates: []integration.Operation{
			{
				Inputs: property.NewMap(map[string]property.Value{
					"password":     property.New("bar"),
					"saltedLength": property.New(5.0),
				}),
				Hook: func(inputs, output property.Map) {
					saltedPassword := output.Get("saltedPassword").AsString()
					assert.True(t, strings.HasSuffix(saltedPassword, "bar"), "password wrong")
					assert.Len(t, saltedPassword, 8)
				},
			},
		},
	}.Run(t, server)
}

func TestRandomLogin(t *testing.T) {
	server, err := integration.NewServer(t.Context(),
		"random-login",
		semver.Version{Minor: 1},
		integration.WithProvider(provider()),
		integration.WithMocks(&integration.MockResourceMonitor{
			NewResourceF: func(args integration.MockResourceArgs) (string, property.Map, error) {
				// mock the registration of the component's resources
				switch {
				case args.TypeToken == "random:index/randomId:RandomId" && args.Name == "login-id":
					assert.Equal(t, 8.0, args.Inputs.Get("byteLength").AsNumber())
					return "user", property.Map{}, nil

				case args.TypeToken == "random:index/randomInteger:RandomInteger" && args.Name == "login-length":
					assert.Equal(t, 8.0, args.Inputs.Get("min").AsNumber())
					assert.Equal(t, 24.0, args.Inputs.Get("max").AsNumber())
					return args.Name, property.NewMap(map[string]property.Value{
						"result": property.New(12.0),
					}), nil

				case args.TypeToken == "random-login:index:MoreRandomPassword" && args.Name == "login-password":
					assert.Equal(t, 12.0, args.Inputs.Get("length").AsNumber())
					return args.Name, property.NewMap(map[string]property.Value{
						"password": property.New("12345").WithSecret(true),
					}), nil
				}
				return "", property.Map{}, nil
			},
		}),
	)
	require.NoError(t, err)

	// test the "random-login:RandomLogin" component
	resp, err := server.Construct(p.ConstructRequest{
		Urn: "urn:pulumi:stack::project::random-login:index:RandomLogin::login",
		Inputs: property.NewMap(map[string]property.Value{
			"petName": property.New(false),
		}),
	})
	require.NoError(t, err)

	require.Equal(t, property.NewMap(map[string]property.Value{
		"username": property.New("user"),
		"password": property.New("12345").WithSecret(true),
	}), resp.State)
}
