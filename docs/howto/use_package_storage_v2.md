# Use Package Storage v2

## What is the Package Storage v2?

Package Storage v2 is the successor of the [package-storage](https://github.com/elastic/package-storage) Git repository,
and it is composed of Google Cloud buckets, service accounts and jobs responsible for maintenance including publishing and indexing.

The Package Storage v2 is available publicly behind the endpoint: [package-storage.elastic.co](https://package-storage.elastic.co/)
and exposes different package resources:
* zipped packages (e.g. [barracuda-0.2.2.zip](https://storage.googleapis.com/elastic-bekitzur-package-storage/artifacts/packages/barracuda-0.2.2.zip))
* package signatures (e.g. [barracuda-0.2.2.zip.sig](https://storage.googleapis.com/elastic-bekitzur-package-storage/artifacts/packages/barracuda-0.2.2.zip.sig))
* extracted static resources (e.g. [cisco-0.11.5, screenshot](https://storage.googleapis.com/elastic-bekitzur-package-storage/artifacts/static/cisco-0.11.5/img/kibana-cisco-asa.png))

The Package Storage v2 has flat structure. It does not introduce any logic to divide packages into stages or a control panel to promote them.
We recommend to use proper versioning instead and follow these rules:
* a package with `version < 1.0.0` is experimental
* a package with version containing a prerelease tag (beta1, SNAPSHOT, next) is experimental

## What is the goal of storage migration from v1 to v2?

## What should I do to automatically publish packages we own?

### Existing packages

### Next revisions

#### Requirements

#### Code modifications