#!/usr/bin/env groovy

@Library('apm@current') _

pipeline {
  agent { label 'ubuntu-20 && immutable' }
  environment {
    REPO = "elastic-package"

    BASE_DIR="src/github.com/elastic/elastic-package"
    JOB_GIT_CREDENTIALS = "f6c7695a-671e-4f4f-a331-acdce44ff9ba"
    GITHUB_TOKEN_CREDENTIALS = "2a9602aa-ab9f-4e52-baf3-b71ca88469c7"
    PIPELINE_LOG_LEVEL='INFO'
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
        stash allowEmpty: true, name: 'source', useDefaultExcludes: false
      }
    }
  }
  post {
    cleanup {
      notifyBuildResult(prComment: true)
    }
  }
}

def cleanup(){
  dir("${BASE_DIR}"){
    deleteDir()
  }
  unstash 'source'
}