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
	"testing"

	"github.com/blang/semver"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	s "github.com/pulumi/pulumi-go-provider/middleware/schema"
)

const OpenAPISchemaBytes = `{
  "swagger": "2.0",
  "info": {
    "description": "Todo Backend API",
    "version": "1.0.0",
    "title": "Todos API"
  },
  "host": "functodobackend.azurewebsites.net",
  "schemes": [
    "https"
  ],
  "basePath": "/api",
  "paths": {
    "/todos": {
      "post": {
        "summary": "Create a new todo",
        "operationId": "Todo_Create",
        "responses": {
          "200": {
            "description": "successful operation",
            "schema": {
              "$ref": "#/definitions/Todo"
            }
          },
          "400": {
            "description": "Invalid input",
            "schema": {
              "type": "string"
            }
          }
        },
        "parameters": [
          {
            "description": "Todo Object",
            "required": true,
            "name": "body",
            "in": "body",
            "schema": {
              "$ref": "#/definitions/Todo"
            }
          }
        ],
        "consumes": [
          "application/json"
        ],
        "produces": [
          "application/json"
        ]
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
            "description": "",
            "required": true,
            "type": "string"
          }
        ],
        "responses": {
          "200": {
            "description": "successful operation",
            "schema": {
              "$ref": "#/definitions/Todo"
            }
          },
          "404": {
            "description": "Invalid Todo ID value"
          }
        },
        "produces": [
          "application/json"
        ]
      },
      "delete": {
        "summary": "delete a single todo",
        "operationId": "Todo_Delete",
        "parameters": [
          {
            "name": "todoId",
            "in": "path",
            "description": "",
            "required": true,
            "type": "string"
          }
        ],
        "responses": {
          "200": {
            "description": "successful operation"
          },
          "404": {
            "description": "can not find todo"
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
            "description": "",
            "required": true,
            "type": "string"
          }
        ],
        "responses": {
          "200": {
            "description": "successful operation",
            "schema": {
              "$ref": "#/definitions/Todo"
            }
          },
          "404": {
            "description": "Todo not found"
          }
        },
        "produces": [
          "application/json"
        ]
      }
    }
  },
  "definitions": {
    "Todo": {
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
      },
      "required": ["title"]
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

func TestSchema(t *testing.T) {
	r := (&Resource{
		Token: "todo:index:Todo",
		Create: &Operation{
			Operation: *OpenApiSchema.Paths["/todos"].Post,
		},
		Read: &Operation{
			Operation: *OpenApiSchema.Paths["/todos/{todoId}"].Get,
		},
		Update: &Operation{
			Operation: *OpenApiSchema.Paths["/todos/{todoId}"].Patch,
		},
		Delete: &Operation{
			Operation: *OpenApiSchema.Paths["/todos/{todoId}"].Delete,
		},
	}).Schema()

	assert.Equal(t, "", getSchema(r))
}
