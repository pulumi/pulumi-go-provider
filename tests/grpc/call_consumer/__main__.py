# Copyright 2024, Pulumi Corporation.  All rights reserved.

import pulumi
import pulumi_test as test

c = test.Component(resource_name="my-component", my_input="foo")

c.my_method(arg1="bar")
