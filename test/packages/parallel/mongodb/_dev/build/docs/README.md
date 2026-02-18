# MongoDB Integration

This integration is used to fetch logs and metrics from [MongoDB](https://www.mongodb.com/).

## Compatibility

This is a lite copy of the original MongoDB package from the integrations repository. The `status` data stream has been tested with MongoDB versions 6 and 7, which are the currently maintained versions according to the MongoDB release lifecycle.

## MongoDB Privileges
In order to use the metrics datasets, the MongoDB user specified in the package
configuration needs to have certain [privileges](https://docs.mongodb.com/manual/core/authorization/#privileges).

We recommend using the [clusterMonitor](https://docs.mongodb.com/manual/reference/built-in-roles/#clusterMonitor) 
role to cover all the necessary privileges.

You can use the following command in Mongo shell to create the privileged user
(make sure you are using the `admin` db by using `db` command in Mongo shell).

```
db.createUser(
    {
        user: "beats",
        pwd: "pass",
        roles: ["clusterMonitor"]
    }
)
```

You can use the following command in Mongo shell to grant the role to an 
existing user (make sure you are using the `admin` db by using `db` command in 
Mongo shell).

```
db.grantRolesToUser("user", ["clusterMonitor"])
```

## Metrics

### status

The `status` returns a document that provides an overview of the database's state.

It requires the following privileges, which is covered by the [clusterMonitor](https://docs.mongodb.com/manual/reference/built-in-roles/#clusterMonitor) role:

* [serverStatus](https://docs.mongodb.com/manual/reference/privilege-actions/#serverStatus) 
action on [cluster resource](https://docs.mongodb.com/manual/reference/resource-document/#cluster-resource)

{{event "status"}}

The fields reported are:

{{fields "status"}}
