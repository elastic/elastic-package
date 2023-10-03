# Update between Package Spec major versions

Each Package Spec major version may bring breaking changes or additional
validations that may make tests to fail for some packages. Find here guidelines
about how to fix these issues.

Version of the Package Spec used by a package is defined by the `format_version`
setting in the `manifest.yml` file.

## Troubleshooting upgrades to Package Spec v3

### Error: building package failed: resolving external fields failed: can't resolve fields: field ... cannot be reused at top level

Some ECS fields include a reusability condition, so they can be reused on other
objects also defined in ECS. Some of these fields explicitly allow to reuse the
fields in the top level, while others explicitly disallow it.

Previous versions of elastic-package allowed to import reusable fields from ECS
at the top level in all cases, what shouldn't be always allowed.

If after updating to Package Spec v3 you find this error, please do the
following:

1. Check if your package is actually using this field.
2. If it is not using it, please remove it, and you are done.
3. If it is using it, copy the definition from ECS, or from the zip of a
   previous build of the package.
4. Consider moving the field to an ECS field if there is a suitable one. You
   may need to duplicate the field for some time to avoid breaking changes.

For example, `geo` fields are not expected to be used in the top level. If any
of your integrations is doing it, please check to what location this field
refers to. If it refers to the location of the host running elastic-agent, these
fields could be under `host.geo`. If they refer to the client side of a
connection, you could use `client.geo`.

### field ...: Additional property ... is not allowed

Package Spec 3.0.0 doesn't allow to use dotted notation in yaml configuration in
manifests. Previous implementation could lead to ambiguous results if the same
setting was included with and withoud dotted notation.

This is commonly found in `conditions` or in `elasticsearch` settings.

To solve this, please use nested dotations. So if for example your package has
something like the following:
```
conditions:
  elastic.subscription: basic
```
Transform it to the following:
```
conditions:
  elastic:
    subscription: basic
```

### file "..." is invalid: dangling reference found: ...

Package Spec 3.0.0 has stricter validation for some Kibana objects now. It
checks if all references included are defined in the own package.

Please remove or fix any reference to missing objects.

### field processors...: Additional property ... is not allowed

Some ingest pipeline processors are not well supported in the current Package
Spec, or there are alternative preferred ways to define them. These processors
won't be allowed to prevent other issues and have more consistent user
experience for some features.

If you find this error while trying to use the `reroute` processor, please use
instead the `routing_rules.yml` file, so users can more easily customize the
routing rules.

If you find this error while trying to use a new processor, please open an issue
in the Package Spec repository so we can add support for it.

### field owner: type is required

Package Spec 3.0.0 now requires the owner type field to be set. This field
describes who owns the package and the level of support that is provided.
The 'elastic' value indicates that the package is built and maintained by
Elastic. The 'partner' value indicates that the package is built and
maintained by a partner vendor and may include involvement from Elastic.
The 'community' value indicates the package is built and maintained by
non-Elastic community members.

The field was initially introduced in Package Spec `2.11.0` and prior to this
version assumed an implicit default of `elastic`. In `2.11.0`, the implicit
default changed to `community`. To avoid accidentally tagging an integration
with the wrong owner type, the field is now required.

The value must be one of the following:

- `elastic`
- `partner`
- `community`

```
owner:
  type: elastic
```

### expected filter in dashboard: ...

It is required to include filters in dashboards, to limit the size of the data
handled to the one included in related datasets.

To fix this issue, please include a filter in the dashboard. It usually means to
add a filter based on `data_stream.dataset`. But it is open to the package
developer to provide any filter that fits the use case of the package.

There are two variants of this error:

- `no filter found`: that means that no kind of filter has been found in the
  dashboard, and one should be added.
- `saved query found, but no filter`: that means that a saved query has been
  found, but no filter. If that's the case, please migrate the saved query to a
  filter. We want to make this filtering only in filters, for consistency
  between different dashboards, and to allow users to quickly filter using the
  query bar without affecting the provided filters.

### "My Dashboard" contains legacy visualization: "My Visualization" (metric, TSVB)

All visualizations must be created using [Lens](https://www.elastic.co/kibana/kibana-lens) or [Vega](https://www.elastic.co/guide/en/kibana/current/vega.html).

The only exceptions are
- Markdown panels created from the dashboard application. There are no plans to deprecate these.
- TSVB markdown. Support will eventually be removed, but this is
  currently allowed because we do not yet offer an alternative for
  injecting analytics into markdown. Prefer the dashboard markdown
  panels when possible.
- The legacy dashboard controls ("input-control-vis"). These should be replaced
  with the [new dashboard controls](https://www.elastic.co/guide/en/kibana/current/add-controls.html) but we are not currently
  enforcing this with tooling.

**Note:** most legacy visualizations can be converted by selecting "Convert to Lens"
from the dashboard panel context menu or by clicking "Edit visualization in Lens"
after opening the visualization in the editor.

## Troubleshooting upgrades to Package Spec v2

### field (root): Additional property license is not allowed

The `license` field was deprecated in Package Spec 1.13.0 and removed in 2.0.0.
This field was used to indicate the required subscription. The `elastic.subscription`
condition should be used instead.

So, for example, for a package with `license: basic`, you must remove this line
and add the following condition:
```
conditions:
  elastic:
    subscription: basic
```

### field ...: Additional property ... is not allowed

Validation of properties on fields definitions is stricter now. If you see any
error about properties not allowed in fields definitions, this probably means
one of these two things:

- There is a typo, please check that this is the property you wanted to use.
- The property is not supported, then you should probably remove it. If you think
that the property should be supported, please [open an issue](https://github.com/elastic/package-spec/issues/new?assignees=&labels=discuss&template=Change_Proposal.md&title=%5BChange+Proposal%5D+)

Some of the non-supported commonly used properties are `required`, `release` or
`overwrite`.

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
