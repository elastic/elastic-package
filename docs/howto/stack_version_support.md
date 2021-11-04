Intro...

TBD: When to reconsider the version constraint.


Support for all versions since the release of the package
----
Example: `>=7.16.0`.

With this approach, the released package can be used in any version since the
release of the package.

Pros:
* No need to keep multiple development branches, no need for backports.

Cons:
* False premise, difficult to ensure compatibility with future majors.

Recommendation: Not recommended.

Support for multiple majors
----
Example: `^7.14.0 || ^8.0.0`.

With this approach, the released package can be used in any tested major version
since the release of the package.

Pros:
* No need to keep multiple development branches, no need for backports, same
  package for 7.x and 8.x.
* Less disruptive for users.

Cons:
* Testing should be done with more versions.
* May require compatibility code. 
* There are differences between major versions that have influence on test
  results (for example: different GeoIP databases).

Recommendation: Good for simple packages monitoring services that don't need
many special features, and have low coupling with specific versions of the stack.

Support for current major
----
Example: `^8.0.0`

With this approach, the released package can be used only in the specified
major.

Pros:
* Testing need to be done with less versions.
* Changes don't need to take into account older versions.
* More open to use newer features of the stack.

Cons:
* May require multiple development branches and backport to support multiple
  majors. This is specially a problem in repositories with multiple packages.

Recommendation: Good option for packages that use features not available in
older majors, or that introduce breaking changes.

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
versions of the stack, or that may be even bundled with it.
