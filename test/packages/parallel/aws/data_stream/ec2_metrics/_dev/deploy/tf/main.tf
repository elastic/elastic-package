variable "TEST_RUN_ID" {
  default = "detached"
}

provider "aws" {
  default_tags {
    tags = {
      run_id       = var.TEST_RUN_ID
      environment  = var.ENVIRONMENT
      owner        = var.OWNER
      branch       = var.BRANCH_NAME
      build        = var.BUILD_ID
      created_date = var.CREATED_DATE
      created_date_time = var.CREATED_DATE_TIME
    }
  }
}

resource "aws_instance" "i" {
  ami           = data.aws_ami.latest-amzn.id
  monitoring = true
  instance_type = "t1.micro"
  tags = {
    Name = "elastic-package-test-${var.TEST_RUN_ID}"
  }
}

data "aws_ami" "latest-amzn" {
  most_recent = true
  owners = [ "amazon" ] # AWS
  filter {
    name   = "name"
    values = ["amzn2-ami-minimal-hvm-*-ebs"]
  }
}
