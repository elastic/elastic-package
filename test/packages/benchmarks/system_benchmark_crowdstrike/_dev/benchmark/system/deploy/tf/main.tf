resource "local_file" "benchmark_log" {
  source          = "./files/sample.log"
  filename        = "/tmp/service_logs/tf-benchmark-${var.TEST_RUN_ID}.log"
  file_permission = "0777"
}
