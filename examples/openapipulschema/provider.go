package main

import (
	"fmt"
	"net/url"
	"os"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
	"github.com/pulumi/pulumi-go-provider/openapipulschema"
)

const (
	// Comes out empty after loading by kin-openapi/openapi3
	apigatewayV2Spec = "https://raw.githubusercontent.com/aws/aws-sdk-js/master/apis/apigatewayv2-2018-11-29.normal.json"

	// Can be loaded by kin-openapi/openapi3 but pulschema fails
	asanaSpec = "https://raw.githubusercontent.com/Asana/developer-docs/master/defs/asana_oas.yaml"

	// Can be loaded by kin-openapi/openapi3 but pulschema fails
	// "failed to generate property types for {Extensions:map[] OneOf:[] AnyOf:[] AllOf:[] Not:<nil> Type: Title: Format: Description:The value of the property. Required on create and update. Enum:[] Default:<nil> Example:<nil> ExternalDocs:<nil> UniqueItems:false ExclusiveMin:false ExclusiveMax:false Nullable:false ReadOnly:false WriteOnly:false AllowEmptyValue:false Deprecated:false XML:<nil> Min:<nil> Max:<nil> MultipleOf:<nil> MinLength:0 MaxLength:<nil> Pattern: MinItems:0 MaxItems:<nil> Items:<nil> Required:[] Properties:map[] MinProps:0 MaxProps:<nil> AdditionalProperties:{Has:<nil> Schema:<nil>} Discriminator:<nil>}"
	jiraSpec = "https://developer.atlassian.com/cloud/jira/platform/swagger-v3.v3.json"

	// Can be loaded by pulschema but fails jsonschema validation of `pulumi package get-schema`
	openaiSpec = "https://raw.githubusercontent.com/openai/openai-openapi/master/openapi.yaml"

	// Cannot be loaded by pulschema due to the use of $ref
	azureResourcesSpec = "https://raw.githubusercontent.com/Azure/azure-rest-api-specs/master/specification/resources/resource-manager/Microsoft.Resources/stable/2019-08-01/resources.json"
)

func main() {
	specUri, err := url.Parse(azureResourcesSpec)
	exitIfErr(err)

	m := schema.Metadata{
		DisplayName: "ARM via OpenAI",
	}

	provider, err := openapipulschema.Provider(*specUri, m)
	exitIfErr(err)

	err = p.RunProvider("azurerm", "0.1.0", *provider)
	exitIfErr(err)
}

func exitIfErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
