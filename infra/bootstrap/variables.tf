variable "project" {
  description = "Project name used as prefix for all resources."
  type        = string
  default     = "bulk-import-aws"
}

variable "region" {
  description = "AWS region for the bootstrap resources."
  type        = string
  default     = "eu-west-1"
}
