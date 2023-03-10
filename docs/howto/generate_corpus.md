# HOWTO: Generate corpus for a package dataset

## Introduction

The `elastic-package` tool can be used to generate a rally corpus for a package dataset.
This feature is currently in beta and manual steps are required to create a valid rally track from the generated corpus.
Currently, only data for what we have related assets on https://github.com/elastic/elastic-integration-corpus-generator-tool are supported.

### Generate a corpus for a package dataset

#### Steps

1. Run the elastic-package command for generating the corpus of the package dataset:
   `elastic-package benchmark generate-corpus --dataset sqs --package aws --size 100M`
   1. replace the sample value for `--dataset` with the one of the dataset you want to generate a corpus for
   2. replace the sample value for `--package` with the one of the package you want to generate a corpus for
   3. replace the sample value for `--size` with the *approximate* size of the corpus you want to generate
2. Choose a file where to redirect the output of the command if you want to save it:
   `elastic-package benchmark generate-corpus --dataset sqs --package aws --size 100M > aws.sqs.100M.ndjson`
    1. replace the sample value of the redirect file with the one you've chosen

### Generate a rally track for a package dataset and run a rally benchmark

*BEWARE*: this is only supported for `metrics` type data streams.

#### Steps

1. Run the elastic-package command for generating the corpus of the package dataset:
   `elastic-package benchmark generate-corpus --dataset sqs --package aws --size 100M --rally-track-output-dir
   ./track-output-dir`
   1. replace the sample value for `--dataset` with the one of the dataset you want to generate a corpus for
   2. replace the sample value for `--package` with the one of the package you want to generate a corpus for
   3. replace the sample value for `--size` with the *approximate* size of the corpus you want to generate
   4. replace the sample value for `--rally-track-output-dir` with the path to the folder where you want to save the rally track and the generated corpus (the folder will be created if it does not exist already)
2. Go to the Kibana instance of the cluster you want to run the rally on and install the integration package that you have generated the rally track for. 
3. Run the rally race with the generated track:
   `esrally race --kill-running-processes --track-path=./track-output-dir --target-hosts=my-deployment.es.eastus2.azure.elastic-cloud.com:443 --pipeline=benchmark-only`
   1. replace the sample value for `--track-path` with the path to the folder provided as `--rally-track-output-dir` at step 1
   2. replace the sample value for `--target-hosts` with the host and port of the Elasticsearch instance(s) you want rally to connect to.
   3. You might need to add the "client-options" parameter to rally in order to authenticate and use SSL: `--client-options="use_ssl:true,verify_certs:true,basic_auth_user:'elastic',basic_auth_password:'changeme'"`
      1. replace the sample value for `basic_auth_user` and `basic_auth_password` in `--client-options` to the credentials of the user in the cluster you want rally to use.
