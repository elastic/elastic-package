variable "TEST_RUN_ID" {
  default = "detached"
}

variable "REPO_NAME" {
  default = "unknown-repo"
}

variable "PULL_REQUEST" {
  default = "unknown-pr"
}

variable "CI_BUILD_NUMBER" {
  default = "unknown-build"
}

variable "BRANCH_NAME" {
  description = "Branch name for tagging purposes"
  default = "unknown-branch"
}

variable "BUILD_ID" {
  description = "Build ID in the CI for tagging purposes"
  default = "unknown-build"
}

variable "CREATED_DATE" {
  description = "Creation date for tagging purposes"
  default = "unknown-date"
}
variable "gcp_project_id" {
  type = string
}

variable "zone" {
  type = string
  // NOTE: if you change this value you **must** change it also for test
  // configuration, otherwise the tests will not be able to find metrics in
  // the specified region
  default = "us-central1-a"
  # https://cloud.google.com/compute/docs/regions-zones#available
}
