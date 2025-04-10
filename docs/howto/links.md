# HOWTO: Use links to reuse common files.

## Introduction

Many packages have files that are equal between them. This is more common in pipelines, 
input configurations, and field definitions.

In order to help developers, there is the ability to define links, so a file that might be reused needs to only be defined once, and can be reused from any other packages.


# Links

Currently, there are some specific places where links can be defined:

- `elasticsearch/ingest_pipeline`
- `data_stream/**/elasticsearch/ingest_pipeline`
- `agent/input`
- `data_stream/**/agent/stream`
- `data_stream/**/fields`

A link consists of a file with a `.link` extension that contains a path, relative to its location, to the file that it will be replaced with. It also consists of a checksum to validate the linked file is up to date with the package expectations.

`data_stream/foo/elasticsearch/ingest_pipeline/default.yml.link`

```
../../../../../testpackage/data_stream/test/elasticsearch/ingest_pipeline/default.yml f7c5f0c03aca8ef68c379a62447bdafbf0dcf32b1ff2de143fd6878ee01a91ad
```

This will use the contents of the linked file during validation, tests, and building of the package, so functionally nothing changes from the package point of view.

## The `_dev/shared` folder

As a convenience, shared files can be placed under `_dev/shared` if they are going to be
reused from several places. They can even be added outside of any package, in any place in the repository.
