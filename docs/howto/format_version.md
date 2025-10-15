# Guidelines for the format version to use in a package

Packages need to indicate the version of the [Package Spec](https://github.com/elastic/package-spec) they are using.
This is done by setting the `format_version` field in the main manifest.

The version of the spec used influences the features available for the package,
the validations that are performed and some formatting rules. In general newer
versions include more validations intended to improve the reliability of
packages. They also include definitions that enable the use of some features.

Some stack versions restrict the discovery and installation of packages of
specific spec versions. This helps to avoid using packages that require features
that are not available in older versions of the stack.

## What format versions are supported by each version of the stack?

This is controlled in two places now:
- In the [Kibana default configuration](https://github.com/elastic/kibana/blob/84fcda021be1d71018fa77005837da7e932c6d7f/x-pack/plugins/fleet/server/config.ts#L224).
- In the user-provided configuration of Kibana, by setting
  `xpack.fleet.internal.registry.spec.min` and/or
  `xpack.fleet.internal.registry.spec.max`.

The following table shows what versions of the stack support each format
version.

| Format Version | Versions of the stack that support it |
| -- | -- |
| <2.3 | <9.0 |
| 2.3 - 2.11 | All versions, except serverless. |
| 3.0 | All versions. |
| 3.1 - 3.3 | All versions from 8.16. |
| 3.4 | 8.19 and all versions from 9.1. |
| 3.5 | All versions from 9.2. |

Versions older than 8.11 didn't have restrictions on the supported spec versions,
so they might have limited support for some features.

Serverless supports from 3.0 to 3.5.

In case of doubt, you can check the Fleet default configuration, and the
configuration overrides in the Kibana repository.

## What format version to choose for a package?

The general rule of thumb is to use the earliest version after 3.0 that enables
everything that is required for a package.

The safest option, to support a broader range of stack versions, and a greater
number of features, would be to use 3.0.4. If you need some newer feature, you
would need to increase this version, but this can limit the availability of the
package in older versions of the stack.

Regarding compatibility with versions of the stack, check also the [guidelines
for stack version constraints](./stack_version_support.md).
