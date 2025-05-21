# HOWTO: Use `elastic-package` to test packages using a Serverless project

## Introduction

`elastic-package` supports creating a new Serverless project to assist developers in testing their own packages in these offerings.

**Please note:** Testing packages using this method may result in additional charges.


### The `serverless` stack provider

The `serverless` provider facilitates the creation and management of a Serverless project, running the Elastic Agent and other services as local containers using Docker compose.

To use the `serverless` provider, follow these steps:
1. Obtain an API key from [Elastic Cloud API keys](https://cloud.elastic.co/account/keys).
2. Set the environment variable `EC_API_KEY` with the obtained API key:
   ```shell
   export EC_API_KEY=<api_key>
   ```
3. (Recommended) Create a new elastic-package profile for serverless:
   ```shell
   elastic-package profiles create serverless
   elastic-package profiles use serverless
   ```
4. Update the profile configuration with the following keys (e.g. `~/.elastic-package/profiles/serverless/config.yml`):
   ```yaml
   stack.elastic_cloud.host: https://cloud.elastic.co
   # supported observability and security types
   stack.serverless.type: observability
   stack.serverless.region: aws-us-east-1
   ```

After completing these steps, create a new Elastic Serverless project by running:
```sh
elastic-package stack up -v -d --provider serverless
```

Once this command finishes successfully, your environment will be ready to use
any `elastic-package` subcommand (`install`, `dump`, `test`, ...) targeting the Serverless project created with the above command.

To clean up everything, run:
```sh
elastic-package stack down -v
```

**Recommendation:** Ensure that Serverless projects are properly deleted upon completion of testing to avoid any unexpected charges.

Although Serverless projects are accessible through the management UI at [Elastic Cloud Projects](https://cloud.elastic.co/projects),
those projects created by `elastic-package` must be deleted using the `stack down` command as previously mentioned.
This procedure ensures that all resources and local configurations are cleaned up correctly, thereby allowing `elastic-package` to function
correctly in subsequent commands.

If you need to switch back to a local stack, remember to change to the previous profile. If the previous profile was `default`, run:
```shell
elastic-package profiles use default
```

### Considerations about testing with the `serverless` provider

There are some known issues when running pipeline tests with the `serverless` provider.

In Serverless projects, the GeoIP database cannot be modified as it is done with local stack.
To avoid errors related to those GeoIP fields in these tests, the results from
pipeline tests should not be compared. This can be achieved by setting the following environment variable:
```shell
export ELASTIC_PACKAGE_SERVERLESS_PIPELINE_TEST_DISABLE_COMPARE_RESULTS=true
```

### How to use an existing `Serverless` project

In case you want to test your packages using an already existing Serverless project, it could be used
the `environment` provider instead.
You can learn more about this in [this document](https://github.com/elastic/elastic-package/blob/main/docs/howto/use_existing_stack.md).

**IMPORTANT**: This provider modifies the Fleet configuration of the target stack.
Avoid using it in environments that you use for other purposes, specially in production environments
