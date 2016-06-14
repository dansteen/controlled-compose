# controlled-compose
Re-implemention of docker-compose that allows greater control over the containers that are started.

This uses [libcompose](https://github.com/docker/libcompose) for most of it's docker interaction.  Note that this is very much a work-in progress, and should not yet be considered really usable as there is no real cli yet.

#Todo:
- Figure out how to use libcompose's networking feature
- Build a CLI
- Add additional functions (other than just "up")
