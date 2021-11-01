# HOWTO: Use MaxMind's GeoIP database in tests

Elasticsearch provides default GeoIP databases that can be downloaded in runtime and which weights ~70 MB. This can be
a root cause of flakiness of package tests, so elastic-package embeds small samples of GeoIP databases, that can identify
accurately only few ranges of IP addresses:

```
1.128.3.4
175.16.199.1
216.160.83.57
216.160.83.61
67.43.156.12
81.2.69.143
81.2.69.144
81.2.69.145
81.2.69.193
89.160.20.112
89.160.20.156
67.43.156.12
67.43.156.13
67.43.156.14
67.43.156.15
2a02:cf40:add:4002:91f2:a9b2:e09a:6fc6
```

If you want the ingest pipeline to include a "geo" section in the event, feel free to use one of above IP addresses.
Embedded databases contain information about: cities, countries and ASNs.