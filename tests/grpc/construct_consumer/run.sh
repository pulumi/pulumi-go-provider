#!/usr/bin/env sh

set -euf

cd ../provider && go build -o ../construct_consumer/pulumi-resource-test && cd -

export PULUMI_CONFIG_PASSPHRASE=""

mkdir -p state
pulumi login --cloud-url "file://$PWD/state"
pulumi stack select organization/test/test --create
PULUMI_DEBUG_GRPC=grpc.json pulumi up       --yes
                            pulumi destroy  --yes
                            pulumi stack rm --yes
pulumi logout
rm -rf "$PWD/state"
