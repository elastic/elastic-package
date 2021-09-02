#!sh

set -euxo pipefail

cp -r /stage/*.tf /workspace

cleanup() {
  r=$?

  set -x
  terraform destroy -auto-approve

  exit $r
}
trap cleanup EXIT INT TERM

terraform init
terraform plan
terraform apply -auto-approve && touch /tmp/tf-applied

echo "Terraform definitions applied."

set +x
while true; do sleep 1; done # wait for ctrl-c
