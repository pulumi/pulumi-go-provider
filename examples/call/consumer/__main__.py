import pulumi_test as test

p = test.Provider("explicit-provider")

p.make_pet()
