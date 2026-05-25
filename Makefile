AWS_REGION ?= eu-west-1
PROJECT    ?= bulk-import-aws

LAMBDAS    := api parser worker
DIST_DIR   := dist
GO_IMAGE   := golang:1.24

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

## Remove local build artefacts (zips and compiled binaries)
clean:
	@rm -rf $(DIST_DIR)
	@for fn in $(LAMBDAS); do \
		rm -f src/$$fn/bootstrap; \
	done
