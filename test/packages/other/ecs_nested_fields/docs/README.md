# ECS nested fields mapping validation test

Test package for https://github.com/elastic/elastic-package/issues/3639.

It declares only the `email.attachments` ECS nested parent field.  During system tests,
Elasticsearch dynamically maps child fields (e.g. `email.attachments.file.name`) when documents
are indexed.  The mapping validator must accept these children when they are valid ECS fields,
rather than failing at the parent path with a false-positive error.
