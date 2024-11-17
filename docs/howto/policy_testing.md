# HOWTO: Writing policy tests for a package

## Introduction
Elastic Packages support a variety of configuration variables as defined in
their manifest files. Policy tests allow to check that specific sets of
variables are accepted by Fleet, and they produce an expected agent policy.

## Defining policy tests

Policy tests can be defined in data streams in integration packages, or at the
package level in input packages.

Each test is composed of two files, one for the configuration of the policy, and
another one for the expected result.

When defining the tests at the data stream level, they must be defined following
this structure.
```
<package root>/
  data_stream/
    <data stream>/
      _dev/
        test/
          policy/
            test-<test name>.yml
            test-<test name>.expected
```

When defining the tests at the package level, in input packages, they must be
defined following this structure:
```
<package root>/
  _dev/
    test/
      policy/
        test-<test name>.yml
        test-<test name>.expected
```

It is possible, and encouraged, to define multiple policy tests for each package
or data stream.


## Global test configuration

Each package could define a configuration file in `_dev/test/config.yml` to skip all the policy tests.

```yaml
policy:
  skip:
    reason: <reason>
    link: <link_to_issue>
```

### Defining the configuration of the policy

Test configuration for the policy is defined in a YAML file prefixed with
`test-`.

In these configuration files it is possible to define:
- Values for the variables of the package manifest.
- Values for the variables of the data stream manifests (only used for
  integration packages).
- Input to use, for packages supporting multiple input types.

For example, the following configuration tells Fleet to create a policy using an
specific input, and some variables at the package level:
```
input: httpjson
vars:
  url: http://localhost:1234/api/v1/logs
  username: test
  password: test
```

The following configuration would set a value for a variable defined at the data
stream level:
```
data_stream:
  vars:
    paths:
      - "/var/logs/apache/access.log*"
```

This configuration may look familiar to you if you are used to define system
tests. The main difference is that policy tests are not going to be executed, so
anything can be configured there, without expectations on having running
services or reachable services. Also, no placeholders are expected to be found
in policy tests.


### Defining the expected policy

Once you have decided the policy settings you would like to test, you should
define the expected resulting policy. In principle it is possible to define it
manually given that most of the information is included in the package, but it
can be quite cumbersome. `elastic-package` is able to generate this file for
you, using the `--generate` flag.

If you run the policy tests with the `--generate` flag, `elastic-package` will
write the found policy in the expected place.
```
$ elastic-package test policy --generate
```

Then check that the generated content is what you would expect to have.


## Running policy tests

You can run policy tests with the `elastic-package test` command. If a package
includes policy tests, they will be executed if no test type is specified. You
can also run the policy tests only indicating its type:
```
$ elastic-package test policy
```

With integration packages, you can run the policy tests for a single data
stream, for example:
```
$ elastic-package test policy --data-streams access
```

Results are displayed using the usual format options. When the test fail,
`elastic-package` shows the differences between the expected and found policy.
