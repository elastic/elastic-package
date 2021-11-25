# Guidelines for version constraints

Packages should include constraints for the supported Kibana versions. These
constraints are defined in the `manifest.yml` file. For example the following
condition specifies that the package can be used with any 7.x version, starting
on 7.9.0.
```
conditions:
  kibana.version: `^7.9.0`
```


## What condition to use?

TLDR; For new packages, use the current version of the Elastic Stack, and
support the current major, for example: `^7.15.0`. For existing packages, jump
to the next section.

Usually you should define the equivalent to a range of versions. You can read more
about the possible conditions in https://github.com/masterminds/semver#basic-comparisons

The lower bound should be a version of Kibana where the package works. In
principle it can be the version used to develop the package, but the package
developer may test the package with older versions, and if it works, use it to
support a broader set of Elastic Stack versions.

The upper bound may be more open, you can decide to support any future
version, any future minor version in the same major,
or a limited number of future versions, majors or minors. There may be
exceptions to these rules, for example you may decide to stop testing and
releasing new features for a previous major by moving from something like
`^7.16.0 || ^8.0.0` to `^8.0.0`, even if the package itself works.

Having a broader range of versions also helps with upgrades, a package that
works with a big range of minors will potentially cause less disruption to
users that a package that needs to be upgraded on every minor release.

Having a too broad range of versions may cause problems in the future, a package
released without an upper bound can continue to be available even in future
majors where it doesn't work anymore.

As a developer you also need to take into account that maintaining different
versions of the package for different versions of the Elastic Stack will require
of different code bases (or branches) and the neccesity of backport or
cherry-pick bugfixes between them.

For more recommendations, read the section below about recommendations on support for
multiple versions.


## When to update the condition?

In general, you must update the condition if:
* Compatibility with one or more of the oldest versions is broken.
* You want to extend support to more versions, by increasing the upper bound.

You may also consider to update the condition if:
* You want to offer a new feature only on certain versions of the Elastic Stack.
* You want to stop supporting old versions of the Elastic Stack.
* You want to prepare the package for a future known deprecation or breaking change, for example to change from `^8.0.0` to `^8.0.0, <8.5.0` it the change is going to happen in 8.5.0.
* You need to extend the upper bound to support new versions.

Updating the condition requires to release a new version of the package, this
can be done with or without additional changes.


## Is it ok to stop supporting older versions?

The decision is yours, package developer. But take into account that it is very
possible that there are still users of these versions, specially if they are
still supported. Releasing bugfixes or even new features for users of these
versions will provide more value to them for a longer time. There can be users
(or customers) that cannot upgrade their stacks, but they could still benefit of
your fixes if their versions are supported.


## Recomendations on support for multiple versions

### Support for multiple majors

Examples: `^7.14.0 || ^8.0.0`, `^7.14.0 || ^8.0.0 || ^9.0.0`

With this approach, the released package can be used in any tested major version
since the release of the package.

Pros:
* No need to keep multiple development branches, no need for backports, same
  package for 7.x and 8.x.
* Less disruptive for users.

Cons:
* Testing should be done with more versions.
* May require compatibility code.
* Deprecation path needed if package removed before next major.
* There are differences between major versions that have influence on test
  results (for example: different GeoIP databases).

Recommendation: Good for simple packages monitoring services that don't need
many special features, and have low coupling with specific versions of the
Elastic Stack.


### Support for current major

Examples: `^8.0.0`, `^8.2.0`

With this approach, the released package can be used only in the specified
major.

Pros:
* Testing need to be done with less versions.
* Changes don't need to take into account older versions.
* More open to use newer features of the Elastic Stack.

Cons:
* May require multiple development branches and backport to support multiple
  majors. This is specially a problem in repositories with multiple packages.
* Deprecation path needed if package removed before next major.
* Stack updates may be affected by updates of these packages, difficult to
  provide compatibility during upgrades, though frictions may be more acceptable
  in upgrades between majors.

Recommendation: Good option for packages that use features not available in older
majors, or that introduce breaking changes. Also recommended if the maintainer
decides to don't provide bugfixes for previous major.


### Support for last minors of previous major and first minors of the next

Examples: `^7.16.0 || ~8.0.0`, `>=7.16.0, <8.3.0`

With this approach, the released package can be used in some of the last minors
of previous majors, and some of the first minors of the next major.

Pros:
* Delays the need of keeping multiple development branches.
* Delays the decision of supporting the whole new major.
* Less disruptive for users.
* There are differences between major versions that have influence on test
  results (for example: different GeoIP databases).

Cons:
* Testing should be done with multiple majors.
* May require compatibility code.
* New releases are needed on new minors, even if the package doesn't change.
* There are differences between major versions that have influence on test
  results (for example: different GeoIP databases).

Recommendation: Good for cases where package developers need or want to take
a defensive stand on the supported versions and have the capacity to follow
the release train of the Elastic Stack.


### Support for current minor, or to specific versions

Example: `~8.1.0` or `8.1.0`

With this approach, the released package can be used only in the specified
minor.

Pros:
* Testing is limited to more specific versions.
* Even more open to the use of the latest features.

Cons:
* Requires one development branch per version of the Elastic Stack, couples
  package lifecycle to the Elastic Stack, more prone to need backports.
* Releases are needed for each version of the Elastic Stack.
* Stack updates may be affected by updates of these packages, difficult to
  provide compatibility during upgrades.

Recommendation: Good option for packages that are highly coupled to specific
versions of the Elastic Stack, or that may be even bundled with it. Recommended
for packages that are not going to be maintained in future versions, as PoCs,
experiments, or packages that use experimental features of the Elastic Stack.


### Support for all versions since the release of the package

Example: `>=7.16.0`.

With this approach, the released package can be used in any version since the
release of the package.

Pros:
* No need to keep multiple development branches, no need for backports.
* No update needed when a new version of the Elastic Stack is released.

Cons:
* False premise, difficult to ensure compatibility with future majors.
* Deprecation path needed if package removed. It may be needed to remove it from
  the store if orphan of maintainers.

Recommendation: Not recommended. This caused problems in the past in some
packages and was replaced by `^`.
