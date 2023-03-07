// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/elastic/trigger-jenkins-buildkite-plugin/jenkins"
)

const (
	publishingRemoteJob = "package_storage/job/publishing-job-remote"
	signingJob          = "elastic+unified-release+master+sign-artifacts-wigh-gpg"

	publishJobKey = "publish"
	signJobKey    = "sign"
)

var allowedJenkinsJobs = map[string]string{
	publishJobKey: publishingRemoteJob,
	signJobKey:    signingJob,
}

var (
	jenkinsHost  = os.Getenv("JENKINS_HOST_SECRET")
	jenkinsUser  = os.Getenv("JENKINS_USERNAME_SECRET")
	jenkinsToken = os.Getenv("JENKINS_TOKEN")
)

func jenkinsJobOptions() []string {
	keys := make([]string, 0, len(allowedJenkinsJobs))
	for k := range allowedJenkinsJobs {
		keys = append(keys, k)
	}
	return keys
}

func main() {
	jenkinsJob := flag.String("jenkins-job", "", fmt.Sprintf("Jenkins job to trigger. Allowed values: %s", strings.Join(jenkinsJobOptions(), " ,")))
	zipPackagePath := flag.String("package", "", "Path to zip package file (*.zip) ")
	sigPackagePath := flag.String("signature", "", "Path to the signature file of the package file (*.zip.sig)")
	async := flag.Bool("async", false, "Run async the Jenkins job")
	flag.Parse()

	if _, ok := allowedJenkinsJobs[*jenkinsJob]; !ok {
		log.Fatal("Invalid jenkins job")
	}

	log.Printf("Triggering job: %s", allowedJenkinsJobs[*jenkinsJob])

	ctx := context.Background()
	client, err := jenkins.NewJenkinsClient(ctx, jenkinsHost, jenkinsUser, jenkinsToken)
	if err != nil {
		log.Fatalf("error creating jenkins client")
	}

	switch *jenkinsJob {
	case publishJobKey:
		err = runPublishingRemoteJob(ctx, client, *async, allowedJenkinsJobs[*jenkinsJob], *zipPackagePath, *sigPackagePath)
	case signJobKey:
		err = runSignPackageJob(ctx, client, *async, allowedJenkinsJobs[*jenkinsJob], *zipPackagePath)
	default:
		log.Fatal("unsupported jenkins job")
	}

	if err != nil {
		log.Fatal("Error: %s", err)
	}
}

func runSignPackageJob(ctx context.Context, client *jenkins.JenkinsClient, async bool, jobName, packagePath string) error {
	params := map[string]string{}
	// TODO set parameters for sign job

	return client.RunJob(ctx, jobName, async, params)
}

func runPublishingRemoteJob(ctx context.Context, client *jenkins.JenkinsClient, async bool, jobName, packagePath, signaturePath string) error {

	// Run the job with some parameters
	params := map[string]string{
		"dry_run":                   "true",
		"gs_package_build_zip_path": packagePath,
		"gs_package_signature_path": signaturePath,
	}

	return client.RunJob(ctx, jobName, async, params)
}
