// Copyright 2016-2025, Pulumi Corporation.
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

package kubernetes

import (
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type NginxDeployment struct {
	pulumi.ResourceState
	DeploymentArgs
	Image pulumi.StringPtrOutput `pulumi:"image"`
}

type DeploymentArgs struct {
	Replicas pulumi.IntInput `pulumi:"replicas"`
}

func CreateNginxDeployment(ctx *pulumi.Context, name string, compArgs DeploymentArgs, opts ...pulumi.ResourceOption) (*NginxDeployment, error) {
	comp := &NginxDeployment{}
	err := ctx.RegisterComponentResource("pkg:index:MyNginxDeployment", name, comp, opts...)
	if err != nil {
		return nil, err
	}

	dep1Args := &appsv1.DeploymentArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Labels: pulumi.StringMap{
				"app": pulumi.String("nginx"),
			},
		},
		Spec: &appsv1.DeploymentSpecArgs{
			Replicas: compArgs.Replicas,
			Selector: &metav1.LabelSelectorArgs{
				MatchLabels: pulumi.StringMap{
					"app": pulumi.String("nginx"),
				},
			},
			Template: &corev1.PodTemplateSpecArgs{
				Metadata: &metav1.ObjectMetaArgs{
					Labels: pulumi.StringMap{
						"app": pulumi.String("nginx"),
					},
				},
				Spec: &corev1.PodSpecArgs{
					Containers: corev1.ContainerArray{
						&corev1.ContainerArgs{
							Image: pulumi.String("nginx:1.14.2"),
							Name:  pulumi.String("nginx"),
							Ports: corev1.ContainerPortArray{
								&corev1.ContainerPortArgs{
									ContainerPort: pulumi.Int(80),
								},
							},
						},
					},
				},
			},
		},
	}

	dep1, err := appsv1.NewDeployment(ctx, name+"-dep1", dep1Args, pulumi.Parent(comp))
	if err != nil {
		return nil, err
	}

	comp.Image = dep1.Spec.Template().Spec().Containers().Index(pulumi.Int(0)).Image()

	// Create a ConfigMap that is nested under the Deployment.
	_, err = corev1.NewConfigMap(ctx, name+"-cm", &corev1.ConfigMapArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: dep1.Metadata.Name(),
		},
	}, pulumi.Parent(dep1))
	if err != nil {
		return nil, err
	}

	return comp, nil
}
