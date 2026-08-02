AWS_REGION ?= eu-west-1
AWS_PROFILE ?= lab-sergio
PROJECT    ?= bulk-import-aws

# Remote state bucket, created by infra/bootstrap. Its name embeds the account
# id, so it is resolved at run time rather than committed. Override to target
# another account.
TF_STATE_BUCKET ?= $(shell cd infra/bootstrap 2>/dev/null && terraform output -raw state_bucket 2>/dev/null)

LAMBDAS    := api parser worker
DIST_DIR   := dist
GO_IMAGE   := golang:1.24

export AWS_PROFILE
export AWS_REGION

.PHONY: build test package fmt tf-init tf-plan tf-apply tf-destroy deploy-web clean

## Run the Go unit tests for every Lambda
#
# The whole repository is mounted rather than just the module, because the
# parser's tests read the CSVs in samples/ and assert on what they parse to.
#
# Each package builds its AWS clients in init(), which runs before any test
# does. Pinning the region and switching off the instance metadata probe keeps
# that from waiting on a lookup that cannot succeed off an EC2 instance. No test
# here talks to AWS.
test:
	@for fn in $(LAMBDAS); do \
		echo "==> testing $$fn"; \
		docker run --rm \
			-v "$$PWD":/repo \
			-w /repo/src/$$fn \
			-e AWS_REGION=$(AWS_REGION) \
			-e AWS_EC2_METADATA_DISABLED=true \
			$(GO_IMAGE) \
			go test ./... || exit 1; \
	done

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
	@test -n "$(TF_STATE_BUCKET)" || { \
		echo "TF_STATE_BUCKET is empty. Apply infra/bootstrap first, or set it explicitly."; \
		exit 1; \
	}
	cd infra && terraform init -input=false -backend-config="bucket=$(TF_STATE_BUCKET)"

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
