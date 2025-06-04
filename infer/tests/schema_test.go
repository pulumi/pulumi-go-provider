// Copyright 2025, Pulumi Corporation.
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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
)

func TestGetSchema(t *testing.T) {
	t.Parallel()

	prov := providerWithConfig(t, Config{})
	resp, err := prov.GetSchema(p.GetSchemaRequest{
		Version: 0,
	})
	require.NoError(t, err)

	assert.JSONEq(t, `
{
  "name": "test",
  "version": "1.0.0",
  "config": {
    "variables": {
      "value": {
        "type": "string",
        "description": "A value that is set in the provider config.",
        "deprecationMessage": "A deprecation message."
      }
    }
  },
  "types": {
    "test:index:NestedDefaults": {
      "properties": {
        "b": { "type": "boolean", "default": true },
        "f": { "type": "number", "default": 4 },
        "i": { "type": "integer", "default": 8 },
        "pb": { "type": "boolean", "default": true },
        "pf": { "type": "number", "default": 4 },
        "pi": { "type": "integer", "default": 8 },
        "pppi": { "type": "integer", "default": 64 },
        "ps": { "type": "string", "default": "two" },
        "s": { "type": "string", "default": "two" }
      },
      "type": "object"
    },
    "test:index:NoDefaults": {
      "properties": { "s": { "type": "string" } },
      "type": "object"
    },
    "test:index:OptWithReq": {
      "properties": {
        "empty": { "type": "string" },
        "opt": { "type": "string", "default": "default-value" },
        "req": { "type": "string" }
      },
      "type": "object",
      "required": ["req"]
    },
    "test:index:RecursiveArgs": {
      "properties": {
        "other": { "$ref": "#/types/test:index:RecursiveArgs" },
        "value": { "type": "string", "default": "default-value" }
      },
      "type": "object"
    }
  },
  "provider": {
    "description": "The provider configuration.",
    "properties": {
      "value": {
        "type": "string",
        "description": "A value that is set in the provider config.",
        "deprecationMessage": "A deprecation message."
      }
    },
    "inputProperties": {
      "value": {
        "type": "string",
        "description": "A value that is set in the provider config.",
        "deprecationMessage": "A deprecation message."
      }
    }
  },
  "resources": {
    "test:index:CustomCheckNoDefaults": {
      "properties": { "input": { "type": "string", "secret": true } },
      "required": ["input"],
      "inputProperties": { "input": { "type": "string", "secret": true } },
      "requiredInputs": ["input"]
    },
    "test:index:Echo": {
      "properties": {
        "int": { "type": "integer" },
        "intOut": { "type": "integer" },
        "nameOut": { "type": "string" },
        "strMap": {
          "type": "object",
          "additionalProperties": { "type": "string" }
        },
        "strMapOut": {
          "type": "object",
          "additionalProperties": { "type": "string" }
        },
        "string": { "type": "string" },
        "stringOut": { "type": "string" }
      },
      "required": ["string", "int", "nameOut", "stringOut", "intOut"],
      "inputProperties": {
        "int": { "type": "integer" },
        "strMap": {
          "type": "object",
          "additionalProperties": { "type": "string" }
        },
        "string": { "type": "string" }
      },
      "requiredInputs": ["string", "int"]
    },
    "test:index:Increment": {
      "properties": {
        "int": { "type": "integer" },
        "other": { "type": "integer" }
      },
      "required": ["int"],
      "inputProperties": {
        "int": { "type": "integer" },
        "other": { "type": "integer" }
      },
      "requiredInputs": ["int"]
    },
    "test:index:RandomComponent": {
      "properties": {
        "prefix": { "type": "string" },
        "result": { "type": "string" }
      },
      "required": ["prefix", "result"],
      "inputProperties": { "prefix": { "type": "string" } },
      "requiredInputs": ["prefix"],
      "isComponent": true
    },
    "test:index:ReadConfig": {
      "properties": { "config": { "type": "string" } },
      "required": ["config"]
    },
    "test:index:ReadConfigComponent": {
      "properties": { "config": { "type": "string" } },
      "required": ["config"],
      "isComponent": true
    },
    "test:index:ReadConfigCustom": {
      "properties": { "config": { "type": "string" } },
      "required": ["config"]
    },
    "test:index:ReadEnv": {
      "properties": {
        "b": { "type": "boolean", "defaultInfo": { "environment": ["BOOL"] } },
        "f64": {
          "type": "number",
          "defaultInfo": { "environment": ["FLOAT64"] }
        },
        "i": { "type": "integer", "defaultInfo": { "environment": ["INT"] } },
        "s": { "type": "string", "defaultInfo": { "environment": ["STRING"] } }
      },
      "inputProperties": {
        "b": { "type": "boolean", "defaultInfo": { "environment": ["BOOL"] } },
        "f64": {
          "type": "number",
          "defaultInfo": { "environment": ["FLOAT64"] }
        },
        "i": { "type": "integer", "defaultInfo": { "environment": ["INT"] } },
        "s": { "type": "string", "defaultInfo": { "environment": ["STRING"] } }
      }
    },
    "test:index:Recursive": {
      "properties": {
        "other": { "$ref": "#/types/test:index:RecursiveArgs" },
        "value": { "type": "string", "default": "default-value" }
      },
      "inputProperties": {
        "other": { "$ref": "#/types/test:index:RecursiveArgs" },
        "value": { "type": "string", "default": "default-value" }
      }
    },
    "test:index:Wired": {
      "properties": {
        "name": { "type": "string" },
        "stringAndInt": { "type": "string" },
        "stringPlus": { "type": "string" }
      },
      "required": ["name", "stringAndInt", "stringPlus"],
      "inputProperties": {
        "int": { "type": "integer" },
        "string": { "type": "string" }
      },
      "requiredInputs": ["string", "int"]
    },
    "test:index:WiredPlus": {
      "properties": {
        "int": { "type": "integer" },
        "name": { "type": "string" },
        "string": { "type": "string" },
        "stringAndInt": { "type": "string" },
        "stringPlus": { "type": "string" }
      },
      "required": ["string", "int", "name", "stringAndInt", "stringPlus"],
      "inputProperties": {
        "int": { "type": "integer" },
        "string": { "type": "string" }
      },
      "requiredInputs": ["string", "int"]
    },
    "test:index:WithDefaults": {
      "properties": {
        "arrNested": {
          "type": "array",
          "items": { "$ref": "#/types/test:index:NestedDefaults" }
        },
        "arrNestedPtr": {
          "type": "array",
          "items": { "$ref": "#/types/test:index:NestedDefaults" }
        },
        "mapNested": {
          "type": "object",
          "additionalProperties": {
            "$ref": "#/types/test:index:NestedDefaults"
          }
        },
        "mapNestedPtr": {
          "type": "object",
          "additionalProperties": {
            "$ref": "#/types/test:index:NestedDefaults"
          }
        },
        "nested": { "$ref": "#/types/test:index:NestedDefaults" },
        "nestedPtr": { "$ref": "#/types/test:index:NestedDefaults" },
        "noDefaults": { "$ref": "#/types/test:index:NoDefaults" },
        "optWithReq": { "$ref": "#/types/test:index:OptWithReq" },
        "pi": { "type": "integer", "default": 2 },
        "s": { "type": "string", "default": "one" }
      },
      "required": ["nestedPtr"],
      "inputProperties": {
        "arrNested": {
          "type": "array",
          "items": { "$ref": "#/types/test:index:NestedDefaults" }
        },
        "arrNestedPtr": {
          "type": "array",
          "items": { "$ref": "#/types/test:index:NestedDefaults" }
        },
        "mapNested": {
          "type": "object",
          "additionalProperties": {
            "$ref": "#/types/test:index:NestedDefaults"
          }
        },
        "mapNestedPtr": {
          "type": "object",
          "additionalProperties": {
            "$ref": "#/types/test:index:NestedDefaults"
          }
        },
        "nested": { "$ref": "#/types/test:index:NestedDefaults" },
        "nestedPtr": { "$ref": "#/types/test:index:NestedDefaults" },
        "noDefaults": { "$ref": "#/types/test:index:NoDefaults" },
        "optWithReq": { "$ref": "#/types/test:index:OptWithReq" },
        "pi": { "type": "integer", "default": 2 },
        "s": { "type": "string", "default": "one" }
      },
      "requiredInputs": ["nestedPtr"]
    }
  },
  "functions": {
    "test:index:getJoin": {
      "inputs": {
        "properties": {
          "elems": { "type": "array", "items": { "type": "string" } },
          "sep": { "type": "string", "default": "," }
        },
        "type": "object",
        "required": ["elems"]
      },
      "outputs": {
        "properties": { "result": { "type": "string" } },
        "type": "object",
        "required": ["result"]
      }
    }
  }
}
`, resp.Schema)
}
