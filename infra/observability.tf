resource "aws_sns_topic" "alerts" {
  name = "${local.name_prefix}-alerts"
}

# Email subscription. AWS sends a confirmation link to this address on
# creation; until clicked, the subscription stays in PendingConfirmation
# and no alerts are delivered.
resource "aws_sns_topic_subscription" "alerts_email" {
  topic_arn = aws_sns_topic.alerts.arn
  protocol  = "email"
  endpoint  = "sersanhen@gmail.com"
}

# Fires as soon as anything lands in the dead-letter queue. With
# TreatMissingData = notBreaching we avoid noise when the queue is empty
# and SQS reports no metric data.
resource "aws_cloudwatch_metric_alarm" "dlq_not_empty" {
  alarm_name        = "${local.name_prefix}-dlq-not-empty"
  alarm_description = "One or more records failed processing after retries and reached the DLQ."

  namespace   = "AWS/SQS"
  metric_name = "ApproximateNumberOfMessagesVisible"
  statistic   = "Maximum"

  dimensions = {
    QueueName = aws_sqs_queue.records_dlq.name
  }

  period              = 60
  evaluation_periods  = 1
  threshold           = 0
  comparison_operator = "GreaterThanThreshold"
  treat_missing_data  = "notBreaching"

  alarm_actions = [aws_sns_topic.alerts.arn]
  ok_actions    = [aws_sns_topic.alerts.arn]
}

resource "aws_budgets_budget" "monthly" {
  name         = local.name_prefix
  budget_type  = "COST"
  limit_amount = "10"
  limit_unit   = "USD"
  time_unit    = "MONTHLY"

  notification {
    comparison_operator        = "GREATER_THAN"
    threshold                  = 50
    threshold_type             = "PERCENTAGE"
    notification_type          = "ACTUAL"
    subscriber_email_addresses = ["sersanhen@gmail.com"]
  }

  notification {
    comparison_operator        = "GREATER_THAN"
    threshold                  = 80
    threshold_type             = "PERCENTAGE"
    notification_type          = "ACTUAL"
    subscriber_email_addresses = ["sersanhen@gmail.com"]
  }

  notification {
    comparison_operator        = "GREATER_THAN"
    threshold                  = 100
    threshold_type             = "PERCENTAGE"
    notification_type          = "ACTUAL"
    subscriber_email_addresses = ["sersanhen@gmail.com"]
  }
}
