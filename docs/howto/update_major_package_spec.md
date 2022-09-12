# Update between Package Spec major versions

Each Package Spec major version may bring breaking changes or additional
validations that may make tests to fail for some packages. Find here guidelines
about how to fix these issues.

Version of the Package Spec used by a package is defined by the `format_version`
setting in the `manifest.yml` file.

## Troubleshooting upgrades from Package Spec v1 to v2

### field (root): Additional property license is not allowed

The `license` field was deprecated in Package Spec 1.13.0 and removed in 2.0.0.
This field was used to indicate the required subscription. The `elastic.subscription`
condition should be used instead.

So, for example, for a package with `license: basic`, you must remove this line
and add the following condition:
```
conditions:
  elastic.subscription: basic
```

### field ... is not normalized as expected: expected array, found ...

ECS fields can indicate normalization rules. `elastic-package` checks that they
are followed in test documents.

To solve this, modify the ingest pipeline of the package to produce an array
instead of single values. This is needed even when the field is only going to
store a single value.

This only affects how the data is represented in the source documents, it
doesn't affect how the data can be queried.

For example the following processor:
```
- set:
    field: event.category
    value: "web"
```

Should be replaced with:
```
- set:
    field: event.category
    value: ["web"]
```

### field "event.type" value ... is not one of the expected values (...) for ...

Some ECS fields can add restrictions on the values that `event.type` can have.
So when using any of these fields, the values of `event.type` must be aligned
with the expected values.

To solve this, ensure that the generated documents have the `event.type` set to
some of the expected values.

For example if a document contains `event.category: web`, the value of
`event.type` must be `access`, `error` or `info` according to ECS 8.4.

### field "event.dataset" should have value ..., it has ...

The fields `event.dataset` and `data_stream.dataset` should contain the name of
the package and the name of the data stream that generates it, separated by a
dot. For example for documents of the "access" data stream of the Apache module,
it should be `apache.access`.

If these fields are not being correctly populated, look for the source of the
value.

If it is a constant keyword, review the configured value.
```
- name: event.dataset
  type: constant_keyword
  external: ecs
  value: "apache.access"
```

If the value comes with an unexpected value from the collector, you can override
it in the pipeline:
```
- set:
    field: event.dataset
    value: "apache.access"
```

Changing the value of `event.dataset` can be considered a breaking change, take
this into account in your package when adding the changelog entry.

### field "..." is undefined

Elastic Package looks for fields definitions for all the fields included in
documents generated during tests. It skips some fields that are considered to
depend more on the version of the Elastic Agent than on the integration itself.

The list of fields allowed to don't have a definition has been reduced for
packages following the Package Spec v2.

The fields that must be defined now are the ones starting belonging to the
following groups:
* `cloud`
* `event`
* `host`

When these fields appear without using custom processing, it is very possible
that they are relevant to the integration, and they should be explicitly
defined.

To solve this issue, define the missing definitions. Usually they are ECS
fields, so they can be imported as:
```
- name: cloud.region
  external: ecs
```
