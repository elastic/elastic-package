#!/usr/bin/env groovy

@Library('apm@current') _

pipeline {
  agent { label 'ubuntu-20 && immutable' }
  environment {
    REPO = "elastic-package"
    REPO_BUILD_TAG = "${env.REPO}/${env.BUILD_TAG}"

    BASE_DIR="src/github.com/elastic/elastic-package"
    JOB_GIT_CREDENTIALS = "f6c7695a-671e-4f4f-a331-acdce44ff9ba"
    GITHUB_TOKEN_CREDENTIALS = "2a9602aa-ab9f-4e52-baf3-b71ca88469c7"
    PIPELINE_LOG_LEVEL='INFO'

    // Signing
    JOB_SIGNING_CREDENTIALS = 'sign-artifacts-with-gpg-job'
    INFRA_SIGNING_BUCKET_NAME = 'internal-ci-artifacts'
    INFRA_SIGNING_BUCKET_SIGNED_ARTIFACTS_SUBFOLDER = "${env.REPO_BUILD_TAG}/signed-artifacts"
    INFRA_SIGNING_BUCKET_ARTIFACTS_PATH = "gs://${env.INFRA_SIGNING_BUCKET_NAME}/${env.REPO_BUILD_TAG}"
    INFRA_SIGNING_BUCKET_SIGNED_ARTIFACTS_PATH = "gs://${env.INFRA_SIGNING_BUCKET_NAME}/${env.INFRA_SIGNING_BUCKET_SIGNED_ARTIFACTS_SUBFOLDER}"

    // Publishing
    INTERNAL_CI_JOB_GCS_CREDENTIALS = 'internal-ci-gcs-plugin'
    PACKAGE_STORAGE_UPLOADER_CREDENTIALS = 'upload-package-to-package-storage'
    PACKAGE_STORAGE_UPLOADER_GCP_SERVICE_ACCOUNT = 'secret/gce/elastic-bekitzur/service-account/package-storage-uploader'
    PACKAGE_STORAGE_INTERNAL_BUCKET_QUEUE_PUBLISHING_PATH = "gs://elastic-bekitzur-package-storage-internal/queue-publishing/${env.REPO_BUILD_TAG}"
  }
  options {
    timeout(time: 1, unit: 'HOURS')
    buildDiscarder(logRotator(numToKeepStr: '20', artifactNumToKeepStr: '20', daysToKeepStr: '30'))
    timestamps()
    ansiColor('xterm')
    disableResume()
    durabilityHint('PERFORMANCE_OPTIMIZED')
    rateLimitBuilds(throttle: [count: 60, durationName: 'hour', userBoost: true])
    quietPeriod(10)
  }
  triggers {
    issueCommentTrigger("${obltGitHubComments()}")
  }
  stages {
    stage('Checkout') {
      steps {
        pipelineManager([ cancelPreviousRunningBuilds: [ when: 'PR' ] ])
        deleteDir()
        gitCheckout(basedir: "${BASE_DIR}")
        stash(allowEmpty: true, name: 'source', useDefaultExcludes: false)
      }
    }
    stage('Build package') {
      steps {
        cleanup()
        withGoEnv() {
          dir("${BASE_DIR}") {
            sh(label: 'Install elastic-package',script: "make install")
            // sh(label: 'Install elastic-package', script: 'go build github.com/elastic/elastic-package')
            dir("test/packages/package-storage/package_storage_candidate") {
              sh(label: 'Build package', script: "elastic-package build -v --zip")
            }
          }
        }
        stash(allowEmpty: true, name: 'build-package', includes: "${BASE_DIR}/build/packages/*.zip", useDefaultExcludes: false)
      }
    }
    stage('Sign and publish package') {
      steps {
        cleanup(source: 'build-package')
        dir("${BASE_DIR}") {
          packageStoragePublish('build/packages')
        }
      }
    }
  }
  post {
    cleanup {
      notifyBuildResult(prComment: false)
    }
  }
}

def packageStoragePublish(builtPackagesPath) {
  signUnpublishedArtifactsWithElastic(builtPackagesPath)
  uploadUnpublishedToPackageStorage(builtPackagesPath)
}

