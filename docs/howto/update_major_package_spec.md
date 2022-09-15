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

### field ...: Additional property ... is not allowed

Validation of properties on fields definitions is stricter now. If you see any
error about properties not allowed in fields definitions, this probably means
one of these two things:

There is a typo, please check that this is the property you wanted to use.

The property is not supported, then you should probably remove it. If you think
that the property should be supported, please [open an issue](https://github.com/elastic/package-spec/issues/new?assignees=&labels=discuss&template=Change_Proposal.md&title=%5BChange+Proposal%5D+)

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

### field ...type: ...type must be one of the following:...

This happens when a field doesn't have one of the allowed types. In Package Spec
2.0.0 we are removing the `array` data type. The reason is that this cannot be
mapped to any data type in Elasticsearch, and an invalid configuration can lead
to an invalid mapping.

Any Elasticsearch field can contain any number of values of the same type, and
this is what is expected in most of the cases when using the `array` type.

If you are using something like the following definition:
```
- name: ciphersuites
  type: array
```

You should be using something like this:
```
- name: ciphersuites
  type: keyword
```

If you were using `object_type` to define the type of the elements in the array,
like this:
```
- name: ciphersuites
  type: array
  object_type: keyword
```

You can instead use the object type directly as type:
```
- name: ciphersuites
  type: keyword
```

### Invalid field ilm_policy

Package Spec 2.0.0 will include additional validations on the `ilm_policy` field of
the data streams manifest. These validations aim to ensure that the ILM policy
is defined in the data stream, so the package is self-contained.

If you find errors related to this field, it can be caused by a typo, or because
the policy is missing. The error message will describe the expected values.

To solve this error, check that the value of `ilm_policy` follows the pattern:
```
{data stream type}-{package name}.{data stream name}-{ilm file name without extension}
```

And an ILM policy definition exists in:
```
./data_streams/{data stream name}/elasticsearch/ilm/{ilm file name without extension}.json
```

It could be the case that you are trying to reference to an ILM policy defined
in some other place out of the package. This is not supported. If you have a use
case for this, please [open an issue](https://github.com/elastic/package-spec/issues/new?assignees=&labels=discuss&template=Change_Proposal.md&title=%5BChange+Proposal%5D+)
for discussion.
