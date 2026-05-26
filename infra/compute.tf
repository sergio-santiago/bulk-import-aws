locals {
  lambdas = {
    api = {
      environment = {
        IMPORTS_TABLE        = aws_dynamodb_table.imports.name
        IMPORT_RECORDS_TABLE = aws_dynamodb_table.import_records.name
        UPLOADS_BUCKET       = aws_s3_bucket.uploads.id
      }
    }
    parser = {
      environment = {
        IMPORTS_TABLE     = aws_dynamodb_table.imports.name
        RECORDS_QUEUE_URL = aws_sqs_queue.records.url
      }
    }
    worker = {
      environment = {
        IMPORTS_TABLE        = aws_dynamodb_table.imports.name
        IMPORT_RECORDS_TABLE = aws_dynamodb_table.import_records.name
      }
    }
  }
}

# Explicit log groups so retention is enforced. Otherwise Lambda creates
# them implicitly on first invocation with infinite retention.
resource "aws_cloudwatch_log_group" "lambda" {
  for_each = local.lambdas

  name              = "/aws/lambda/${local.name_prefix}-${each.key}"
  retention_in_days = 14
}

resource "aws_lambda_function" "this" {
  for_each = local.lambdas

  function_name = "${local.name_prefix}-${each.key}"
  role          = local.lambda_execution_role_arn

  # Packaged zips are produced by `make package` and live in dist/. The hash
  # forces Terraform to detect rebuilds.
  filename         = "${path.module}/../dist/${each.key}.zip"
  source_code_hash = filebase64sha256("${path.module}/../dist/${each.key}.zip")

  handler       = "bootstrap"
  runtime       = "provided.al2023"
  architectures = ["arm64"]
  memory_size   = 256
  timeout       = 30

  environment {
    variables = each.value.environment
  }

  depends_on = [aws_cloudwatch_log_group.lambda]
}
