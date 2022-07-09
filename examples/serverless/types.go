package main

import (
	"context"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type serviceArgs struct {
	Functions map[string]Function `pulumi:"functions"`
	Provider  ServiceProvider     `pulumi:"provider"`
}

// The set of arguments for constructing a Service resource.
type ServiceArgs struct {
	Functions FunctionMapInput     `pulumi:"functions"`
	Provider  ServiceProviderInput `pulumi:"provider"`
}

func (ServiceArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*serviceArgs)(nil)).Elem()
}

type ServiceInput interface {
	pulumi.Input

	ToServiceOutput() ServiceOutput
	ToServiceOutputWithContext(ctx context.Context) ServiceOutput
}

func (*Service) ElementType() reflect.Type {
	return reflect.TypeOf((*Service)(nil))
}

func (i *Service) ToServiceOutput() ServiceOutput {
	return i.ToServiceOutputWithContext(context.Background())
}

func (i *Service) ToServiceOutputWithContext(ctx context.Context) ServiceOutput {
	return pulumi.ToOutputWithContext(ctx, i).(ServiceOutput)
}

func (i *Service) ToServicePtrOutput() ServicePtrOutput {
	return i.ToServicePtrOutputWithContext(context.Background())
}

func (i *Service) ToServicePtrOutputWithContext(ctx context.Context) ServicePtrOutput {
	return pulumi.ToOutputWithContext(ctx, i).(ServicePtrOutput)
}

type ServicePtrInput interface {
	pulumi.Input

	ToServicePtrOutput() ServicePtrOutput
	ToServicePtrOutputWithContext(ctx context.Context) ServicePtrOutput
}

type servicePtrType ServiceArgs

func (*servicePtrType) ElementType() reflect.Type {
	return reflect.TypeOf((**Service)(nil))
}

func (i *servicePtrType) ToServicePtrOutput() ServicePtrOutput {
	return i.ToServicePtrOutputWithContext(context.Background())
}

func (i *servicePtrType) ToServicePtrOutputWithContext(ctx context.Context) ServicePtrOutput {
	return pulumi.ToOutputWithContext(ctx, i).(ServicePtrOutput)
}

// ServiceArrayInput is an input type that accepts ServiceArray and ServiceArrayOutput values.
// You can construct a concrete instance of `ServiceArrayInput` via:
//
//          ServiceArray{ ServiceArgs{...} }
type ServiceArrayInput interface {
	pulumi.Input

	ToServiceArrayOutput() ServiceArrayOutput
	ToServiceArrayOutputWithContext(context.Context) ServiceArrayOutput
}

type ServiceArray []ServiceInput

func (ServiceArray) ElementType() reflect.Type {
	return reflect.TypeOf(([]*Service)(nil))
}

func (i ServiceArray) ToServiceArrayOutput() ServiceArrayOutput {
	return i.ToServiceArrayOutputWithContext(context.Background())
}

func (i ServiceArray) ToServiceArrayOutputWithContext(ctx context.Context) ServiceArrayOutput {
	return pulumi.ToOutputWithContext(ctx, i).(ServiceArrayOutput)
}

// ServiceMapInput is an input type that accepts ServiceMap and ServiceMapOutput values.
// You can construct a concrete instance of `ServiceMapInput` via:
//
//          ServiceMap{ "key": ServiceArgs{...} }
type ServiceMapInput interface {
	pulumi.Input

	ToServiceMapOutput() ServiceMapOutput
	ToServiceMapOutputWithContext(context.Context) ServiceMapOutput
}

type ServiceMap map[string]ServiceInput

func (ServiceMap) ElementType() reflect.Type {
	return reflect.TypeOf((map[string]*Service)(nil))
}

func (i ServiceMap) ToServiceMapOutput() ServiceMapOutput {
	return i.ToServiceMapOutputWithContext(context.Background())
}

func (i ServiceMap) ToServiceMapOutputWithContext(ctx context.Context) ServiceMapOutput {
	return pulumi.ToOutputWithContext(ctx, i).(ServiceMapOutput)
}

type ServiceOutput struct {
	*pulumi.OutputState
}

func (ServiceOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*Service)(nil))
}

func (o ServiceOutput) ToServiceOutput() ServiceOutput {
	return o
}

func (o ServiceOutput) ToServiceOutputWithContext(ctx context.Context) ServiceOutput {
	return o
}

func (o ServiceOutput) ToServicePtrOutput() ServicePtrOutput {
	return o.ToServicePtrOutputWithContext(context.Background())
}

func (o ServiceOutput) ToServicePtrOutputWithContext(ctx context.Context) ServicePtrOutput {
	return o.ApplyT(func(v Service) *Service {
		return &v
	}).(ServicePtrOutput)
}

type ServicePtrOutput struct {
	*pulumi.OutputState
}

func (ServicePtrOutput) ElementType() reflect.Type {
	return reflect.TypeOf((**Service)(nil))
}

func (o ServicePtrOutput) ToServicePtrOutput() ServicePtrOutput {
	return o
}

func (o ServicePtrOutput) ToServicePtrOutputWithContext(ctx context.Context) ServicePtrOutput {
	return o
}

