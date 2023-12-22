default:
	go build -o pulumi-resource-config
	$(MAKE) -C consumer
	pulumi package gen-sdk --language nodejs schema-config.json
	$(MAKE) -C consumer/ts
