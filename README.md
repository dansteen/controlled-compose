# controlled-compose
Re-implementation of docker-compose that allows greater control over the container startup process.

Docker-Compose is a great tool.  However, when trying to use it in a CI/CD pipeline, it has a few issues.   Most noticeable of these is the lack of ability to manage the startup process of the containers in your composition.   Docker-compose will start up the containers in order, however it will not wait for applications to be "ready", and it will happily continue starting containers in the event of failures up the chain.   Docker-compose does have the option to --abort-on-container-exit, but sometimes containers are running one-off configuration commands, and are supposed to exit.  These as, well as a few other issues, led to the creation of controlled-compose - an application that gives you control over how your containers are started, what constitutes a successful or failed start, and what events to wait for prior to continue starting containers.  

Controlled-compose uses [libcompose](https://github.com/docker/libcompose) for most of it's docker interaction.  

# Features

Controlled-compose adds the following features:

- Add a "require" config stanza that allows requiring other compose files as prerequisites to the current one.
- Set success and fail exit codes for containers. Or specify that they should not exit.
- Set a timeout after which a container will be marked succeeded or failed.
- List on STDOUT or STDERR for regex to indicate success or failure
- Monitor a file for regex to indicate success or failure
- Adjust "volumes:" stanza paths to be relative to the CWD rather than the location of the compose file

# Commands

Controlled-compose currently implements only a subset of the full docker-compose commands.  Currently we implement the following:

- rm
- up
- build

# Compose File Reference

controlled-compose adds some additional config stanzas to the compose-file specification.


| Stanza | Parent |  Description
| ------ | ----- | -----------
| require | None |  Pull in the file mentioned as a prerequisite to this file.  File paths are either absolute or relative to the referencing file.
| state_conditions | service name | The parent config stanza for our state conditions

## Available State Conditions
The following state conditions are currently available to control the compose run.


| Condition | Parameters | Values | Description 
| --------- | ---------- | ----------- |
| exit      |  None      |        | An array of exit codes to treat as success.  Any exit not included in this list will result in a failure.  The special value "-1" is used to indicate that **any** exit should be considered a failure (i.e. the process is supposed to continue to run)
| timeout   |            |        | Only give the supplied amount of time prior to `state` returned
|           | duration   |        | Value in seconds to wait prior to `state` being returned
|           | status     | failure &#124; success | Which state to return after the timeout triggers
| filemonitor |          |        | Monitor files for STDIN or STDOUT for `regex` and return `state`.  This is provided as an array as multiple files can be monitored.
|             | file     | &lt;filename&gt; &#124; STDIN &#124; STDOUT | The name of the file to monitor or the literal strings STDIN or STDOUT.  In the event a file is supplied, the path should be give inside the docker container.  If this path is not exported as a volume, it will be automatically added to the export list and exported to a folder name composed of the project name and the PID.
|             | regex    |        | The regular expression to monitor the file for.
|             | status   | success &#124; failure | The status to act on if the regex is found

## Examples
This demonstrates how to use controlled-compose to start a postgres container and wait until it is ready prior to starting subsequent containers.  This will start the postgres container, and expect it to keep running.  It will then monitor STDOUT until it finds a string that matches the supplied regex.  If it does not find the regex in 60 seconds, or if the process exits.  The container will fail and subsequent containers will not be started.  If it finds the regex, it will move on to starting other containers:


### postgres.yml
```
version: '2'
services:
  db.local:
    image: postgres:9.3
    environment:
      POSTGRES_USER: user
      POSTGRES_DB: dbname
      POSTGRES_PASSWORD: password
    state_conditions:
      exit: [-1]
      timeout:
        duration: 60
        status: failure
      filemonitor:
        - file: STDOUT
          regex: PostgreSQL init process complete; ready for start up.
          status: success
```

This example builds on the previous example and demonstrates how to run one-off configuration commands.  It will run the command, and expect it to exit with an exit code of 0.  It will wait 10 seconds for it to finish, and if it has not completed in that time, will fail:

### config-postgres.yml

```
require: postgres.yml
version: '2'
services:
  config.db.tmp:
    image: postgres:9.3
    environment:
      PGPASSWORD: password
      PGUSER: user
      PGHOST: db.local
      PGDATABASE: dbname
    entrypoint: psql -t -c 'CREATE DATABASE newdb WITH OWNER user'
    state_conditions:
      exit: [0]
      timeout:
        duration: 10
        status: failure
    depends_on:
      - db.local
```


This example builds on the above, and starts up an application that uses the databases created previously.  It starts the application, and expects it to keep running.   It monitors the file /var/log/application.log for the supplied regex, and if it finds it, continues starting subsequent containers.  If it does not find it in 300 seconds it exits with a failure and subsequent containers are not started.  Note that the file path provided is the path to the file **inside** the docker container.  However, the actual monitoring occurs **outside** of the container, so we need to export that path as a volume.  If that path is not exported in the "volumes" stanza already, it will be automatically added with an unique mountpoint.

### application.yml
```
require: config-postgres.yml
version: '2'
services:
  org-api.app.local:
    image: application:prod
    volumes:
      - ./logs:/var/log/
    depends_on:
      - config.db.tmp
    state_conditions:
      exit: [-1]
      filemonitor:
        - file: /var/log/application.log
          regex: Started application@[[a-z0-9]+{HTTP/1.1}{0.0.0.0:[0-9]+}
          status: success
      timeout:
        duration: 300
        status: failure
```
