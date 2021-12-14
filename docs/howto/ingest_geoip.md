# HOWTO: Use MaxMind's GeoIP database in tests

Elasticsearch provides default GeoIP databases that can be downloaded in runtime and which weights ~70 MB. This can be
a root cause of flakiness of package tests, so elastic-package embeds small samples of GeoIP databases, that can identify
accurately only few ranges of IP addresses included [here](../../internal/fields/_static/allowed_geo_ips.txt)

If you want the ingest pipeline to include a "geo" section in the event, feel free to use one of above IP addresses.
Embedded databases contain information about: cities, countries and ASNs.