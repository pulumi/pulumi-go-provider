package main

import (
	kubernetes "example/kubernetes"

	"github.com/pulumi/pulumi-go-provider/component"
	"github.com/pulumi/pulumi-go-provider/infer"
)

func init() {
	component.RegisterType(infer.ComponentProviderResource(
		infer.ComponentFn[kubernetes.DeploymentArgs, *kubernetes.NginxDeployment](kubernetes.CreateNginxDeployment)))
	component.RegisterType(infer.ComponentProviderResource(
		infer.ComponentFn[RandomComponentArgs, *RandomComponent](NewMyComponent)))
}