type ServiceArrayOutput struct{ *pulumi.OutputState }

func (ServiceArrayOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*[]Service)(nil))
}

func (o ServiceArrayOutput) ToServiceArrayOutput() ServiceArrayOutput {
	return o
}

func (o ServiceArrayOutput) ToServiceArrayOutputWithContext(ctx context.Context) ServiceArrayOutput {
	return o
}

func (o ServiceArrayOutput) Index(i pulumi.IntInput) ServiceOutput {
	return pulumi.All(o, i).ApplyT(func(vs []interface{}) Service {
		return vs[0].([]Service)[vs[1].(int)]
	}).(ServiceOutput)
}

type ServiceMapOutput struct{ *pulumi.OutputState }

func (ServiceMapOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*map[string]Service)(nil))
}

func (o ServiceMapOutput) ToServiceMapOutput() ServiceMapOutput {
	return o
}

func (o ServiceMapOutput) ToServiceMapOutputWithContext(ctx context.Context) ServiceMapOutput {
	return o
}

func (o ServiceMapOutput) MapIndex(k pulumi.StringInput) ServiceOutput {
	return pulumi.All(o, k).ApplyT(func(vs []interface{}) Service {
		return vs[0].(map[string]Service)[vs[1].(string)]
	}).(ServiceOutput)
}

type Event struct {
	Http *HttpEvent `pulumi:"http,optional"`
	Sqs  *SqsEvent  `pulumi:"sqs,optional"`
}

// EventInput is an input type that accepts EventArgs and EventOutput values.
// You can construct a concrete instance of `EventInput` via:
//
//          EventArgs{...}
type EventInput interface {
	pulumi.Input

	ToEventOutput() EventOutput
	ToEventOutputWithContext(context.Context) EventOutput
}

type EventArgs struct {
	Http HttpEventPtrInput `pulumi:"http"`
	Sqs  SqsEventPtrInput  `pulumi:"sqs"`
}

func (EventArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*Event)(nil)).Elem()
}

func (i EventArgs) ToEventOutput() EventOutput {
	return i.ToEventOutputWithContext(context.Background())
}

func (i EventArgs) ToEventOutputWithContext(ctx context.Context) EventOutput {
	return pulumi.ToOutputWithContext(ctx, i).(EventOutput)
}

// EventArrayInput is an input type that accepts EventArray and EventArrayOutput values.
// You can construct a concrete instance of `EventArrayInput` via:
//
//          EventArray{ EventArgs{...} }
type EventArrayInput interface {
	pulumi.Input

	ToEventArrayOutput() EventArrayOutput
	ToEventArrayOutputWithContext(context.Context) EventArrayOutput
}

type EventArray []EventInput

func (EventArray) ElementType() reflect.Type {
	return reflect.TypeOf((*[]Event)(nil)).Elem()
}

func (i EventArray) ToEventArrayOutput() EventArrayOutput {
	return i.ToEventArrayOutputWithContext(context.Background())
}

func (i EventArray) ToEventArrayOutputWithContext(ctx context.Context) EventArrayOutput {
	return pulumi.ToOutputWithContext(ctx, i).(EventArrayOutput)
}

type EventOutput struct{ *pulumi.OutputState }

func (EventOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*Event)(nil)).Elem()
}

func (o EventOutput) ToEventOutput() EventOutput {
	return o
}

func (o EventOutput) ToEventOutputWithContext(ctx context.Context) EventOutput {
	return o
}

func (o EventOutput) Http() HttpEventPtrOutput {
	return o.ApplyT(func(v Event) *HttpEvent { return v.Http }).(HttpEventPtrOutput)
}

func (o EventOutput) Sqs() SqsEventPtrOutput {
	return o.ApplyT(func(v Event) *SqsEvent { return v.Sqs }).(SqsEventPtrOutput)
}

type EventArrayOutput struct{ *pulumi.OutputState }

func (EventArrayOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*[]Event)(nil)).Elem()
}

func (o EventArrayOutput) ToEventArrayOutput() EventArrayOutput {
	return o
}

func (o EventArrayOutput) ToEventArrayOutputWithContext(ctx context.Context) EventArrayOutput {
	return o
}

func (o EventArrayOutput) Index(i pulumi.IntInput) EventOutput {
	return pulumi.All(o, i).ApplyT(func(vs []interface{}) Event {
		return vs[0].([]Event)[vs[1].(int)]
	}).(EventOutput)
}

type Function struct {
	Environment map[string]string `pulumi:"environment,optional"`
	Events      []Event           `pulumi:"events,optional"`
	Handler     *string           `pulumi:"handler,optional"`
}

// FunctionInput is an input type that accepts FunctionArgs and FunctionOutput values.
// You can construct a concrete instance of `FunctionInput` via:
//
//          FunctionArgs{...}
type FunctionInput interface {
	pulumi.Input

	ToFunctionOutput() FunctionOutput
	ToFunctionOutputWithContext(context.Context) FunctionOutput
}

