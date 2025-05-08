
.PHONY: generate
generate:
	pulumi package get-schema . > schema.json
	pulumi package gen-sdk ./schema.json --language go