# HOWTO: Deprecate a package or individual feature

## Introduction

You can mark a package or an individual feature as `deprecated`. Deprecation means that the package or feature is retired and will only be maintained for a defined period. packages maintained by Elastic the maintenance period will follow EOL policies that can be found in [https://www.elastic.co/support/eol](https://www.elastic.co/support/eol).

Individual feature deprecation is supported for: inputs, policy templates, variables, and data streams.

Deprecation markers are available since spec version `3.6.0`.

## Lifecycle of package or feature deprecation

1. A package or feature is marked as deprecated.  
   You set the `deprecated.since` field to the version when deprecation takes effect and the `deprecated.description` field with a message regarding the deprecation. Both are required. Optionally, you can add information about a new package or feature that replaces the current one using `deprecated.replaced_by`. Available fields for replacing a package or individual feature are: `package`, `input`, `policy_template`, `variable` and `data_stream`.

   Some examples of deprecated field:

   ```yaml
   # deprecated package
    deprecated:
     since: "2.4.0"
     description: This package is deprecated and will reach End-of-Life after the maintenance period.
     replaced_by:
      package: new_integration
   ```

   ```yaml
   # deprecated policy_template
    deprecated:
     since: "2.4.0"
     description: This policy_template is deprecated.
     replaced_by:
      policy_template: new_policy
   ```

   ```yaml
   # deprecated input
    deprecated:
     since: "2.4.0"
     description: This input is deprecated.
     replaced_by:
      input: new_input_name
   ```

2. It remains available in the registry for installation and use.

3. A maintenance period follows (usually one year), during which the package or feature stays available and can receive critical fixes. Newer versions of the package can be released, which should update the deprecated information.

4. After this period, the package or feature reaches End-of-Life (EOL) and may be removed from the registry if the authors choose to do so.

## Deprecate a package

1. Bump the package version and set the deprecation in the package manifest (`manifest.yml`). Add a `deprecated` block with `since` set to the current package version:

   ```yaml
   format_version: "3.5.0"
   name: my_integration
   title: My Integration
   version: "2.4.0"
   # ... other manifest fields ...

   deprecated:
     since: "2.4.0"
     description: This package is deprecated and will reach End-of-Life after the maintenance period.
   ```

2. Document the deprecation in the changelog (`changelog.yml`). Add an entry for this version and use the `deprecation` type so that Fleet UI can show deprecation warnings:

   ```yaml
   # newer versions go on top
   - version: "2.4.0"
     changes:
       - description: This package is deprecated and will reach End-of-Life after the maintenance period.
         type: deprecation
         link: https://github.com/elastic/integrations/issues/1234
   ```

3. Publish the new version to the package registry. From this version onward, the package is officially retired but remains installable; Kibana will display warnings during the maintenance period.

## Deprecate an individual feature

1. Add deprecation to the feature in the appropriate manifest. For a data stream, set the `deprecated` block in that data stream’s `manifest.yml`. The exact schema is defined in the Package Spec; typically it includes `since` with the version when the feature was deprecated.

   Example of a data stream manifest (`data_stream/<stream_name>/manifest.yml`):

   ```yaml
   title: "Legacy logs"
   type: logs
   # ... other data stream fields ...

   deprecated:
     since: "2.4.0"
   ```

2. Record the change in the changelog (`changelog.yml`) with type `deprecation`:

   ```yaml
   - version: "2.4.0"
     changes:
       - description: The "legacy_logs" data stream is deprecated and will be removed in a future version.
         type: deprecation
         link: https://github.com/elastic/integrations/issues/1234
   ```

3. Publish the new package version. The deprecated feature stays available for the maintenance period; Kibana and Fleet can show warnings based on the registry and changelog.

## Updates to a deprecated package

Modifications to a deprecated package are allowed as long as the deprecated status is kept. You can release security updates or other patches: bump the version (for example from 2.4.0 to 2.4.1), keep the `deprecated` block in the manifest, and update the deprecation description if needed. The package remains deprecated; only the version and the message change.

The package registry always exposes the latest deprecation information for a package. If a package was deprecated in 2.4.0 and you later release a patched version 2.4.1 that is also deprecated (with or without an updated description), clients that retrieve package information via the registry will see the deprecation details from 2.4.1 — the most recent version — not from 2.4.0.
