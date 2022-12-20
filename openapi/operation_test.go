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

package openapi

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/blang/semver"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	s "github.com/pulumi/pulumi-go-provider/middleware/schema"
)

const OpenAPISchemaBytes = `{
  "openapi": "3.0.1",
  "info": {
    "title": "Todos API",
    "description": "Todo Backend API",
    "version": "1.0.0"
  },
  "servers": [
    {
      "url": "https://functodobackend.azurewebsites.net/api"
    }
  ],
  "paths": {
    "/todos": {
      "post": {
        "summary": "Create a new todo",
        "operationId": "Todo_Create",
        "requestBody": {
          "description": "Todo Object",
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/Todo"
              }
            }
          },
          "required": true
        },
        "responses": {
          "200": {
            "description": "successful operation",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/Todo"
                }
              }
            }
          },
          "400": {
            "description": "Invalid input",
            "content": {
              "application/json": {
                "schema": {
                  "type": "string"
                }
              }
            }
          }
        },
        "x-codegen-request-body-name": "body"
      }
    },
    "/todos/{todoId}": {
      "get": {
        "summary": "Details of one Todo",
        "operationId": "Todo_Get",
        "parameters": [
          {
            "name": "todoId",
            "in": "path",
            "required": true,
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "successful operation",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/Todo"
                }
              }
            }
          },
          "404": {
            "description": "Invalid Todo ID value",
            "content": {}
          }
        }
      },
      "delete": {
        "summary": "delete a single todo",
        "operationId": "Todo_Delete",
        "parameters": [
          {
            "name": "todoId",
            "in": "path",
            "required": true,
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "successful operation",
            "content": {}
          },
          "404": {
            "description": "can not find todo",
            "content": {}
          }
        }
      },
      "patch": {
        "summary": "Update an existing Todo",
        "operationId": "Todo_Update",
        "parameters": [
          {
            "name": "todoId",
            "in": "path",
            "required": true,
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "successful operation",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/Todo"
                }
              }
            }
          },
          "404": {
            "description": "Todo not found",
            "content": {}
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "Todo": {
        "required": [
          "title"
        ],
        "type": "object",
        "properties": {
          "id": {
            "type": "string",
            "readOnly": true
          },
          "title": {
            "type": "string"
          },
          "order": {
            "type": "integer",
            "format": "int32"
          },
          "completed": {
            "type": "boolean"
          },
          "url": {
            "type": "string"
          }
        }
      }
    }
  }
}`

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}

func getSchema(r s.Resource) string {
	return must(integration.NewServer("pkg", semver.Version{}, s.Wrap(p.Provider{}, s.Options{
		Resources: []s.Resource{r},
	})).GetSchema(p.GetSchemaRequest{})).Schema
}

var OpenApiSchema = must(openapi3.NewLoader().LoadFromData(
	[]byte(OpenAPISchemaBytes)))

const ExpectedSchema = `{
  "name": "pkg",
  "version": "0.0.0",
  "config": {},
  "provider": {},
  "resources": {
    "pkg:index:Todo": {
      "properties": {
        "completed": {
          "type": "boolean"
        },
        "id": {
          "type": "string"
        },
        "order": {
          "type": "integer"
        },
        "title": {
          "type": "string"
        },
        "todoId": {
          "type": "string"
        },
        "url": {
          "type": "string"
        }
      },
      "required": [
        "completed",
        "id",
        "order",
        "title",
        "todoId",
        "url"
      ],
      "inputProperties": {
        "completed": {
          "type": "boolean"
        },
        "id": {
          "type": "string"
        },
        "order": {
          "type": "integer"
        },
        "title": {
          "type": "string"
        },
        "url": {
          "type": "string"
        }
      },
      "requiredInputs": [
        "completed",
        "id",
        "order",
        "title",
        "url"
      ],
      "stateInputs": {
        "properties": {
          "todoId": {
            "type": "string"
          }
        },
        "required": [
          "todoId"
        ]
      }
    }
  }
}`

func TestSchema(t *testing.T) {
	m := New(OpenApiSchema)
	r := (&Resource{
		Token:  "todo:index:Todo",
		Create: m.NewOperation(OpenApiSchema.Paths["/todos"].Post),
		Read:   m.NewOperation(OpenApiSchema.Paths["/todos/{todoId}"].Get),
		Update: m.NewOperation(OpenApiSchema.Paths["/todos/{todoId}"].Patch),
		Delete: m.NewOperation(OpenApiSchema.Paths["/todos/{todoId}"].Delete),
	}).Schema()

	schema := new(bytes.Buffer)
	require.NoError(t, json.Indent(schema, []byte(getSchema(r)), "", "  "))
	assert.Equal(t, ExpectedSchema, schema.String())
}
