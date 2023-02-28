// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bndr/gojenkins"
	"github.com/pkg/errors"
)

type jenkinsClient struct {
	client *gojenkins.Jenkins
}

func newJenkinsClient(ctx context.Context, host, user, token string) (*jenkinsClient, error) {
	jenkins, err := gojenkins.CreateJenkins(nil, host, user, token).Init(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "client coult not be created")
	}

	return &jenkinsClient{
		client: jenkins,
	}, nil
}

func (j *jenkinsClient) runJob(ctx context.Context, jobName string, async bool, params map[string]string) error {
	queueId, err := j.client.BuildJob(ctx, publishingRemoteJob, params)
	if err != nil {
		fmt.Printf("error running job %s : %s\n", publishingRemoteJob, err)
		return err
	}
	build, err := j.getBuildFromJobAndQueueID(ctx, jobName, queueId)
	if err != nil {
		return err
	}
	log.Printf("Job triggered %s/%d\n", jobName, build.GetBuildNumber())

	if async {
		return nil
	}

	log.Printf("Waiting to be finished %s\n", build.GetUrl())
	j.waitForBuildFinished(ctx, build)

	log.Printf("Build %s finished with result: %v\n", build.GetUrl(), build.GetBuildNumber(), build.GetResult())
	return nil
}

func (j *jenkinsClient) getBuildFromJobAndQueueID(ctx context.Context, jobName string, queueId int64) (*gojenkins.Build, error) {
	job, err := j.client.GetJob(ctx, jobName)
	if err != nil {
		return nil, errors.Wrapf(err, "not able to get job %s", jobName)
	}

	build, err := j.getBuildFromQueueID(ctx, job, queueId)
	if err != nil {
		return nil, errors.Wrapf(err, "not able to get build from %s", jobName)
	}
	return build, nil
}

// based on https://github.com/bndr/gojenkins/blob/master/jenkins.go#L282
func (j *jenkinsClient) getBuildFromQueueID(ctx context.Context, job *gojenkins.Job, queueid int64) (*gojenkins.Build, error) {
	task, err := j.client.GetQueueItem(ctx, queueid)
	if err != nil {
		return nil, err
	}
	// Jenkins queue API has about 4.7second quiet period
	waitingTime := 1000 * time.Millisecond
	for task.Raw.Executable.Number == 0 {
		select {
		case <-time.After(waitingTime):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		time.Sleep(waitingTime)
		_, err = task.Poll(ctx)
		if err != nil {
			return nil, err
		}
	}

	build, err := job.GetBuild(ctx, task.Raw.Executable.Number)
	if err != nil {
		return nil, errors.Wrapf(err, "not able to retrieve build %s", task.Raw.Executable.Number)
	}
	return build, nil
}

func (j *jenkinsClient) waitForBuildFinished(ctx context.Context, build *gojenkins.Build) {
	waitingTime := 5000 * time.Millisecond
	for build.IsRunning(ctx) {
		log.Printf("Build still running, waiting for 5 secs...")
		select {
		case <-time.After(waitingTime):
		case <-ctx.Done():
			return
		}
		time.Sleep(waitingTime)
		build.Poll(ctx)
	}
}