def signUnpublishedArtifactsWithElastic(builtPackagesPath) {
  def unpublished = false
  dir(builtPackagesPath) {
    findFiles()?.findAll{ it.name.endsWith('.zip') }?.collect{ it.name }?.sort()?.each {
      def packageZip = it
      if (isAlreadyPublished(packageZip)) {
        return
      }

      unpublished = true
      googleStorageUpload(bucket: env.INFRA_SIGNING_BUCKET_ARTIFACTS_PATH,
        credentialsId: env.INTERNAL_CI_JOB_GCS_CREDENTIALS,
        pattern: '*.zip',
        sharedPublicly: false,
        showInline: true)
    }
  }

  if (!unpublished) {
    return
  }

  withCredentials([string(credentialsId: env.JOB_SIGNING_CREDENTIALS, variable: 'TOKEN')]) {
    triggerRemoteJob(auth: CredentialsAuth(credentials: 'local-readonly-api-token'),
      job: 'https://internal-ci.elastic.co/job/elastic+unified-release+master+sign-artifacts-with-gpg',
      token: TOKEN,
      parameters: [
        gcs_input_path: env.INFRA_SIGNING_BUCKET_ARTIFACTS_PATH,
      ],
      useCrumbCache: false,
      useJobInfoCache: false)
  }
  googleStorageDownload(bucketUri: "${env.INFRA_SIGNING_BUCKET_SIGNED_ARTIFACTS_PATH}/*",
    credentialsId: env.INTERNAL_CI_JOB_GCS_CREDENTIALS,
    localDirectory: builtPackagesPath + '/',
    pathPrefix: "${env.INFRA_SIGNING_BUCKET_SIGNED_ARTIFACTS_SUBFOLDER}")
    sh(label: 'Rename .asc to .sig', script: 'for f in ' + builtPackagesPath + '/*.asc; do mv "$f" "${f%.asc}.sig"; done')
}

def uploadUnpublishedToPackageStorage(builtPackagesPath) {
  dir(builtPackagesPath) {
    withGCPEnv(secret: env.PACKAGE_STORAGE_UPLOADER_GCP_SERVICE_ACCOUNT) {
      withCredentials([string(credentialsId: env.PACKAGE_STORAGE_UPLOADER_CREDENTIALS, variable: 'TOKEN')]) {
        findFiles()?.findAll{ it.name.endsWith('.zip') }?.collect{ it.name }?.sort()?.each {
          def packageZip = it
          if (isAlreadyPublished(packageZip)) {
            return
          }

          sh(label: 'Upload package .zip file', script: "gsutil cp ${packageZip} ${env.PACKAGE_STORAGE_INTERNAL_BUCKET_QUEUE_PUBLISHING_PATH}/")
          sh(label: 'Upload package .sig file', script: "gsutil cp ${packageZip}.sig ${env.PACKAGE_STORAGE_INTERNAL_BUCKET_QUEUE_PUBLISHING_PATH}/")

          triggerRemoteJob(auth: CredentialsAuth(credentials: 'local-readonly-api-token'),
            job: 'https://internal-ci.elastic.co/job/package_storage/job/publishing-job-remote',
            token: TOKEN,
            parameters: [
              dry_run: true,
              gs_package_build_zip_path: "${env.PACKAGE_STORAGE_INTERNAL_BUCKET_QUEUE_PUBLISHING_PATH}/${packageZip}",
              gs_package_signature_path: "${env.PACKAGE_STORAGE_INTERNAL_BUCKET_QUEUE_PUBLISHING_PATH}/${packageZip}.sig",
            ],
            useCrumbCache: true,
            useJobInfoCache: true)
        }
      }
    }
  }
}

def isAlreadyPublished(packageZip) {
  def responseCode = httpRequest(method: "HEAD",
    url: "https://package-storage.elastic.co/artifacts/packages/${packageZip}",
    response_code_only: true)
  return responseCode == 200
}

def cleanup(Map args = [:]) {
  def source = args.containsKey('source') ? args.source : 'source'
  dir("${BASE_DIR}"){
    deleteDir()
  }
  unstash source
}
