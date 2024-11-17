# HOWTO: Make dashboards editable in Kibana

## Introduction

As of 8.11, managed assets, including dashboards, are read-only in Kibana. This change was introduced to prevent users from losing changes on package upgrades. Integrations authors, however, need the ability to edit assets in order to adopt new features.

## Making a dashboard editable

Dashboards can be made editable in Kibana by using the `elastic-package edit dashboards` command. This command can either be run interactively, allowing manual selection of dashboards, or be passed a comma-separated list of dashboard ids.

NB: after modifying dashboards, these need to be exported using `elastic-package export dashboards`.

### Using the interactive dashboard selection prompt

Run the following command:
```
elastic-package edit dashboards
```

Use the interactive dashboard selection prompt to select the dashboard(s) that should be made editable.

### Using a comma-separated list of dashboard ids

Pass the list with the `-d` flag:
```
elastic-package edit dashboards -d 123,456,789
```

Each dashboard id will be processed and the outcome of the updates will be listed in the command's final output.

### Command output

The final output will provide the outcome (success or failure) of the update for each dashboard.

For example, assuming the following command:
```
elastic-package edit dashboards -d 123,456,789
```

#### Success

Assuming '123', '456' and '789' are valid dashboard ids and all three updates succeed, the output will be successful and report the URLs of the updated dashboards:
```
Make Kibana dashboards editable

The following dashboards are now editable in Kibana:
https://<kibanaURL>/app/dashboards#/view/123
https://<kibanaURL>/app/dashboards#/view/456
https://<kibanaURL>/app/dashboards#/view/789

Done
```

#### Partial failure

Assuming that `456` is an invalid dashboard id and that the update is successful for ids `123` and `789`, the output will report the URLs of the updated dashboards as well as an error listing the failures:
```
Make Kibana dashboards editable

The following dashboards are now editable in Kibana:
https://<kibanaURL>/app/dashboards#/view/123
https://<kibanaURL>/app/dashboards#/view/789

Error: failed to make one or more dashboards editable: failed to export dashboard 456: could not export saved objects; API status code = 400; response body = {"statusCode":400,"error":"Bad Request","message":"Error fetching objects to export","attributes":{"objects":[{"id":"456","type":"dashboard","error":{"statusCode":404,"error":"Not Found","message":"Saved object [dashboard/456] not found"}}]}}
```

### Optional flags

* `allow-snapshot`: to allow exporting dashboards from a Elastic stack SNAPSHOT version
* `tls-skip-verify`: to skip TLS verify