type FunctionArgs struct {
	Environment pulumi.StringMapInput `pulumi:"environment"`
	Events      EventArrayInput       `pulumi:"events"`
	Handler     pulumi.StringPtrInput `pulumi:"handler"`
}

func (FunctionArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*Function)(nil)).Elem()
}

func (i FunctionArgs) ToFunctionOutput() FunctionOutput {
	return i.ToFunctionOutputWithContext(context.Background())
}

func (i FunctionArgs) ToFunctionOutputWithContext(ctx context.Context) FunctionOutput {
	return pulumi.ToOutputWithContext(ctx, i).(FunctionOutput)
}

// FunctionMapInput is an input type that accepts FunctionMap and FunctionMapOutput values.
// You can construct a concrete instance of `FunctionMapInput` via:
//
//          FunctionMap{ "key": FunctionArgs{...} }
type FunctionMapInput interface {
	pulumi.Input

	ToFunctionMapOutput() FunctionMapOutput
	ToFunctionMapOutputWithContext(context.Context) FunctionMapOutput
}

type FunctionMap map[string]FunctionInput

func (FunctionMap) ElementType() reflect.Type {
	return reflect.TypeOf((*map[string]Function)(nil)).Elem()
}

func (i FunctionMap) ToFunctionMapOutput() FunctionMapOutput {
	return i.ToFunctionMapOutputWithContext(context.Background())
}

func (i FunctionMap) ToFunctionMapOutputWithContext(ctx context.Context) FunctionMapOutput {
	return pulumi.ToOutputWithContext(ctx, i).(FunctionMapOutput)
}

type FunctionOutput struct{ *pulumi.OutputState }

func (FunctionOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*Function)(nil)).Elem()
}

func (o FunctionOutput) ToFunctionOutput() FunctionOutput {
	return o
}

func (o FunctionOutput) ToFunctionOutputWithContext(ctx context.Context) FunctionOutput {
	return o
}

func (o FunctionOutput) Environment() pulumi.StringMapOutput {
	return o.ApplyT(func(v Function) map[string]string { return v.Environment }).(pulumi.StringMapOutput)
}

func (o FunctionOutput) Events() EventArrayOutput {
	return o.ApplyT(func(v Function) []Event { return v.Events }).(EventArrayOutput)
}

func (o FunctionOutput) Handler() pulumi.StringPtrOutput {
	return o.ApplyT(func(v Function) *string { return v.Handler }).(pulumi.StringPtrOutput)
}

type FunctionMapOutput struct{ *pulumi.OutputState }

func (FunctionMapOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*map[string]Function)(nil)).Elem()
}

func (o FunctionMapOutput) ToFunctionMapOutput() FunctionMapOutput {
	return o
}

func (o FunctionMapOutput) ToFunctionMapOutputWithContext(ctx context.Context) FunctionMapOutput {
	return o
}

func (o FunctionMapOutput) MapIndex(k pulumi.StringInput) FunctionOutput {
	return pulumi.All(o, k).ApplyT(func(vs []interface{}) Function {
		return vs[0].(map[string]Function)[vs[1].(string)]
	}).(FunctionOutput)
}

type HttpEvent struct {
	Method *string `pulumi:"method,optional"`
	Path   *string `pulumi:"path,optional"`
}

// HttpEventInput is an input type that accepts HttpEventArgs and HttpEventOutput values.
// You can construct a concrete instance of `HttpEventInput` via:
//
//          HttpEventArgs{...}
type HttpEventInput interface {
	pulumi.Input

	ToHttpEventOutput() HttpEventOutput
	ToHttpEventOutputWithContext(context.Context) HttpEventOutput
}

type HttpEventArgs struct {
	Method pulumi.StringPtrInput `pulumi:"method"`
	Path   pulumi.StringPtrInput `pulumi:"path"`
}

func (HttpEventArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*HttpEvent)(nil)).Elem()
}

func (i HttpEventArgs) ToHttpEventOutput() HttpEventOutput {
	return i.ToHttpEventOutputWithContext(context.Background())
}

func (i HttpEventArgs) ToHttpEventOutputWithContext(ctx context.Context) HttpEventOutput {
	return pulumi.ToOutputWithContext(ctx, i).(HttpEventOutput)
}

func (i HttpEventArgs) ToHttpEventPtrOutput() HttpEventPtrOutput {
	return i.ToHttpEventPtrOutputWithContext(context.Background())
}

func (i HttpEventArgs) ToHttpEventPtrOutputWithContext(ctx context.Context) HttpEventPtrOutput {
	return pulumi.ToOutputWithContext(ctx, i).(HttpEventOutput).ToHttpEventPtrOutputWithContext(ctx)
}

// HttpEventPtrInput is an input type that accepts HttpEventArgs, HttpEventPtr and HttpEventPtrOutput values.
// You can construct a concrete instance of `HttpEventPtrInput` via:
//
//          HttpEventArgs{...}
//
//  or:
//
//          nil
type HttpEventPtrInput interface {
	pulumi.Input

	ToHttpEventPtrOutput() HttpEventPtrOutput
	ToHttpEventPtrOutputWithContext(context.Context) HttpEventPtrOutput
}

