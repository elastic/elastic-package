resource "local_file" "log" {
  source          = "./files/example.log"
  filename        = "/tmp/service_logs/${var.TEST_RUN_ID}.log"
  file_permission = "0777"
}

output "filename" {
    value = local_file.log.filename 
}
