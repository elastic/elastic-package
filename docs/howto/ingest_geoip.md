# HOWTO: Use MaxMind's GeoIP database in tests

Elasticsearch provides default GeoIP databases that can be downloaded in runtime and which weights ~70 MB. This can be
a root cause of flakiness of package tests, so elastic-package embeds small samples of GeoIP databases, that can identify
accurately only few ranges of IP addresses.

Specifically, the following documentation ranges of IP addresses are included in those GeoIP databases:
- [RFC5737](https://datatracker.ietf.org/doc/rfc5737/)
    - 192.0.2.0/24
    - 198.51.100.0/24
    - 203.0.113.0/24
- [RFC6676](https://datatracker.ietf.org/doc/rfc6676/) (multicast addresses allocated for documentation purposes):
    - 233.252.0.0/24
- [RFC3849](https://datatracker.ietf.org/doc/rfc3849/)
    - "2001:DB8::/32"
- [RFC9637](https://datatracker.ietf.org/doc/rfc9637/)
    - "3fff::/20"

If you want the ingest pipeline to include a "geo" section in the event, feel free to use one of above IP addresses.
Embedded databases contain information about: cities, countries and ASNs.