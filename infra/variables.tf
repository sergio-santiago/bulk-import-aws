variable "project" {
  description = "Project name used as prefix and tag for all resources."
  type        = string
  default     = "bulk-import-aws"
}

variable "region" {
  description = "AWS region for the stack."
  type        = string
  default     = "eu-west-1"
}

variable "environment" {
  description = "Logical environment used in tags. Single env in this MVP."
  type        = string
  default     = "prod"
}
