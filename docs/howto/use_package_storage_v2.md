# Use Package Storage v2

## What is the Package Storage v2?

Package Storage v2 is the successor of the [package-storage](https://github.com/elastic/package-storage) Git repository,
and it is composed of Google Cloud buckets, service accounts and jobs responsible for maintenance including publishing and indexing.

The Package Storage v2 is available publicly behind the endpoint: [package-storage.elastic.co](https://package-storage.elastic.co/)
and exposes different package resources:
* zipped packages (e.g. [barracuda-0.2.2.zip](https://package-storage.elastic.co/artifacts/packages/barracuda-0.2.2.zip))
* package signatures (e.g. barracuda-0.2.2 [signature](https://package-storage.elastic.co/artifacts/packages/barracuda-0.2.2.zip.sig))
* extracted static resources (e.g. cisco-0.11.5 [screenshot](https://package-storage.elastic.co/artifacts/static/cisco-0.11.5/img/kibana-cisco-asa.png))

The Package Storage v2 has flat structure. It does not introduce any logic to divide packages into stages or a control panel to promote them.
We recommend to use proper versioning instead and follow these rules:
* a package with `version < 1.0.0` is experimental
* a package with version containing a prerelease tag (beta1, SNAPSHOT, next) is experimental

## What is the goal of storage migration from v1 to v2?

We identified a few issues in v1 design, we couldn't easily overcome or patch:
1. Automatically release new Docker images of the Package Storage without missing packages due to a race condition
   between CI jobs.
2. Controle the Docker image size, which is constantly growing (as of today, >1GB). Packages with size >1GB must be served through the Package Registry too.
3. Deprecate promotion between stages. It caused a lot of frustration for customers and most of them didn't follow the recommended promotion path
   `snapshot -> staging -> production`.
4. Enable validation for incoming packages (spec and signatures).
5. Support package signatures. It wasn't possible to calculate the signature for unarchived package directories.

## What should a package owner do to automatically publish their packages?

### Existing packages

### Next revisions

#### Requirements

#### Code modifications