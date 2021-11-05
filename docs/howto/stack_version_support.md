Intro...

TBD: When to reconsider the version constraint.


Support for multiple majors
----
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
many special features, and have low coupling with specific versions of the stack.


Support for current major
----
Examples: `^8.0.0`, `^8.2.0`

With this approach, the released package can be used only in the specified
major.

Pros:
* Testing need to be done with less versions.
* Changes don't need to take into account older versions.
* More open to use newer features of the stack.

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


Support for last minors of previous major and first minors of the next
----
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
the release train of the stack.


Support for current minor, or to specific versions
----
Example: `~8.1.0` or `8.1.0`

With this approach, the released package can be used only in the specified
minor.

Pros:
* Testing is limited to more specific versions.
* Even more open to the use of the latest features.

Cons:
* Requires one development branch per version of the stack, couples package
  lifecycle to the stack, more prone to need backports.
* Releases are needed for each version of the stack.
* Stack updates may be affected by updates of these packages, difficult to
  provide compatibility during upgrades.

Recommendation: Good option for packages that are highly coupled to specific
versions of the stack, or that may be even bundled with it. Recommended
for packages that are not going to be maintained in future versions, as PoCs,
experiments, or packages that use experimental features of the stack.


Support for all versions since the release of the package
----
Example: `>=7.16.0`.

With this approach, the released package can be used in any version since the
release of the package.

Pros:
* No need to keep multiple development branches, no need for backports.
* No update needed when a new version of the stack is released.

Cons:
* False premise, difficult to ensure compatibility with future majors.
* Deprecation path needed if package removed. It may be needed to remove it from
  the store if orphan of maintainers.

Recommendation: Not recommended. This caused problems in the past in some
packages and was replaced by `^`.

