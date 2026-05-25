terraform {
  backend "s3" {
    bucket       = "bulk-import-aws-tfstate-883099621748"
    key          = "infra/terraform.tfstate"
    region       = "eu-west-1"
    encrypt      = true
    use_lockfile = true
  }
}