type httpEventPtrType HttpEventArgs

func HttpEventPtr(v *HttpEventArgs) HttpEventPtrInput {
	return (*httpEventPtrType)(v)
}

func (*httpEventPtrType) ElementType() reflect.Type {
	return reflect.TypeOf((**HttpEvent)(nil)).Elem()
}

func (i *httpEventPtrType) ToHttpEventPtrOutput() HttpEventPtrOutput {
	return i.ToHttpEventPtrOutputWithContext(context.Background())
}

func (i *httpEventPtrType) ToHttpEventPtrOutputWithContext(ctx context.Context) HttpEventPtrOutput {
	return pulumi.ToOutputWithContext(ctx, i).(HttpEventPtrOutput)
}

type HttpEventOutput struct{ *pulumi.OutputState }

func (HttpEventOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*HttpEvent)(nil)).Elem()
}

func (o HttpEventOutput) ToHttpEventOutput() HttpEventOutput {
	return o
}

func (o HttpEventOutput) ToHttpEventOutputWithContext(ctx context.Context) HttpEventOutput {
	return o
}

func (o HttpEventOutput) ToHttpEventPtrOutput() HttpEventPtrOutput {
	return o.ToHttpEventPtrOutputWithContext(context.Background())
}

func (o HttpEventOutput) ToHttpEventPtrOutputWithContext(ctx context.Context) HttpEventPtrOutput {
	return o.ApplyT(func(v HttpEvent) *HttpEvent {
		return &v
	}).(HttpEventPtrOutput)
}
func (o HttpEventOutput) Method() pulumi.StringPtrOutput {
	return o.ApplyT(func(v HttpEvent) *string { return v.Method }).(pulumi.StringPtrOutput)
}

func (o HttpEventOutput) Path() pulumi.StringPtrOutput {
	return o.ApplyT(func(v HttpEvent) *string { return v.Path }).(pulumi.StringPtrOutput)
}

type HttpEventPtrOutput struct{ *pulumi.OutputState }

func (HttpEventPtrOutput) ElementType() reflect.Type {
	return reflect.TypeOf((**HttpEvent)(nil)).Elem()
}

func (o HttpEventPtrOutput) ToHttpEventPtrOutput() HttpEventPtrOutput {
	return o
}

func (o HttpEventPtrOutput) ToHttpEventPtrOutputWithContext(ctx context.Context) HttpEventPtrOutput {
	return o
}

func (o HttpEventPtrOutput) Elem() HttpEventOutput {
	return o.ApplyT(func(v *HttpEvent) HttpEvent { return *v }).(HttpEventOutput)
}

func (o HttpEventPtrOutput) Method() pulumi.StringPtrOutput {
	return o.ApplyT(func(v *HttpEvent) *string {
		if v == nil {
			return nil
		}
		return v.Method
	}).(pulumi.StringPtrOutput)
}

func (o HttpEventPtrOutput) Path() pulumi.StringPtrOutput {
	return o.ApplyT(func(v *HttpEvent) *string {
		if v == nil {
			return nil
		}
		return v.Path
	}).(pulumi.StringPtrOutput)
}

type ProviderIam struct {
	Role *ProviderIamRole `pulumi:"role,optional"`
}

// ProviderIamInput is an input type that accepts ProviderIamArgs and ProviderIamOutput values.
// You can construct a concrete instance of `ProviderIamInput` via:
//
//          ProviderIamArgs{...}
type ProviderIamInput interface {
	pulumi.Input

	ToProviderIamOutput() ProviderIamOutput
	ToProviderIamOutputWithContext(context.Context) ProviderIamOutput
}

type ProviderIamArgs struct {
	Role ProviderIamRolePtrInput `pulumi:"role"`
}

func (ProviderIamArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*ProviderIam)(nil)).Elem()
}

func (i ProviderIamArgs) ToProviderIamOutput() ProviderIamOutput {
	return i.ToProviderIamOutputWithContext(context.Background())
}

func (i ProviderIamArgs) ToProviderIamOutputWithContext(ctx context.Context) ProviderIamOutput {
	return pulumi.ToOutputWithContext(ctx, i).(ProviderIamOutput)
}

func (i ProviderIamArgs) ToProviderIamPtrOutput() ProviderIamPtrOutput {
	return i.ToProviderIamPtrOutputWithContext(context.Background())
}

func (i ProviderIamArgs) ToProviderIamPtrOutputWithContext(ctx context.Context) ProviderIamPtrOutput {
	return pulumi.ToOutputWithContext(ctx, i).(ProviderIamOutput).ToProviderIamPtrOutputWithContext(ctx)
}

// ProviderIamPtrInput is an input type that accepts ProviderIamArgs, ProviderIamPtr and ProviderIamPtrOutput values.
// You can construct a concrete instance of `ProviderIamPtrInput` via:
//
//          ProviderIamArgs{...}
//
//  or:
//
//          nil
type ProviderIamPtrInput interface {
	pulumi.Input

	ToProviderIamPtrOutput() ProviderIamPtrOutput
	ToProviderIamPtrOutputWithContext(context.Context) ProviderIamPtrOutput
}

