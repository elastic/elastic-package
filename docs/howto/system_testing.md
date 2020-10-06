# HOWTO: Writing system tests for a package

## Introduction
Elastic Packages are comprised of data streams. A system test exercises the end-to-end flow of data for a package's datastream — from ingesting it from the package's integration service all the way to indexing it into an Elasticsearch data stream.

## Process

Conceptually, a system test will perform the following steps:
1. Deploy the Elastic Stack, including a 1-node Elasticsearch cluster, a Kibana instance, and an instance of Elastic Agent.
1. Enroll the Elastic Agent with Fleet (running in the Kibana instance).
1. Depending on the Elastic Package whose data stream is being tested, deploy an instance of the package's integration service.
1. Create a test policy that configures a single data stream for a single package.
1. Assign the test policy to the enrolled Agent.
1. Wait a reasonable amount of time for the Agent to collect data from the integration service and index it into the correct Elasticsearch data stream.
1. Delete test artifacts and tear down deployed resources.

## Limitations

At the moment system tests have limitations. The salient ones are:
* They can only test package's whose integration services can be deployed via a Docker Compose file. Eventually they will be able to test package's that can be deployed via other means, e.g. a Terraform configuration.
* They can only check for the _existence_ of data in the correct Elasticsearch data stream. Eventually they will be able to test the shape and contents of the indexed data as well.

## Defining a system test

## Running a system test



