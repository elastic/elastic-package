# Readme ILM


### Data streams using ILM policies

#### ilm_ds Policy
| Key | Value |
|---|---|
| policy.phases.delete.min_age | 30d |
| policy.phases.hot.actions.rollover.max_age | 30d |
| policy.phases.hot.actions.rollover.max_primary_shard_size | 50gb |

#### lifecycle_ds Policy
| Key | Value |
|---|---|
| data_retention | 30d |

