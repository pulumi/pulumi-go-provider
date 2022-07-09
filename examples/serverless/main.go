package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/blang/semver"
	p "github.com/iwahbe/pulumi-go-provider"
	r "github.com/iwahbe/pulumi-go-provider/resource"
	apigateway "github.com/pulumi/pulumi-aws-apigateway/sdk/go/apigateway"
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/lambda"
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/s3"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	err := p.Run("serverless", semver.Version{Minor: 1},
		p.Components(
			&Service{},
		),
		p.Types(
			&Function{},
			&ServiceProvider{},
			&Event{},
			&ProviderIam{},
			&ProviderIamRole{},
			&HttpEvent{},
			&SqsEvent{},
		),
		p.PartialSpec(schema.PackageSpec{}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
}

type Service struct {
	pulumi.ResourceState

	// Outputs
	Url pulumi.StringPtrOutput `pulumi:"url" provider:"output"`
	// Lambdas []*lambda.Function     `pulumi:"lambdas" provider:"output"`

	// Inputs
	Functions FunctionMapInput     `pulumi:"functions"`
	Provider  ServiceProviderInput `pulumi:"provider"`
}

func (s *Service) Annotate(a r.Annotator) {
	a.Describe(&s.Functions, "Configure the functions to deploy.")
	a.Describe(&s.Provider, "Configure general settings to apply across all functions.")

	a.Describe(&s.Url, "The URL at which any HTTP handlers are exposed, if any of the functions expose HTTP handlers.")
}

