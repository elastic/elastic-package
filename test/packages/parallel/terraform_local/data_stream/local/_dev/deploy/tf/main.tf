resource "local_file" "log" {
  source          = "./files/example.log"
  filename        = "/tmp/service_logs/file.log"
  file_permission = "0777"
}

locals {
  items ={
    environment  = "${var.ENVIRONMENT}"
    repo         = "${var.REPO}"
    branch       = "${var.BRANCH}"
    build        = "${var.BUILD_ID}"
    created_date = "${var.CREATED_DATE}"
  }
}

resource "local_file" "log_variables" {
  content         = format("%s\n", jsonencode(local.items))
  filename        = "/tmp/service_logs/file_vars.log"
  file_permission = "0777"
}
