resource "local_file" "log" {
  source          = "./files/example.log"
  filename        = "/tmp/service_logs/file.log"
  file_permission = "0777"
}
