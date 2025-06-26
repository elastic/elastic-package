## Package types

There are different types of packages, for different use cases.

Integration packages provide complete monitoring solutions for specific
applications of services. An integration package can be used to configure
multiple data streams and multiple agent inputs, it also includes dashboards
and other assets related to the monitored service.
For example the Apache package includes configurations to collect and store
access and error logs and metrics. It also includes dashboards to leverage the
collected data.

Input packages are intended for generic use cases, where the user needs to
collect custom data. They define how to configure an agent input, so the user
can adapt it to their case. An example is the log input package, that users can
use to collect custom logs, and provide their own processing and mapping.

Content packages include assets for the Elastic Stack, but are not intended to
define how to ingest data. They can include static data for indexes, as
knowledge bases or sample data. Or they can contain resources that can be used
with data collected according to certain conventions, for example dashboards for
data collected by other means.

### What package type to use?

When [creating a new package](./create_new_package.md) you will need to select
the type. If you are not sure, you can follow this section to help deciding:

* Do you plan to provide a whole solution for a given service?
  * If yes, can Fleet manage the collector agent?
    * If yes, create an integration package.
    * If not, create a content package that complements the method/s used for ingestion.
  * If not, do you plan to define how to collect data?
    * If yes, do you plan to define how to collect data in a generic way for a given protocol?
      * If yes, create an input package.
      * If not, create an integration package for the use case.
    * If not, create a content package.

### A note on using integration packages for everything

For legacy reasons, integration packages can be used for most use cases. For
example a package can include dashboards without including any data stream, as
if it were a content package. Or an integration package can define a single data
stream, as if it were an input package.

Even when this is true, these cases push integration packages (and the code
supporting them) beyond what they were designed for. By having different types
we can focus on better supporting this specialized use cases. For example content
packages can be allowed to have bigger sizes, and we can prepare all our infra
and code for them. Or input packages are better designed for custom use cases,
giving a better experience on these cases.

In the future we may be limiting what can be done with integration packages,
providing migration paths to the other types.