func (s *Service) Construct(name string, ctx *pulumi.Context) error {
	component := &Service{}
	// What about opts?
	err := ctx.RegisterComponentResource("serverless:index:Service", name, component)
	if err != nil {
		return err
	}

	functionsOutput := s.Functions.ToFunctionMapOutput()
	providerOutput := s.Provider.ToServiceProviderOutput()

	// Create IAM Role
	defaultPolicies := []string{
		string(iam.ManagedPolicyAWSLambdaBasicExecutionRole),
	}
	executionRole, err := iam.NewRole(ctx, "role", &iam.RoleArgs{
		AssumeRolePolicy: pulumi.ToMap(map[string]interface{}{
			"Version": "2012-10-17",
			"Statement": []interface{}{
				map[string]interface{}{
					"Effect": "Allow",
					"Principal": map[string]interface{}{
						"Service": []string{
							"lambda.amazonaws.com",
						},
					},
					"Action": []string{
						"sts:AssumeRole",
					},
				},
			},
		}),
		ManagedPolicyArns: providerOutput.Iam().Role().ManagedPolicies().ApplyT(func(policies []string) []string {
			return append(defaultPolicies, policies...)
		}).(pulumi.StringArrayOutput),
	}, pulumi.Parent(component))
	if err != nil {
		return err
	}

	// Create S3 Bucket
	bucket, err := s3.NewBucket(ctx, "bucket", &s3.BucketArgs{
		Versioning: &s3.BucketVersioningArgs{
			Enabled: pulumi.Bool(true),
		},
		ForceDestroy: pulumi.Bool(true),
	}, pulumi.Parent(component))
	if err != nil {
		return err
	}

	// Compute the contents to add to the bucket
	paths, err := allPaths(".")
	if err != nil {
		return err
	}

	// Upload Package to Bucket
	object, err := s3.NewBucketObject(ctx, "payload", &s3.BucketObjectArgs{
		Bucket: bucket.ID(),
		Key:    pulumi.String("functions.zip"),
		Source: pulumi.NewAssetArchive(paths),
	}, pulumi.Parent(component))
	if err != nil {
		return err
	}

	// Create Functions
	out := functionsOutput.ApplyT(func(functions map[string]Function) (pulumi.StringPtrOutput, error) {
		var routes []apigateway.RouteArgs

		for fname, f := range functions {
			var handler pulumi.StringPtrInput
			if f.Handler != nil {
				handler = pulumi.StringPtr(*f.Handler)
			}
			var envArgs *lambda.FunctionEnvironmentArgs
			if len(f.Environment) > 0 {
				envArgs = &lambda.FunctionEnvironmentArgs{
					Variables: pulumi.ToStringMap(f.Environment),
				}
			}
			function, err := lambda.NewFunction(ctx, fname, &lambda.FunctionArgs{
				Handler:         handler,
				Runtime:         providerOutput.Runtime(),
				S3Bucket:        bucket.ID(),
				S3Key:           object.Key,
				S3ObjectVersion: object.VersionId,
				Role:            executionRole.Arn,
				MemorySize:      pulumi.Int(1024),
				Timeout:         pulumi.Int(6),
				Environment:     envArgs,
			}, pulumi.Parent(component))
			if err != nil {
				return pulumi.StringPtr("").ToStringPtrOutput(), err
			}
			// component.Lambdas = append(component.Lambdas, function)

			for i, ev := range f.Events {
				if ev.Sqs != nil {
					// TODO[pulumi/pulumi#6957]: This should not be necessary - but appears to be needed
					// because we have to run this in an apply.
					eventSourceArn := ""
					if ev.Sqs.Arn != nil {
						eventSourceArn = *ev.Sqs.Arn
					}
					lambda.NewEventSourceMapping(ctx, fmt.Sprintf("%s-%s-%d", fname, "sqs", i), &lambda.EventSourceMappingArgs{
						FunctionName:   function.Arn,
						BatchSize:      pulumi.Int(10),
						EventSourceArn: pulumi.String(eventSourceArn),
						Enabled:        pulumi.Bool(true),
					}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{executionRole}))
				} else if ev.Http != nil {
					var method *apigateway.Method
					if ev.Http.Method != nil {
						m := apigateway.Method(*ev.Http.Method)
						method = &m
					}
					routes = append(routes, apigateway.RouteArgs{
						Path:         *ev.Http.Path,
						Method:       method,
						EventHandler: function,
					})

					lambda.NewPermission(ctx, fmt.Sprintf("%s-%s-%d", fname, "http", i), &lambda.PermissionArgs{
						Function:  function.Arn,
						Action:    pulumi.String("lambda:InvokeFunction"),
						Principal: pulumi.String("apigateway.amazonaws.com"),
						// TODO: SourceArn
					}, pulumi.Parent(component))
				}
			}
		}

		var url pulumi.StringPtrOutput
		if routes != nil {
			apigw, err := apigateway.NewRestAPI(ctx, "api", &apigateway.RestAPIArgs{
				Routes: routes,
			}, pulumi.Parent(component))
			if err != nil {
				return pulumi.StringPtr("").ToStringPtrOutput(), err
			}
			url = apigw.Url.ToStringPtrOutput()
		}

		return url, nil
	})

	// TODO[pulumi/pulumi#6073]: Workaround issue with not being able to directly return outputs from Apply.
	component.Url = out.ApplyT(func(v interface{}) *string {
		return v.(*string)
	}).(pulumi.StringPtrOutput)

	if err := ctx.RegisterResourceOutputs(component, pulumi.Map{
		"url": component.Url,
	}); err != nil {
		return err
	}

	return nil
}

// allPaths computes the file paths from within the given folder, applying
// the exclusions of certain files.
func allPaths(folder string) (map[string]interface{}, error) {
	paths := map[string]interface{}{}
	err := filepath.WalkDir(folder, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// TODO: Allow much richer specification of inclusions/exclusions from the packaging.
		for _, exclude := range []string{"Pulumi.yaml", "Pulumi.*.yaml", "."} {
			matched, err := filepath.Match(exclude, path)
			if err != nil {
				return err
			}
			if matched {
				return nil
			}
		}
		if !d.IsDir() {
			paths[path] = pulumi.NewFileAsset(path)
		}
		return nil
	})
	return paths, err
}
