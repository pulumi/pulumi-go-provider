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

package types

import (
	"fmt"
	"reflect"

	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

func GetResourceSchema[R, I, O any](isComponent bool) (schema.ResourceSpec, multierror.Error) {
	var r R
	var errs multierror.Error
	descriptions := GetAnnotated(reflect.TypeOf(r))

	properties, required, err := propertyListFromType(reflect.TypeOf(new(O)), isComponent)
	if err != nil {
		var o O
		errs.Errors = append(errs.Errors, fmt.Errorf("could not serialize output type %T: %w", o, err))
	}

	inputProperties, requiredInputs, err := propertyListFromType(reflect.TypeOf(new(I)), isComponent)
	if err != nil {
		var i I
		errs.Errors = append(errs.Errors, fmt.Errorf("could not serialize input type %T: %w", i, err))
	}

	return schema.ResourceSpec{
		ObjectTypeSpec: schema.ObjectTypeSpec{
			Properties:  properties,
			Description: descriptions.Descriptions[""],
			Required:    required,
		},
		InputProperties: inputProperties,
		RequiredInputs:  requiredInputs,
		IsComponent:     isComponent,
	}, errs
}
