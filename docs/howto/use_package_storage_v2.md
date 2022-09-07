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
* a package with `version < 1.0.0` is a technical preview.
* a package with `version >= 1.0.0` can contain prerelease tags (beta1, SNAPSHOT, next) on its version to indicate its prerelease state.

### Prerelease and stable version

A flat storage structure has implications for exposing revisions in the Fleet Integrations UI.

The Package Storage v1 allowed for pinning package revisions to particular stages and let package owners decide on when to promote the package.
We received critical feedback about this approach as it required package owners to take a manual action to promote package revisions.
The Package Storage v2 assumes that every published revision is exposed to Fleet users, and package owners can release their packages
as snapshots or technical previews using semantic versioning (prerelease tags).

Notice: if you prefer Fleet users not to pick up and install prereleases, it's recommended to be cautious with exposing new features
until [kibana#122973](https://github.com/elastic/kibana/issues/122973) is implemented. With this feature, users can decide
whether they can pick up prerelease package revisions. If you can't expose new features for any reason, you should stick
to a local development environment instead.

## What is the goal of storage migration from v1 to v2?

We identified a few issues in v1 design, we couldn't easily overcome or patch:
1. Automatically release new Docker images of the Package Storage without missing packages due to a race condition
   between CI jobs.
2. Control the Docker image size, which is constantly growing (as of today, >1GB). Packages with size >1GB must be served through the Package Registry too.
3. Deprecate promotion between stages. It caused a lot of frustration for package developers and most of them didn't follow the recommended promotion path
   `snapshot -> staging -> production`.
4. Enable validation for incoming packages (spec and signatures).
5. Support package signatures. It wasn't possible to calculate the signature for unarchived package directories.

## What should a package owner do to automatically publish their packages?

### Existing packages

Package revisions already deployed in the production Package Storage (present in the `production` branch of the [package-storage](https://github.com/elastic/package-storage))
are automatically synced with the new storage. In this case we disable the validation as some older packages will not be able
to pass current spec requirements.

Sync between v1 and v2 will be enabled until we officially deprecate the v1 storage (no more PRs or promotions).

### Next revisions

Before we deprecate the v1 storage, package owners will have to adjust their releasing pipelines to submit packages
to the new destination. Every package candidate should be submitted together with a corresponding signature, generated
using the [Elastic signing pipeline](https://internal-ci.elastic.co/job/elastic+unified-release+master+sign-artifacts-with-gpg/).

Here is the list of requirements and code modifications based on the `beats-ci`.

#### Requirements

1. CI job signing credentials (`sign-artifacts-with-gpg-job`) - use them to call the signing pipeline on
   the `internal-ci` Jenkins instance. The pipeline will sign artifacts uploaded to the signing bucket and upload there their signatures.
2. Signing bucket credentials (`internal-ci-gcs-plugin`) - use them to upload zipped packages to be signed
   and download matching signatures.
3. Package Storage GCP uploader credentials (`upload-package-to-package-storage`) - use them to upload a package candidate to the "queue" bucket.
   The candidates will be picked by the publishing job and removed after processing.
4. Package Storage uploader secret (`secret/gce/elastic-bekitzur/service-account/package-storage-uploader`) - use it to kick off
   the publishing job to process the uploaded candidate.

#### Code modifications

These code modifications refer to the Jenkinsfile/groovy files, which will orchestrate the Jenkins worker to sign the package
and publish it using the Package Storage publishing job.

Function [packageStoragePublish(...)](https://github.com/elastic/elastic-package/blob/f8f678d20b9b60d438188e8dfd2fb4e7519b5a69/.ci/package-storage-publish.groovy#L70)

##### Sign the package candidate

Function [signUnpublishedArtifactsWithElastic(...)](https://github.com/elastic/elastic-package/blob/f8f678d20b9b60d438188e8dfd2fb4e7519b5a69/.ci/package-storage-publish.groovy#L87-L122).

1. Check if the package has been already published (HTTP request to EPR).
2. Upload the package candidate to the signing bucket.
3. Call the Elastic signing pipeline to create matching signatures. The pipeline signs them using the Elastic private key.
4. Once the job succeeded, download package signatures.

##### Publish the package candidate

Function [uploadUnpublishedToPackageStorage(...)](https://github.com/elastic/elastic-package/blob/f8f678d20b9b60d438188e8dfd2fb4e7519b5a69/.ci/package-storage-publish.groovy#L124-L151).

1. Check if the package has been already published (HTTP request to EPR).
2. Upload the package candidate to the special "queue" bucket - `elastic-bekitzur-package-storage-internal`.
3. Call the [publishing job](https://internal-ci.elastic.co/job/package_storage/job/publishing-job-remote/). The publishing jobs verifies
   correctness of the package format and corresponding signature. Next, the job extracts static resources, uploads the zipped package
   to the public bucket, and schedules indexing in background.