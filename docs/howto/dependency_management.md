# HOWTO: Enable dependency management

## Motivation

As the package universe keeps growing, there are more occurrences of fields reusing by different integrations, especially
ones basing on the [Elastic Common Schema](https://github.com/elastic/ecs) (ECS). Without dependency management in place
developers tended to copy over same field definitions (mostly ECS related) from one integration to another, leading to
an increase of repository size and accidentally introducing inconsistencies. As there was no single source of truth defining
which field definition was correct, maintenance and typo correction process was expensive.

The described situation brought us to a point in time when a simple dependency management was a requirement to maintain
all used fields, especially ones imported from external sources.

## Principles of operation

Currently Elastic Packages support build-time dependencies that can be used as external field sources. They use a flat
dependency model represented with an additional build manifest, stored in an optional YAML file - `_dev/build/build.yml`:

```yaml
dependencies:
  ecs:
    reference: git@<commit SHA or Git tag>
```

When the elastic-package builds the package, it uses the build manifest to construct a dependencies map with references.

## External fields

While the builder processes fields files and encounters references to external sources, for example:

```yaml
- name: event.category
  external: ecs
- name: event.created
  external: ecs
- name: user_agent.os.full
  external: ecs
```

... it will try to resolve them using the prepared dependencies map and replace with actual definitions (importing).
The tool will try to download and cache locally referenced schemas (e.g. `git@0b8b7d6121340e99a1eb463c91fd1bc7c9eb2e41` or `git@1.10`).
Cached files are stored in a dedicated directory - `~/.elastic-package/cache/fields/`. It's assumed that schema (versioned) files
do not change.

To verify if building process went well, you can open `build` directory and compare fields (e.g. `./build/packages/nginx/1.2.3/access/fields/ecs.yml`):

```yaml
- description: |-
    This is one of four ECS Categorization Fields, and indicates the second level in the ECS category hierarchy.
    `event.category` represents the "big buckets" of ECS categories. For example, filtering on `event.category:process` yields all events relating to process activity. This field is closely related to `event.type`, which is used as a subcategory.
    This field is an array. This will allow proper categorization of some events that fall in multiple categories.
  name: event.category
  type: keyword
- description: |-
    event.created contains the date/time when the event was first read by an agent, or by your pipeline.
    This field is distinct from @timestamp in that @timestamp typically contain the time extracted from the original event.
    In most situations, these two timestamps will be slightly different. The difference can be used to calculate the delay between your source generating an event, and the time when your agent first processed it. This can be used to monitor your agent's or pipeline's ability to keep up with your event source.
    In case the two timestamps are identical, @timestamp should be used.
  name: event.created
  type: date
- description: Operating system name, including the version or code name.
  name: user_agent.os.full
  type: keyword
```

Fields in output fields files are stored sorted in alphabetical order.

### ECS repository

This dependency type refers to the ECS repository and allows for importing fields (name, type, description) from the common schema.
The schema is imported from the generated artifact (`generated/beats/fields.ecs.yml`) and it depends on a Git tag or a commit SHA.

To import fields from ECS v1.9, prepare the following `build.yml` file:

```yaml
dependencies:
  ecs:
    reference: git@1.9
```

and use a following field definition:

```yaml
- name: event.category
  external: ecs
```