type providerIamPtrType ProviderIamArgs

func ProviderIamPtr(v *ProviderIamArgs) ProviderIamPtrInput {
	return (*providerIamPtrType)(v)
}

func (*providerIamPtrType) ElementType() reflect.Type {
	return reflect.TypeOf((**ProviderIam)(nil)).Elem()
}

func (i *providerIamPtrType) ToProviderIamPtrOutput() ProviderIamPtrOutput {
	return i.ToProviderIamPtrOutputWithContext(context.Background())
}

func (i *providerIamPtrType) ToProviderIamPtrOutputWithContext(ctx context.Context) ProviderIamPtrOutput {
	return pulumi.ToOutputWithContext(ctx, i).(ProviderIamPtrOutput)
}

type ProviderIamOutput struct{ *pulumi.OutputState }

func (ProviderIamOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*ProviderIam)(nil)).Elem()
}

func (o ProviderIamOutput) ToProviderIamOutput() ProviderIamOutput {
	return o
}

func (o ProviderIamOutput) ToProviderIamOutputWithContext(ctx context.Context) ProviderIamOutput {
	return o
}

func (o ProviderIamOutput) ToProviderIamPtrOutput() ProviderIamPtrOutput {
	return o.ToProviderIamPtrOutputWithContext(context.Background())
}

func (o ProviderIamOutput) ToProviderIamPtrOutputWithContext(ctx context.Context) ProviderIamPtrOutput {
	return o.ApplyT(func(v ProviderIam) *ProviderIam {
		return &v
	}).(ProviderIamPtrOutput)
}
func (o ProviderIamOutput) Role() ProviderIamRolePtrOutput {
	return o.ApplyT(func(v ProviderIam) *ProviderIamRole { return v.Role }).(ProviderIamRolePtrOutput)
}

type ProviderIamPtrOutput struct{ *pulumi.OutputState }

func (ProviderIamPtrOutput) ElementType() reflect.Type {
	return reflect.TypeOf((**ProviderIam)(nil)).Elem()
}

func (o ProviderIamPtrOutput) ToProviderIamPtrOutput() ProviderIamPtrOutput {
	return o
}

func (o ProviderIamPtrOutput) ToProviderIamPtrOutputWithContext(ctx context.Context) ProviderIamPtrOutput {
	return o
}

func (o ProviderIamPtrOutput) Elem() ProviderIamOutput {
	return o.ApplyT(func(v *ProviderIam) ProviderIam { return *v }).(ProviderIamOutput)
}

func (o ProviderIamPtrOutput) Role() ProviderIamRolePtrOutput {
	return o.ApplyT(func(v *ProviderIam) *ProviderIamRole {
		if v == nil {
			return nil
		}
		return v.Role
	}).(ProviderIamRolePtrOutput)
}

type ProviderIamRole struct {
	ManagedPolicies []string `pulumi:"managedPolicies,optional"`
}

// ProviderIamRoleInput is an input type that accepts ProviderIamRoleArgs and ProviderIamRoleOutput values.
// You can construct a concrete instance of `ProviderIamRoleInput` via:
//
//          ProviderIamRoleArgs{...}
type ProviderIamRoleInput interface {
	pulumi.Input

	ToProviderIamRoleOutput() ProviderIamRoleOutput
	ToProviderIamRoleOutputWithContext(context.Context) ProviderIamRoleOutput
}

type ProviderIamRoleArgs struct {
	ManagedPolicies pulumi.StringArrayInput `pulumi:"managedPolicies"`
}

func (ProviderIamRoleArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*ProviderIamRole)(nil)).Elem()
}

func (i ProviderIamRoleArgs) ToProviderIamRoleOutput() ProviderIamRoleOutput {
	return i.ToProviderIamRoleOutputWithContext(context.Background())
}

func (i ProviderIamRoleArgs) ToProviderIamRoleOutputWithContext(ctx context.Context) ProviderIamRoleOutput {
	return pulumi.ToOutputWithContext(ctx, i).(ProviderIamRoleOutput)
}

func (i ProviderIamRoleArgs) ToProviderIamRolePtrOutput() ProviderIamRolePtrOutput {
	return i.ToProviderIamRolePtrOutputWithContext(context.Background())
}

func (i ProviderIamRoleArgs) ToProviderIamRolePtrOutputWithContext(ctx context.Context) ProviderIamRolePtrOutput {
	return pulumi.ToOutputWithContext(ctx, i).(ProviderIamRoleOutput).ToProviderIamRolePtrOutputWithContext(ctx)
}

// ProviderIamRolePtrInput is an input type that accepts ProviderIamRoleArgs, ProviderIamRolePtr and ProviderIamRolePtrOutput values.
// You can construct a concrete instance of `ProviderIamRolePtrInput` via:
//
//          ProviderIamRoleArgs{...}
//
//  or:
//
//          nil
type ProviderIamRolePtrInput interface {
	pulumi.Input

	ToProviderIamRolePtrOutput() ProviderIamRolePtrOutput
	ToProviderIamRolePtrOutputWithContext(context.Context) ProviderIamRolePtrOutput
}

