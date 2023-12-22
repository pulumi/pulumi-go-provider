default: export PULUMI_CONFIG_PASSPHRASE = ""

default:
	rm -f grpc.json
	mkdir -p state
	pulumi login --cloud-url "file://$(CURDIR)/state"
	pulumi stack select organization/test/test --create
	PULUMI_DEBUG_GRPC=grpc.json pulumi up       --yes
								pulumi destroy  --yes
								pulumi stack rm --yes
	pulumi logout
	rm -r "$(CURDIR)/state"
