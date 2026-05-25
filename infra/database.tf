resource "aws_dynamodb_table" "imports" {
  name         = "${local.name_prefix}-imports"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "import_id"

  attribute {
    name = "import_id"
    type = "S"
  }

  attribute {
    name = "file_hash"
    type = "S"
  }

  # GSI used to detect duplicate uploads of the same file. The parser does a
  # conditional write keyed by file_hash to enforce idempotency.
  global_secondary_index {
    name            = "file_hash-index"
    hash_key        = "file_hash"
    projection_type = "ALL"
  }

  point_in_time_recovery {
    enabled = true
  }
}

resource "aws_dynamodb_table" "import_records" {
  name         = "${local.name_prefix}-import-records"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "import_id"
  range_key    = "record_id"

  attribute {
    name = "import_id"
    type = "S"
  }

  attribute {
    name = "record_id"
    type = "S"
  }

  point_in_time_recovery {
    enabled = true
  }
}