type providerIamRolePtrType ProviderIamRoleArgs

func ProviderIamRolePtr(v *ProviderIamRoleArgs) ProviderIamRolePtrInput {
	return (*providerIamRolePtrType)(v)
}

func (*providerIamRolePtrType) ElementType() reflect.Type {
	return reflect.TypeOf((**ProviderIamRole)(nil)).Elem()
}

func (i *providerIamRolePtrType) ToProviderIamRolePtrOutput() ProviderIamRolePtrOutput {
	return i.ToProviderIamRolePtrOutputWithContext(context.Background())
}

func (i *providerIamRolePtrType) ToProviderIamRolePtrOutputWithContext(ctx context.Context) ProviderIamRolePtrOutput {
	return pulumi.ToOutputWithContext(ctx, i).(ProviderIamRolePtrOutput)
}

type ProviderIamRoleOutput struct{ *pulumi.OutputState }

func (ProviderIamRoleOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*ProviderIamRole)(nil)).Elem()
}

func (o ProviderIamRoleOutput) ToProviderIamRoleOutput() ProviderIamRoleOutput {
	return o
}

func (o ProviderIamRoleOutput) ToProviderIamRoleOutputWithContext(ctx context.Context) ProviderIamRoleOutput {
	return o
}

func (o ProviderIamRoleOutput) ToProviderIamRolePtrOutput() ProviderIamRolePtrOutput {
	return o.ToProviderIamRolePtrOutputWithContext(context.Background())
}

func (o ProviderIamRoleOutput) ToProviderIamRolePtrOutputWithContext(ctx context.Context) ProviderIamRolePtrOutput {
	return o.ApplyT(func(v ProviderIamRole) *ProviderIamRole {
		return &v
	}).(ProviderIamRolePtrOutput)
}
func (o ProviderIamRoleOutput) ManagedPolicies() pulumi.StringArrayOutput {
	return o.ApplyT(func(v ProviderIamRole) []string { return v.ManagedPolicies }).(pulumi.StringArrayOutput)
}

type ProviderIamRolePtrOutput struct{ *pulumi.OutputState }

func (ProviderIamRolePtrOutput) ElementType() reflect.Type {
	return reflect.TypeOf((**ProviderIamRole)(nil)).Elem()
}

func (o ProviderIamRolePtrOutput) ToProviderIamRolePtrOutput() ProviderIamRolePtrOutput {
	return o
}

func (o ProviderIamRolePtrOutput) ToProviderIamRolePtrOutputWithContext(ctx context.Context) ProviderIamRolePtrOutput {
	return o
}

func (o ProviderIamRolePtrOutput) Elem() ProviderIamRoleOutput {
	return o.ApplyT(func(v *ProviderIamRole) ProviderIamRole { return *v }).(ProviderIamRoleOutput)
}

func (o ProviderIamRolePtrOutput) ManagedPolicies() pulumi.StringArrayOutput {
	return o.ApplyT(func(v *ProviderIamRole) []string {
		if v == nil {
			return nil
		}
		return v.ManagedPolicies
	}).(pulumi.StringArrayOutput)
}

type ServiceProvider struct {
	Iam     *ProviderIam `pulumi:"iam,optional"`
	Name    *string      `pulumi:"name,optional"`
	Runtime *string      `pulumi:"runtime,optional"`
}

// ServiceProviderInput is an input type that accepts ServiceProviderArgs and ServiceProviderOutput values.
// You can construct a concrete instance of `ServiceProviderInput` via:
//
//          ServiceProviderArgs{...}
type ServiceProviderInput interface {
	pulumi.Input

	ToServiceProviderOutput() ServiceProviderOutput
	ToServiceProviderOutputWithContext(context.Context) ServiceProviderOutput
}

type ServiceProviderArgs struct {
	Iam     ProviderIamPtrInput   `pulumi:"iam"`
	Name    pulumi.StringPtrInput `pulumi:"name"`
	Runtime pulumi.StringPtrInput `pulumi:"runtime"`
}

func (ServiceProviderArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*ServiceProvider)(nil)).Elem()
}

func (i ServiceProviderArgs) ToServiceProviderOutput() ServiceProviderOutput {
	return i.ToServiceProviderOutputWithContext(context.Background())
}

func (i ServiceProviderArgs) ToServiceProviderOutputWithContext(ctx context.Context) ServiceProviderOutput {
	return pulumi.ToOutputWithContext(ctx, i).(ServiceProviderOutput)
}

func (i ServiceProviderArgs) ToServiceProviderPtrOutput() ServiceProviderPtrOutput {
	return i.ToServiceProviderPtrOutputWithContext(context.Background())
}

func (i ServiceProviderArgs) ToServiceProviderPtrOutputWithContext(ctx context.Context) ServiceProviderPtrOutput {
	return pulumi.ToOutputWithContext(ctx, i).(ServiceProviderOutput).ToServiceProviderPtrOutputWithContext(ctx)
}

