terraform {
  backend "s3" {
    # bucket is intentionally absent: the name embeds the AWS account id, so it
    # is passed at init time instead of being committed. See the Makefile's
    # tf-init target, which reads it from the bootstrap stack's output.
    key          = "infra/terraform.tfstate"
    region       = "eu-west-1"
    encrypt      = true
    use_lockfile = true
  }
}
