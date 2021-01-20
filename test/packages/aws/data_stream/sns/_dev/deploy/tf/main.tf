provider "aws" {}

resource "random_string" "id" {
  length = 5
  upper   = false
  lower   = true
  number  = false
  special = false
}

resource "aws_sns_topic" "t" {
  name = "ep-sns-topic-${random_string.id.result}"
}