// ServiceProviderPtrInput is an input type that accepts ServiceProviderArgs, ServiceProviderPtr and ServiceProviderPtrOutput values.
// You can construct a concrete instance of `ServiceProviderPtrInput` via:
//
//          ServiceProviderArgs{...}
//
//  or:
//
//          nil
type ServiceProviderPtrInput interface {
	pulumi.Input

	ToServiceProviderPtrOutput() ServiceProviderPtrOutput
	ToServiceProviderPtrOutputWithContext(context.Context) ServiceProviderPtrOutput
}

type serviceProviderPtrType ServiceProviderArgs

func ServiceProviderPtr(v *ServiceProviderArgs) ServiceProviderPtrInput {
	return (*serviceProviderPtrType)(v)
}

func (*serviceProviderPtrType) ElementType() reflect.Type {
	return reflect.TypeOf((**ServiceProvider)(nil)).Elem()
}

func (i *serviceProviderPtrType) ToServiceProviderPtrOutput() ServiceProviderPtrOutput {
	return i.ToServiceProviderPtrOutputWithContext(context.Background())
}

func (i *serviceProviderPtrType) ToServiceProviderPtrOutputWithContext(ctx context.Context) ServiceProviderPtrOutput {
	return pulumi.ToOutputWithContext(ctx, i).(ServiceProviderPtrOutput)
}

type ServiceProviderOutput struct{ *pulumi.OutputState }

func (ServiceProviderOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*ServiceProvider)(nil)).Elem()
}

func (o ServiceProviderOutput) ToServiceProviderOutput() ServiceProviderOutput {
	return o
}

func (o ServiceProviderOutput) ToServiceProviderOutputWithContext(ctx context.Context) ServiceProviderOutput {
	return o
}

func (o ServiceProviderOutput) ToServiceProviderPtrOutput() ServiceProviderPtrOutput {
	return o.ToServiceProviderPtrOutputWithContext(context.Background())
}

func (o ServiceProviderOutput) ToServiceProviderPtrOutputWithContext(ctx context.Context) ServiceProviderPtrOutput {
	return o.ApplyT(func(v ServiceProvider) *ServiceProvider {
		return &v
	}).(ServiceProviderPtrOutput)
}
func (o ServiceProviderOutput) Iam() ProviderIamPtrOutput {
	return o.ApplyT(func(v ServiceProvider) *ProviderIam { return v.Iam }).(ProviderIamPtrOutput)
}

func (o ServiceProviderOutput) Name() pulumi.StringPtrOutput {
	return o.ApplyT(func(v ServiceProvider) *string { return v.Name }).(pulumi.StringPtrOutput)
}

func (o ServiceProviderOutput) Runtime() pulumi.StringPtrOutput {
	return o.ApplyT(func(v ServiceProvider) *string { return v.Runtime }).(pulumi.StringPtrOutput)
}

type ServiceProviderPtrOutput struct{ *pulumi.OutputState }

func (ServiceProviderPtrOutput) ElementType() reflect.Type {
	return reflect.TypeOf((**ServiceProvider)(nil)).Elem()
}

func (o ServiceProviderPtrOutput) ToServiceProviderPtrOutput() ServiceProviderPtrOutput {
	return o
}

func (o ServiceProviderPtrOutput) ToServiceProviderPtrOutputWithContext(ctx context.Context) ServiceProviderPtrOutput {
	return o
}

func (o ServiceProviderPtrOutput) Elem() ServiceProviderOutput {
	return o.ApplyT(func(v *ServiceProvider) ServiceProvider { return *v }).(ServiceProviderOutput)
}

func (o ServiceProviderPtrOutput) Iam() ProviderIamPtrOutput {
	return o.ApplyT(func(v *ServiceProvider) *ProviderIam {
		if v == nil {
			return nil
		}
		return v.Iam
	}).(ProviderIamPtrOutput)
}

func (o ServiceProviderPtrOutput) Name() pulumi.StringPtrOutput {
	return o.ApplyT(func(v *ServiceProvider) *string {
		if v == nil {
			return nil
		}
		return v.Name
	}).(pulumi.StringPtrOutput)
}

func (o ServiceProviderPtrOutput) Runtime() pulumi.StringPtrOutput {
	return o.ApplyT(func(v *ServiceProvider) *string {
		if v == nil {
			return nil
		}
		return v.Runtime
	}).(pulumi.StringPtrOutput)
}

type SqsEvent struct {
	Arn *string `pulumi:"arn,optional"`
}

// SqsEventInput is an input type that accepts SqsEventArgs and SqsEventOutput values.
// You can construct a concrete instance of `SqsEventInput` via:
//
//          SqsEventArgs{...}
type SqsEventInput interface {
	pulumi.Input

	ToSqsEventOutput() SqsEventOutput
	ToSqsEventOutputWithContext(context.Context) SqsEventOutput
}

type SqsEventArgs struct {
	Arn pulumi.StringPtrInput `pulumi:"arn"`
}

func (SqsEventArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*SqsEvent)(nil)).Elem()
}

func (i SqsEventArgs) ToSqsEventOutput() SqsEventOutput {
	return i.ToSqsEventOutputWithContext(context.Background())
}

