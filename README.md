# go-work

## Description

* A cron-like app that runs jobs (defined by their commands and optional arguments) at specified intervals
* Jobs are stored in a PostgreSQL database
* Supports multiple concurrent jobs schedulers
* Has a RESTful web interface for adding/removing/listing jobs

## How to start

To start the app, you can either:

* Use the pre-packaged `docker-compose.yml` file,
  which will start the database, app and pgAdmin containers
* Use `docker-compose-environment-only.yml` to start only the database and pgAdmin and run the app through
  the command line interface

### The following environment variables need to be set in both configurations:

1. `POSTGRES_ADMIN_USER` - database administrator username. **Default:** postgres
2. `PGADMIN_DEFAULT_EMAIL` - pgAdmin initial administrator account email. **Default:** pgadmin4@pgadmin.org
3. `PGADMIN_PORT` - port on which pgAdmin will run. **Default:** 5050
4. `PGADMIN_DEFAULT_PASSWORD` - pgAdmin initial administrator account. **Default:** admin
5. `POSTGRES_ADMIN_PASSWORD` - database administrator password
6. `POSTGRES_APP_PASSWORD` - password which the app will use to connect to the database

### Starting everything at once

The `APP_SERVER_PORT` environment variable needs to be set (**Default:** 8080).

```shell
$ docker compose -f docker-compose.yml up
```

By default, schedulers with intervals of 1 and 2 seconds are created.
To change that, change the command line parameters in `docker-compose.yml`

### Starting only the database and using the CLI

#### Parameters:

* `server-port` - Port the app server will start on. **Default:** 8080
* `db-host` - Database host
* `db-port` - Database port. **Default:** 5432
* `interval` - Intervals (in seconds) at which the schedulers will ping the database.
  Multiple schedulers are specified by specifying their corresponding intervals (see below)

```shell
$ docker compose -f docker-compose-environment-only.yml up
$ go mod download
$ go build -o go-work cmd/go-work/main.go
$ ./go-work --server-port <SERVER_PORT> --db-host <DB_HOST> --db-port <DB_PORT> \
--interval <INTERVAL1> --interval <INTERVAL2> ...
```

## How to add/remove/list jobs

Here's an example of a job definition (JSON):

```json
{
  "name": "example_job",
  "crontabString": "15 16 1 */3 *",
  "command": "/home/user/me/check.sh",
  "arguments": [
    "-a",
    "--arg1",
    "--arg2=123"
  ],
  "timeout": 6
}
```

The fields are:

* `name` - Name of the job, a valid identifier (**starts with a letter, cannot contain spaces**, can include letters,
  numbers and underscores)
* `crontabString` - A string that follows the UNIX crontab job definition syntax and specifies when the job should be
  run
* `command` - Command to execute when running the job
* `arguments` - An array of arguments passed to the command. This field is **optional**
* `timeout` - Timeout in seconds, after which the job is terminated

The app has a RESTful api for working with jobs. See the OpenAPI v3 specification in `api/openapi.yml`