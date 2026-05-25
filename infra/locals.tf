locals {
  account_id = data.aws_caller_identity.current.account_id
  partition  = data.aws_partition.current.partition

  name_prefix = var.project

  # Shared Lambda execution role pre-created in the lab environment.
  # All project Lambdas reuse it; S3 access is granted via bucket policies.
  lambda_execution_role_arn = "arn:${local.partition}:iam::${local.account_id}:role/studentLambdaExecutionRole"
}