func (i SqsEventArgs) ToSqsEventOutputWithContext(ctx context.Context) SqsEventOutput {
	return pulumi.ToOutputWithContext(ctx, i).(SqsEventOutput)
}

func (i SqsEventArgs) ToSqsEventPtrOutput() SqsEventPtrOutput {
	return i.ToSqsEventPtrOutputWithContext(context.Background())
}

func (i SqsEventArgs) ToSqsEventPtrOutputWithContext(ctx context.Context) SqsEventPtrOutput {
	return pulumi.ToOutputWithContext(ctx, i).(SqsEventOutput).ToSqsEventPtrOutputWithContext(ctx)
}

// SqsEventPtrInput is an input type that accepts SqsEventArgs, SqsEventPtr and SqsEventPtrOutput values.
// You can construct a concrete instance of `SqsEventPtrInput` via:
//
//          SqsEventArgs{...}
//
//  or:
//
//          nil
type SqsEventPtrInput interface {
	pulumi.Input

	ToSqsEventPtrOutput() SqsEventPtrOutput
	ToSqsEventPtrOutputWithContext(context.Context) SqsEventPtrOutput
}

type sqsEventPtrType SqsEventArgs

func SqsEventPtr(v *SqsEventArgs) SqsEventPtrInput {
	return (*sqsEventPtrType)(v)
}

func (*sqsEventPtrType) ElementType() reflect.Type {
	return reflect.TypeOf((**SqsEvent)(nil)).Elem()
}

func (i *sqsEventPtrType) ToSqsEventPtrOutput() SqsEventPtrOutput {
	return i.ToSqsEventPtrOutputWithContext(context.Background())
}

func (i *sqsEventPtrType) ToSqsEventPtrOutputWithContext(ctx context.Context) SqsEventPtrOutput {
	return pulumi.ToOutputWithContext(ctx, i).(SqsEventPtrOutput)
}

type SqsEventOutput struct{ *pulumi.OutputState }

func (SqsEventOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*SqsEvent)(nil)).Elem()
}

func (o SqsEventOutput) ToSqsEventOutput() SqsEventOutput {
	return o
}

func (o SqsEventOutput) ToSqsEventOutputWithContext(ctx context.Context) SqsEventOutput {
	return o
}

func (o SqsEventOutput) ToSqsEventPtrOutput() SqsEventPtrOutput {
	return o.ToSqsEventPtrOutputWithContext(context.Background())
}

func (o SqsEventOutput) ToSqsEventPtrOutputWithContext(ctx context.Context) SqsEventPtrOutput {
	return o.ApplyT(func(v SqsEvent) *SqsEvent {
		return &v
	}).(SqsEventPtrOutput)
}
func (o SqsEventOutput) Arn() pulumi.StringPtrOutput {
	return o.ApplyT(func(v SqsEvent) *string { return v.Arn }).(pulumi.StringPtrOutput)
}

type SqsEventPtrOutput struct{ *pulumi.OutputState }

func (SqsEventPtrOutput) ElementType() reflect.Type {
	return reflect.TypeOf((**SqsEvent)(nil)).Elem()
}

func (o SqsEventPtrOutput) ToSqsEventPtrOutput() SqsEventPtrOutput {
	return o
}

func (o SqsEventPtrOutput) ToSqsEventPtrOutputWithContext(ctx context.Context) SqsEventPtrOutput {
	return o
}

func (o SqsEventPtrOutput) Elem() SqsEventOutput {
	return o.ApplyT(func(v *SqsEvent) SqsEvent { return *v }).(SqsEventOutput)
}

func (o SqsEventPtrOutput) Arn() pulumi.StringPtrOutput {
	return o.ApplyT(func(v *SqsEvent) *string {
		if v == nil {
			return nil
		}
		return v.Arn
	}).(pulumi.StringPtrOutput)
}

func init() {
	pulumi.RegisterOutputType(EventOutput{})
	pulumi.RegisterOutputType(EventArrayOutput{})
	pulumi.RegisterOutputType(FunctionOutput{})
	pulumi.RegisterOutputType(FunctionMapOutput{})
	pulumi.RegisterOutputType(HttpEventOutput{})
	pulumi.RegisterOutputType(HttpEventPtrOutput{})
	pulumi.RegisterOutputType(ProviderIamOutput{})
	pulumi.RegisterOutputType(ProviderIamPtrOutput{})
	pulumi.RegisterOutputType(ProviderIamRoleOutput{})
	pulumi.RegisterOutputType(ProviderIamRolePtrOutput{})
	pulumi.RegisterOutputType(ServiceProviderOutput{})
	pulumi.RegisterOutputType(ServiceProviderPtrOutput{})
	pulumi.RegisterOutputType(SqsEventOutput{})
	pulumi.RegisterOutputType(SqsEventPtrOutput{})

	pulumi.RegisterOutputType(ServiceOutput{})
	pulumi.RegisterOutputType(ServicePtrOutput{})
	pulumi.RegisterOutputType(ServiceArrayOutput{})
	pulumi.RegisterOutputType(ServiceMapOutput{})
}
