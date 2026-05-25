AWS_REGION ?= eu-west-1
PROJECT    ?= bulk-import-aws

.PHONY: build package fmt tf-init tf-plan tf-apply tf-destroy deploy-web clean

## Compile Go Lambda binaries for linux/arm64
build:
	@:

## Zip Lambda binaries ready for deployment
package:
	@:

## Format Go and Terraform sources
fmt:
	@:

## Initialise Terraform with the remote backend
tf-init:
	@:

## Show the Terraform plan for the current stack
tf-plan:
	@:

## Apply the Terraform plan
tf-apply:
	@:

## Destroy the Terraform-managed stack
tf-destroy:
	@:

## Upload the static frontend to S3 and invalidate CloudFront
deploy-web:
	@:

## Remove local build artefacts
clean:
	@:
