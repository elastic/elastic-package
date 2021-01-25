provider "aws" {}

resource "random_string" "id" {
  length = 5
  upper   = false
  lower   = true
  number  = false
  special = false
}

resource "aws_instance" "i" {
  ami           = "${data.aws_ami.latest-amzn.id}"
  instance_type = "t1.micro"
  tags = {
    Name = "elastic-package-test-${random_string.id.result}"
  }
}

data "aws_ami" "latest-amzn" {
  most_recent = true
  filter {
    name   = "name"
    values = ["amzn2-ami-hvm-*"]
  }
}