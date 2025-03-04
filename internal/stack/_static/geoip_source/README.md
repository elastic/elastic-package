# Set GeoIP databases for elastic-package

`elastic-package` requires to set specific GeoIP databases with known data to avoid flakiness in tests
when the `geoip` processor is used in Ingest Pipelines.

Because of that, `elastic-package` installs specific GeoIP databases in Elasticsearch when the Elastic stack is started (`elastic-package stack up`).
These databases (`GeoLite2-*.mmdb`) are defined here:
- https://github.com/elastic/elastic-package/tree/6c54c5f4a6dcf8f02212d5ff3bc51654649bf8f9/internal/stack/_static

Moreover, these databases should include IPs from the documentation prefixes for both IPv4 and IPv6 to be used in tests.
These ranges are defined in:
- IPv4: https://datatracker.ietf.org/doc/rfc5737/
- IPv6: https://datatracker.ietf.org/doc/rfc3849/

And, the ASN to be used in documentation prefixes are defined in [RFC5398](https://datatracker.ietf.org/doc/rfc5398/)

In the following sections, it is described how to build your own custom GeoIP databases.

## Prerequisites

1. Clone https://github.com/maxmind/MaxMind-DB
2. Install GoLang 1.23 or later
3. Install `mmdbinspect` and add it your PATH (https://github.com/maxmind/mmdbinspect)


## Required changes to be done in MaxMind-DB repository

Requires changes for `pkg/writer/geoip2.go`
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

Required changes for `cmd/write-test-data/main.go`
- Just trigger the creation of GeoLite2 databases
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


## How to build a new GeoIP database for `elastic-package`

As a note, `elastic-package` used to use the GeoIP databases from
[this commit](https://github.com/maxmind/MaxMind-DB/blob/2bf1713b3b5adcb022cf4bb77eb0689beaadcfef/test-data).
To avoid changing any GeoIP data and make the tests fail, it must be kept the same (JSON) source files. That
would ensure that no data is modified and we are just adding new data.

How to generate new GeoIP databases (`*.mmdb`) based on the source JSON files defined in the [MaxMind-DB](https://github.com/maxmind/MaxMind-DB) (`source-data` folder):

```shell

git clone https://github.com/maxmind/MaxMind-DB

cd cmd/write-test-data
go build

cd ../../

# the tool will create the target folder, and it will replace any file that
# could exist in the folder
cmd/write-test-data/write-test-data -source source-data -target my-target-data


# Check the databases generated
mmdbinspect -db internal/stack/_static/GeoLite2-ASN-Test.mmdb 192.0.2.100
mmdbinspect -db internal/stack/_static/GeoLite2-Country-Test.mmdb 192.0.2.100
mmdbinspect -db internal/stack/_static/GeoLite2-City-Test.mmdb 192.0.2.100
```

As mentioned above, `elastic-package` requires to add new entries to add data for the
documentation prefixes.

In `internal/stack/_static/geoip_sources` are saved the current JSON files used to generate
the `mmdb` databases that contain the documentation prefixes with some dummy GeoIP data.
If any other changes are required, those files can be updated and generate new `mmdb` files:
```shell
cd path/to/elastic-package
cd internal/stack_static

# generate mmdb databases with the tool built previously
path/to/cmd/write-test-data/write-test-data -source geoip_sources -target new_databases

# copy new databases to the expected directory
mv new_databases/GeoLite2-ASN-Test.mmdb GeoLite2-ASN.mmdb
mv new_databases/GeoLite2-Country-Test.mmdb GeoLite2-Country.mmdb
mv new_databases/GeoLite2-City-Test.mmdb GeoLite2-City.mmdb
```

Take into account that the files generated by `write-test-data` must be renamed to match the current file names used by `elastic-package`.

As these GeoIP databases are installed when the stack is bootstrapped, any change in those databases require to
restart the stack:
```shell
elastic-package stack down -v
elastic-package stack up -v -d --version <version>
```


