# ECS Mappings Test

Test package to verify support for ECS mappings available to integrations running on stack version 8.13.0 and later.

Please note that the package:

- does not embed the legacy ECS mappings (no `import_mappings`).
- does not define fields in the `ecs.yml` file. 

Mappings for ECS fields (for example, `ecs.version`) come from the `ecs@mappings` component template in the integration index template, which has been available since 8.13.0. 

{{event "first"}}

{{fields "first"}}
