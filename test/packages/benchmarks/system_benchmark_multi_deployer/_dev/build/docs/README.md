# Multi-Deployer Benchmark

Test package that exercises both `docker` and `tf` service deployers in system benchmarks.

- **httpjson** data stream: uses a `docker` deployer running a mock HTTP server.
- **logfile** data stream: uses a `tf` deployer with the Terraform `local` provider (no cloud credentials required).
