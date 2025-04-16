# Copyright 2024, Pulumi Corporation.  All rights reserved.

import pulumi
import pulumi_test as test

c = test.Component(resource_name="my-component", my_input="foo")
pulumi.export("my_output", c.my_output)

result = c.my_method(arg1="bar")
pulumi.export("resp1", result.resp1)