# Set GeoIP databases for elastic-package

## Introduction

`elastic-package` requires to set specific GeoIP databases with known data to avoid flakiness in tests
when the `geoip` processor is used in Ingest Pipelines.

Because of that, `elastic-package` installs specific GeoIP databases in Elasticsearch when the Elastic stack is started (`elastic-package stack up`).
These databases (`GeoLite2-*.mmdb`) must be located at `internal/stack/_static`.

These databases must also include GeoIP data from documentation prefixes for both IPv4 and IPv6 (related issue [elastic-package#2414](https://github.com/elastic/elastic-package/issues/2414)).
Including those documentation prefixes allows developers to set documentation IPs in their pipeline or
system tests that could be enriched thanks to the geoip known data added by the `geoip` processor in the ingest pipeline.
The documentation prefixes are defined in:
- IPv4: https://datatracker.ietf.org/doc/rfc5737/
- IPv6: https://datatracker.ietf.org/doc/rfc3849/

The ASN to be used for those documentation prefixes are defined in [RFC5398](https://datatracker.ietf.org/doc/rfc5398/).

In the following sections, it is described how to build your own custom GeoIP databases.

## Prerequisites

1. Clone https://github.com/maxmind/MaxMind-DB
2. Install GoLang 1.23 or later
3. Install `mmdbinspect` and add it your PATH (https://github.com/maxmind/mmdbinspect)

The following section describes the changes required to build `write-test-data` tool for the `elastic-package` needs.

### Required changes to be done in MaxMind-DB repository

Before building `write-test-data` tool, it requires to apply some changes in the code
to allow creating our own `elastic-package` databases.

These changes have been tested using the code from [this commit](https://github.com/maxmind/MaxMind-DB/commit/0ec71808b19669e9e1bf5e63a8c83b202d9bd115).

Changes to be applied:
- `pkg/writer/geoip2.go`:
    - Include usage of reserved networks to allow adding documentation prefixes:
      ```diff
      --- pkg/writer/geoip2.go
      +++ pkg/writer/geoip2.go
      @@ -47,12 +35,13 @@ func (w *Writer) WriteGeoIP2TestDB() error {

                      dbWriter, err := mmdbwriter.New(
                              mmdbwriter.Options{
      -                               DatabaseType:        dbType,
      -                               Description:         description,
      -                               DisableIPv4Aliasing: false,
      -                               IPVersion:           6,
      -                               Languages:           languages,
      -                               RecordSize:          28,
      +                               DatabaseType:            dbType,
      +                               Description:             description,
      +                               DisableIPv4Aliasing:     false,
      +                               IPVersion:               6,
      +                               Languages:               languages,
      +                               RecordSize:              28,
      +                               IncludeReservedNetworks: true,
                              },
                      )
                      if err != nil {
      ```
    - Just create GeoLite2 databases:
      ```diff
      --- pkg/writer/geoip2.go
      +++ pkg/writer/geoip2.go
      @@ -16,18 +16,6 @@ import (
       // WriteGeoIP2TestDB writes GeoIP2 test mmdb files.
       func (w *Writer) WriteGeoIP2TestDB() error {
              dbTypes := []string{
      -               "GeoIP2-Anonymous-IP",
      -               "GeoIP2-City",
      -               "GeoIP2-Connection-Type",
      -               "GeoIP2-Country",
      -               "GeoIP2-DensityIncome",
      -               "GeoIP2-Domain",
      -               "GeoIP2-Enterprise",
      -               "GeoIP2-IP-Risk",
      -               "GeoIP2-ISP",
      -               "GeoIP2-Precision-Enterprise",
      -               "GeoIP2-Static-IP-Score",
      -               "GeoIP2-User-Count",
                      "GeoLite2-ASN",
                      "GeoLite2-City",
                      "GeoLite2-Country",
      ```
- `cmd/write-test-data/main.go`
    - Just trigger the creation of GeoLite2 databases:
      ```diff
      --- cmd/write-test-data/main.go
      +++ cmd/write-test-data/main.go
      @@ -21,46 +21,6 @@ func main() {
                      os.Exit(1)
              }

      -       if err := w.WriteIPv4TestDB(); err != nil {
      -               fmt.Printf("writing IPv4 test databases: %+v\n", err)
      -               os.Exit(1)
      -       }
      -
      -       if err := w.WriteIPv6TestDB(); err != nil {
      -               fmt.Printf("writing IPv6 test databases: %+v\n", err)
      -               os.Exit(1)
      -       }
      -
      -       if err := w.WriteMixedIPTestDB(); err != nil {
      -               fmt.Printf("writing IPv6 test databases: %+v\n", err)
      -               os.Exit(1)
      -       }
      -
      -       if err := w.WriteNoIPv4TestDB(); err != nil {
      -               fmt.Printf("writing no IPv4 test databases: %+v\n", err)
      -               os.Exit(1)
      -       }
      -
      -       if err := w.WriteNoMapTestDB(); err != nil {
      -               fmt.Printf("writing no map test databases: %+v\n", err)
      -               os.Exit(1)
      -       }
      -
      -       if err := w.WriteMetadataPointersTestDB(); err != nil {
      -               fmt.Printf("writing metadata pointers test databases: %+v\n", err)
      -               os.Exit(1)
      -       }
      -
      -       if err := w.WriteDecoderTestDB(); err != nil {
      -               fmt.Printf("writing decoder test databases: %+v\n", err)
      -               os.Exit(1)
      -       }
      -
      -       if err := w.WriteDeeplyNestedStructuresTestDB(); err != nil {
      -               fmt.Printf("writing decoder test databases: %+v\n", err)
      -               os.Exit(1)
      -       }
      -
              if err := w.WriteGeoIP2TestDB(); err != nil {
                      fmt.Printf("writing GeoIP2 test databases: %+v\n", err)
                      os.Exit(1)
      ```

Once applied all these changes, you can build the `write-test-data` tool:
```shell
git clone https://github.com/maxmind/MaxMind-DB

cd MaxMind-DB
cd cmd/write-test-data

# Before building it, apply the changes mentioned in this section.

go build

# A new binary `write-test-data` should have been generated in the same directory.
```

This tool is the one to be used in the following section.

## How to build a new GeoIP database for `elastic-package`

As a note, `elastic-package` used to use the GeoIP databases from
[this commit](https://github.com/maxmind/MaxMind-DB/blob/2bf1713b3b5adcb022cf4bb77eb0689beaadcfef/test-data).
This ensures that tests performed by `elastic-package` will not be failing since no GeoIP data is modified, just new data is added.

The latest JSON Files used to generate the `mmdb` databases are located in `internal/stack/_static/geoip_sources`.
Those JSON files already contain the required entries for the documentation prefixes with some dummy GeoIP data. And they should be
used as a basis for new changes if required.

As an example, the following example shows how to generate new GeoIP databases (`*.mmdb`) using the source JSON
files (`source-data` folder) defined in the [MaxMind-DB](https://github.com/maxmind/MaxMind-DB) repository without any modification:

```shell
cd path/to/repo/MaxMind-DB
# NOTE: Let's consider that the binary has already been built, and it is available
# at `cmd/write-test-data/write-test-data`

# This creates the target folder, and it will replace any file that
# could exist in the folder
cmd/write-test-data/write-test-data -source source-data -target my-target-data
```

As mentioned above, `elastic-package` requires to add new entries to add data for the
documentation prefixes.

If any other changes are required in the GeoIP databases used by elastic-package, those JSON files located at `internal/stack/_static/geoip_sources`
can be updated and then new `mmdb` files be generated:
```shell
cd path/to/repo/elastic-package
cd internal/stack/_static

# 1. Add the required data into the JSON files in `geoip_sources`
# 2. Generate mmdb databases with the tool built previously
path/to/cmd/write-test-data/write-test-data -source geoip_sources -target new_databases

# 3. Copy the new databases to the directory that `elastic-package` expects
mv new_databases/GeoLite2-ASN-Test.mmdb GeoLite2-ASN.mmdb
mv new_databases/GeoLite2-Country-Test.mmdb GeoLite2-Country.mmdb
mv new_databases/GeoLite2-City-Test.mmdb GeoLite2-City.mmdb

# 4. Remove new_databases folder
rm -rf new_databases
```

Those databases generated can be tested to ensure that the expected data is returned using the `mmdbinspect` tool.
For instance:
```shell
mmdbinspect -db internal/stack/_static/GeoLite2-ASN-Test.mmdb 192.0.2.100
mmdbinspect -db internal/stack/_static/GeoLite2-Country-Test.mmdb 192.0.2.100
mmdbinspect -db internal/stack/_static/GeoLite2-City-Test.mmdb 192.0.2.100
```

Take into account that the files generated by `write-test-data` must be renamed to match the file names used by `elastic-package`.

As these GeoIP databases are installed when the stack is bootstrapped, any change in those databases require to
restart the stack:
```shell
elastic-package stack down -v
elastic-package stack up -v -d --version <version>
```
