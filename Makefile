AWS_REGION ?= eu-west-1
AWS_PROFILE ?= lab-sergio
PROJECT    ?= bulk-import-aws

LAMBDAS    := api parser worker
DIST_DIR   := dist
GO_IMAGE   := golang:1.24

export AWS_PROFILE
export AWS_REGION

.PHONY: build package fmt tf-init tf-plan tf-apply tf-destroy deploy-web clean

## Compile Go Lambda binaries for linux/arm64 inside a Docker container
build:
	@for fn in $(LAMBDAS); do \
		echo "==> building $$fn"; \
		docker run --rm \
			-v "$$PWD/src/$$fn":/src \
			-w /src \
			-e GOOS=linux \
			-e GOARCH=arm64 \
			-e CGO_ENABLED=0 \
			$(GO_IMAGE) \
			sh -c "go mod tidy && go build -o bootstrap ." || exit 1; \
	done

## Zip Lambda binaries into dist/ ready for deployment
package: build
	@mkdir -p $(DIST_DIR)
	@for fn in $(LAMBDAS); do \
		echo "==> packaging $$fn"; \
		rm -f $(DIST_DIR)/$$fn.zip; \
		(cd src/$$fn && zip -q -j ../../$(DIST_DIR)/$$fn.zip bootstrap); \
	done

## Format Terraform and Go sources
fmt:
	cd infra && terraform fmt -recursive
	cd infra/bootstrap && terraform fmt -recursive
	@for fn in $(LAMBDAS); do \
		docker run --rm -v "$$PWD/src/$$fn":/src -w /src $(GO_IMAGE) gofmt -w . ; \
	done

## Initialise Terraform with the remote backend
tf-init:
	cd infra && terraform init -input=false

## Show the Terraform plan for the current stack
tf-plan:
	cd infra && terraform plan -input=false

## Apply the Terraform plan
tf-apply:
	cd infra && terraform apply -input=false -auto-approve

## Destroy the Terraform-managed stack
tf-destroy:
	cd infra && terraform destroy -input=false -auto-approve

## Invalidate the CloudFront cache so the new index.html is served immediately
deploy-web:
	@DIST_ID=$$(cd infra && terraform output -raw cloudfront_distribution_id); \
	aws cloudfront create-invalidation --distribution-id $$DIST_ID --paths "/*"

## Remove local build artefacts (zips and compiled binaries)
clean:
	@rm -rf $(DIST_DIR)
	@for fn in $(LAMBDAS); do \
		rm -f src/$$fn/bootstrap; \
	done
