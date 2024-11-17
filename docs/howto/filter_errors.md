# HOWTO: Filter errors based on Validation Codes

## Introduction

Starting with package-spec 2.13.0, there is an option to filter some errors based on their validation codes.

These validation codes are defined in package-spec [here][package-spec-code-errors].

## How to filter specific errors

Since package-spec 2.13.0, it's allowed to define a new file in the root of the package `validation.yml`.
In this file, the errors to be filtered  will be configured following this syntax:

```yaml
errors:
  exclude_checks:
    - code1
    - code2
```

`exclude_checks` key accepts a list of codes so more than one kind of error can be filtered.


The code errors available to use in this configuration can be checked [here][package-spec-code-errors].
The codes can be also checked in the error message itself shown by elastic-package. If the error is allowed
to be filtered, it will have at the end of the message the code in parenthesis.

For instance, in this example the error code is `SVR00002`:

```
[2023-09-28T19:10:19.758Z] Error: checking package failed: linting package failed: found 1 validation error:
[2023-09-28T19:10:19.758Z]    1. file "/var/lib/jenkins/workspace/est-manager_integrations_PR-8012/src/github.com/elastic/integrations/packages/o365/kibana/dashboard/o365-712e2c00-685d-11ea-8d6a-292ef5d68366.json" is invalid: expected filter in dashboard: no filter found (SVR00002)
```


For that specific error, if it is needed to be filtered, then the configuration file `validation.yml` should be like:
```yaml
errors:
  exclude_checks:
    - SVR00002
```


[package-spec-code-errors]: https://github.com/elastic/package-spec/blob/49120aea627a1652823a7f344ba3d1c9b029fd5a/code/go/pkg/specerrors/constants.go

