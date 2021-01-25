variable "test_run_id" {
  default = "detached"
}

provider "aws" {}

resource "aws_instance" "i" {
  ami           = data.aws_ami.latest-amzn.id
  instance_type = "t1.micro"
  tags = {
    Name = "elastic-package-test-${var.test_run_id}"
  }
}

data "aws_ami" "latest-amzn" {
  most_recent = true
  owners = [ "amazon" ] # AWS
  filter {
    name   = "name"
    values = ["amzn2-ami-hvm-*"]
  }
}