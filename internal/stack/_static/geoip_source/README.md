# Set GeoIP databases for elastic-package

## Introduction

`elastic-package` requires to set specific GeoIP databases with known data to avoid flakiness in tests
when the `geoip` processor is used in Ingest Pipelines.

Because of that, `elastic-package` installs specific GeoIP databases in Elasticsearch when the Elastic stack is started (`elastic-package stack up`).
These databases (`GeoLite2-*.mmdb`) must be located at `internal/stack/_static`.

These databases must also include GeoIP data for the address ranges for documentation for both IPv4 and IPv6 (related issue [elastic-package#2414](https://github.com/elastic/elastic-package/issues/2414)).
Including those documentation ranges allows developers to set documentation IPs in their pipeline or
system tests that could be enriched thanks to the geoip known data added by the `geoip` processor in the ingest pipeline.
The documentation ranges (address blocks) are defined in:
- IPv4: https://datatracker.ietf.org/doc/rfc5737/ and https://datatracker.ietf.org/doc/rfc6676/
- IPv6: https://datatracker.ietf.org/doc/rfc3849/ and https://datatracker.ietf.org/doc/rfc9637/

The ASN to be used for those documentation purposes are defined in [RFC5398](https://datatracker.ietf.org/doc/rfc5398/).

In the following sections, it is described how to build your own custom GeoIP databases.

## Prerequisites

1. Install GoLang 1.24 or later
2. Install `mmdbinspect` and add it your PATH (https://github.com/maxmind/mmdbinspect)

The following section describes the procedure to generate new `mmdb` databases from the JSON source files.

## How to build a new GeoIP database for `elastic-package`

As a note, `elastic-package` used to use the GeoIP databases from
[this commit](https://github.com/maxmind/MaxMind-DB/blob/2bf1713b3b5adcb022cf4bb77eb0689beaadcfef/test-data).
This ensures that tests performed by `elastic-package` will not be failing since no GeoIP data is modified, just new data is added.

The latest JSON Files used to generate the `mmdb` databases are located in `internal/stack/_static/geoip_source`.
Those JSON files already contain the required entries for the documentation ranges with some dummy GeoIP data. And they should be
used as a basis for new changes if required.

If any other changes are required in the GeoIP databases used by elastic-package, update the JSON files located at `internal/stack/_static/geoip_source`
and then generate new `mmdb` files:
```shell
cd path/to/repo/elastic-package

# 1. Add the required data into the JSON files in `geoip_source` (internal/stack/_static/geoip_source)
# 2. Generate mmdb databases (internal/stack/_static)
go run ./tools/geoipdatabases

# If it is needed another directories , it can be used `-source` and `-target` flags.
```

Those databases generated can be tested to ensure that the expected data is returned using the `mmdbinspect` tool.
For instance:
```shell
mmdbinspect -db internal/stack/_static/GeoLite2-ASN.mmdb 192.0.2.100
mmdbinspect -db internal/stack/_static/GeoLite2-Country.mmdb 192.0.2.100
mmdbinspect -db internal/stack/_static/GeoLite2-City.mmdb 192.0.2.100
```

As these GeoIP databases are installed when the stack is bootstrapped, any change in those databases require to
restart the stack:
```shell
elastic-package stack down -v
elastic-package stack up -v -d --version <version>
```
