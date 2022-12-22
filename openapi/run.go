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
	"fmt"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"

	p "github.com/pulumi/pulumi-go-provider"
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func runOp(
	ctx p.Context, resource *Resource, op *Operation, inputs presource.PropertyMap,
) (presource.PropertyMap, error) {
	client := op.Client
	if client == nil {
		client = DefaultClient
	}

	req, err := prepareRequest(ctx, op, inputs)
	if err != nil {
		return nil, err
	}

	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	return collectResponse(resource, op, response)
}

func collectResponse(resource *Resource, op *Operation, response *http.Response) (presource.PropertyMap, error) {
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected code: %d", response.StatusCode)
	}
	body := new(bytes.Buffer)
	// TODO: This needs to catch and recover from buffer to large errors.
	len, err := body.ReadFrom(response.Body)
	if err != nil {
		return nil, err
	}
	if len == 0 {
		return presource.PropertyMap{}, nil
	}

	// TODO: We should check for the encoding we requested, but for now I'm assuming its
	// JSON.

	properties := map[string]interface{}{}
	err = json.NewDecoder(body).Decode(&properties)
	if err != nil {
		return nil, err
	}

	// TODO: Verify that the output we received matches the types and values we are
	// expecting.
	//
	// expected, err := op.schemaOutputs(resource,
	// 	func(tk tokens.Type, typ schema.ComplexTypeSpec) (unknown bool) { return true })
	// if err != nil {
	// 	return nil, fmt.Errorf("discovering expected return values: %w", err)
	// }
	return presource.NewPropertyMapFromMap(properties), nil
}

func prepareRequest(ctx p.Context, op *Operation, inputs presource.PropertyMap) (*http.Request, error) {
	url, err := op.url()
	if err != nil {
		return nil, fmt.Errorf("retrieving url: %w", err)
	}
	header := http.Header{}
	cookies := []*http.Cookie{}
	bodyParams := inputs.Copy()
	for i, paramRef := range op.Parameters {
		param := paramRef.Value
		v, ok := inputs[presource.PropertyKey(param.Name)]
		if ok {
			// This is not a body value since it is a Parameter.
			delete(bodyParams, presource.PropertyKey(param.Name))
			v = unwrapPValue(v)
		}
		switch param.In {
		case openapi3.ParameterInPath:
			if !ok {
				return nil, fmt.Errorf("%s: missing path parameter", param.Name)
			}

			s, err := stringifyPValue(v)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", param.Name, err)
			}
			url.replace(param.Name, s)
		case openapi3.ParameterInQuery:
			if !ok {
				continue
			}
			s, err := stringifyPValue(v)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", param.Name, err)
			}
			url.query(param.Name, s)
		case openapi3.ParameterInHeader:
			if !ok {
				continue
			}
			s, err := stringifyPValue(v)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", param.Name, err)
			}
			header.Add(param.Name, s)
		case openapi3.ParameterInCookie:
			if !ok {
				continue
			}
			s, err := stringifyPValue(v)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", param.Name, err)
			}
			cookies = append(cookies, &http.Cookie{
				Name:  param.Name,
				Value: s,
			})

		default:
			return nil, fmt.Errorf("parameter[%d] has invalid 'in:' component: %q", i, param.In)
		}

	}

	body := op.body()
	for name, value := range bodyParams {
		v, err := jsonifyPValue(value)
		if err != nil {
			return nil, err
		}
		body.add(string(name), v)
	}

	req, err := http.NewRequestWithContext(ctx, op.method(), url.build(), body.build())
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}
	if req.Header == nil {
		req.Header = header
	} else {
		for k, v := range header {
			for _, v := range v {
				req.Header.Add(k, v)
			}
		}
	}

	// NOTE: When we support arbitrary encodings, we will need to parameterize by the
	// encoding used.
	req.Header.Set("Content-Type", "application/json")

	for _, c := range cookies {
		req.AddCookie(c)
	}
	return req, nil
}
