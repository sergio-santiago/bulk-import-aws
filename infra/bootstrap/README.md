# infra/bootstrap

One-off bootstrap for `bulk-import-aws`. Creates the S3 bucket that holds
the remote Terraform state for the main stack. State locking is handled
natively by S3 via `use_lockfile`, so there is no separate DynamoDB lock
table.

This stack uses **local** state (the remote bucket does not exist yet when
it is first applied). The resulting `terraform.tfstate` file lives in this
directory and is ignored by git. Keep a copy somewhere safe if you ever
want to destroy the bootstrap.

## Lab account constraints

The project deploys against a sandbox AWS account with restricted IAM:
roles, users, OIDC providers and access keys cannot be created. As a
consequence the bootstrap deliberately does **not** provision a GitHub
Actions role or OIDC configuration. Deploys are operated locally with the
SSO session and CI is limited to validation and linting. See the root
README for the full picture.

The bucket name is `<project>-tfstate-<account-id>`, built from the caller
identity, and exposed as the `state_bucket` output. The main stack does not
commit that name: `make tf-init` reads this output and passes it through
`terraform init -backend-config`.

## Prerequisites

- Terraform >= 1.6.
- AWS CLI with an active SSO profile pointing at the target account.

## Apply

```bash
export AWS_PROFILE=lab-sergio

cd infra/bootstrap
terraform init
terraform plan
terraform apply
```

## Destroy

```bash
terraform destroy
```

If the state bucket has objects (the main stack's state), empty it first.
With an empty bucket, `destroy` removes everything cleanly.
