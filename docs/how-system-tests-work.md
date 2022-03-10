# How system tests work

This file describe the framework around system tests and it's different pieces and flow.

Topics:

- env vars
  - Jenkinsfile env variables
    - docker envs/tf env/kubernetes(?) (link to how to)
      - test case in `test-default-config.yml`
      - TF_VAR filter https://github.com/elastic/elastic-package/blob/ad7db0a7492a0b49fe0f98f832ead9cbd362fbc9/internal/testrunner/runners/system/servicedeployer/terraform_env.go#L44
- adding GCP envs or the Jenkins shared library => https://github.com/elastic/apm-pipeline-library/tree/main/vars
- stack version used by tests
