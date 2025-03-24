package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/blang/semver"
	p "github.com/pulumi/pulumi-go-provider"
	integration "github.com/pulumi/pulumi-go-provider/integration"
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const schema = `{
  "name": "random-login",
  "version": "0.1.0",
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
      "properties": {
        "length": {
          "$ref": "/random/v4.8.1/schema.json#/resources/random:index/randomInteger:RandomInteger"
        },
        "password": {
          "$ref": "/random/v4.8.1/schema.json#/resources/random:index/randomPassword:RandomPassword"
        }
      },
      "required": [
        "length",
        "password"
      ],
      "inputProperties": {
        "length": {
          "$ref": "/random/v4.8.1/schema.json#/resources/random:index/randomInteger:RandomInteger"
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
          "type": "string"
        },
        "passwordLength": {
          "type": "integer"
        },
        "petName": {
          "type": "boolean",
          "plain": true,
          "description": "Whether to use a memorable pet name or a random string for the Username."
        },
        "username": {
          "type": "string"
        }
      },
      "required": [
        "passwordLength",
        "petName",
        "username",
        "password"
      ],
      "inputProperties": {
        "passwordLength": {
          "type": "integer"
        },
        "petName": {
          "type": "boolean",
          "plain": true
        }
      },
      "requiredInputs": [
        "passwordLength",
        "petName"
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
	server := integration.NewServer("random-login", semver.Version{Minor: 1}, provider())
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
	server := integration.NewServer("random-login", semver.Version{Minor: 1}, provider())
	integration.LifeCycleTest{
		Resource: "random-login:index:RandomSalt",
		Create: integration.Operation{
			Inputs: presource.NewPropertyMapFromMap(map[string]interface{}{
				"password":     "foo",
				"saltedLength": 3,
			}),
			Hook: func(inputs, output presource.PropertyMap) {
				t.Logf("Outputs: %v", output)
				saltedPassword := output["saltedPassword"].StringValue()
				assert.True(t, strings.HasSuffix(saltedPassword, "foo"), "password wrong")
				assert.Len(t, saltedPassword, 6)
			},
		},
		Updates: []integration.Operation{
			{
				Inputs: presource.NewPropertyMapFromMap(map[string]interface{}{
					"password":     "bar",
					"saltedLength": 5,
				}),
				Hook: func(inputs, output presource.PropertyMap) {
					saltedPassword := output["saltedPassword"].StringValue()
					assert.True(t, strings.HasSuffix(saltedPassword, "bar"), "password wrong")
					assert.Len(t, saltedPassword, 8)
				},
			},
		},
	}.Run(t, server)
}
