# Integration With Linked Policy Template Path

Manual fixture for composable integrations where `agent/input/owned.hbs` is produced
from `owned.hbs.link` at build time. The manifest uses `template_path: owned.hbs`
(the materialized filename), matching what Fleet expects after `elastic-package build`.
