# HOWTO: Install package

## Introduction
A package can be installed using `elastic-package install` command. This command uses the Kibana API to install the package in Kibana.

## Kibana < 8.7.0
For versions of `Kibana<8.7.0`, the packages must be exposed via the Package Registry.

In case of development, this means that the package should be built previously and then the Elastic stack must be started. Or at least, package-registry service needs to be restarted in the Elastic stack.

```shell
elastic-package build -v
elastic-package stack up -v -d  # elastic-package stack up -v -d --services package-registry
elastic-package install -v
```

## Kibana >= 8.7.0
Starting with Kibana 8.7.0, there is a new API to install the packages directly without the need to be exposed in Package Registry. It can be use zip files built using `elastic-package build`command.

**NOTE**: These methods assume that the packages have been validated previously.
- Before building the package, the package can be validated by running `elastic-package check` command.

From Kibana 8.7.0 version, `elastic-package install` is able to install packages through the upload API.
- The `--zip` parameter can be used to install a specific zip file.
- `install` subcommand instead of relying on Package Registry, it does the following steps:
    - build the package
    - upload the zip file built to Kibana.

Example of using `--zip` parameter:
```shell
 $ elastic-package stack up -v -d
 $ elastic-package install --zip /home/user/Coding/work/integrations/build/packages/elastic_package_registry-0.0.6.zip -v
2023/02/23 18:44:59 DEBUG Enable verbose logging
2023/02/23 18:44:59 DEBUG Distribution built without a version tag, can't determine release chronology. Please consider using official releases at https://github.com/elastic/elastic-package/releases
Install zip package: /home/user/Coding/work/integrations/build/packages/elastic_package_registry-0.0.6.zip
2023/02/23 18:44:59 DEBUG GET https://127.0.0.1:5601/api/status
Install the package
2023/02/23 18:44:59 DEBUG POST https://127.0.0.1:5601/api/fleet/epm/packages
Installed assets:
- elastic_package_registry-313c2700-099b-11ed-91b6-3b1f9c2b2771 (type: dashboard)
- metrics-elastic_package_registry.metrics-0.0.6 (type: ingest_pipeline)
- metrics-elastic_package_registry.metrics (type: index_template)
- metrics-elastic_package_registry.metrics@package (type: component_template)
- metrics-elastic_package_registry.metrics@custom (type: component_template)
Done
```

Example of using `elastic-package install`
```shell
 $ elastic-package stack up -v -d
 $ elastic-package install -v
2023/02/28 12:34:44 DEBUG Enable verbose logging
2023/02/28 12:34:44 DEBUG Distribution built without a version tag, can't determine release chronology. Please consider using official releases at https://github.com/elastic/elastic-package/releases
2023/02/28 12:34:44 DEBUG Reading package manifest from /home/user/Coding/work/integrations/packages/elastic_package_registry
2023/02/28 12:34:44 DEBUG GET https://127.0.0.1:5601/api/status
2023/02/28 12:34:44 DEBUG Build directory: /home/user/Coding/work/integrations/build/packages/elastic_package_registry/0.0.6
2023/02/28 12:34:44 DEBUG Clear target directory (path: /home/user/Coding/work/integrations/build/packages/elastic_package_registry/0.0.6)
2023/02/28 12:34:44 DEBUG Copy package content (source: /home/user/Coding/work/integrations/packages/elastic_package_registry)
2023/02/28 12:34:44 DEBUG Copy license file if needed
2023/02/28 12:34:44  INFO License text found in "/home/user/Coding/work/integrations/LICENSE.txt" will be included in package
2023/02/28 12:34:44 DEBUG Encode dashboards
2023/02/28 12:34:44 DEBUG Resolve external fields
2023/02/28 12:34:44 DEBUG Package has external dependencies defined
2023/02/28 12:34:44 DEBUG data_stream/metrics/fields/base-fields.yml: source file hasn't been changed
2023/02/28 12:34:44 DEBUG data_stream/metrics/fields/ecs.yml: source file has been changed
2023/02/28 12:34:44 DEBUG data_stream/metrics/fields/fields.yml: source file hasn't been changed
2023/02/28 12:34:44 DEBUG Package doesn't have to import ECS mappings
2023/02/28 12:34:44 DEBUG Build zipped package
2023/02/28 12:34:44 DEBUG Compress using archives.Zip (destination: /home/user/Coding/work/integrations/build/packages/elastic_package_registry-0.0.6.zip)
2023/02/28 12:34:44 DEBUG Create work directory for archiving: /tmp/elastic-package-2222223038/elastic_package_registry-0.0.6
2023/02/28 12:34:44 DEBUG Validating built .zip package (path: /home/user/Coding/work/integrations/build/packages/elastic_package_registry-0.0.6.zip)
2023/02/28 12:34:44  INFO Built package path: /home/user/Coding/work/integrations/build/packages/elastic_package_registry-0.0.6.zip
Install zip package: /home/user/Coding/work/integrations/build/packages/elastic_package_registry-0.0.6.zip
2023/02/28 12:34:44 DEBUG POST https://127.0.0.1:5601/api/fleet/epm/packages
Installed assets:
- elastic_package_registry-313c2700-099b-11ed-91b6-3b1f9c2b2771 (type: dashboard)
- metrics-elastic_package_registry.metrics-0.0.6 (type: ingest_pipeline)
- metrics-elastic_package_registry.metrics (type: index_template)
- metrics-elastic_package_registry.metrics@package (type: component_template)
- metrics-elastic_package_registry.metrics@custom (type: component_template)
Done
```

### Customization

This package installation can be customized to be installed in other Kibana instances setting the needed variables:
- ELASTIC_PACKAGE_KIBANA_HOST
- ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME
- ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD
- ELASTIC_PACKAGE_CA_CERT

As an example:
```bash
export ELASTIC_PACKAGE_KIBANA_HOST="https://test-installation.kibana.test:9243"
export ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME="elastic"
export ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD="xxx"
# if it is a public instance, this variable should not be needed
export ELASTIC_PACKAGE_CA_CERT=""

elastic-package install --zip elastic_package_registry-0.0.6.zip -v
```
