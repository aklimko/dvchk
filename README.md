# DVCHK
[![Build Status](https://travis-ci.com/aklimko/dvchk.svg?branch=master)](https://travis-ci.org/aklimko/dvchk)
[![Docker Pulls](https://shields.beevelop.com/docker/pulls/aklimko/dvchk.svg)](https://hub.docker.com/r/aklimko/dvchk)

DVCHK is a tool that checks if there are newer versions of images for your running containers based on semver versioning.

## How to use
```shell
docker run -it --rm -v /var/run/docker.sock:/var/run/docker.sock aklimko/dvchk
```

## TODO
* Option to check versions with the same or less semver precision e.g. for image version 1.2,
which has also versions 1.2.1, 1.3, 1.3.1, 2, 2.1 and 2.1.1, only 1.3, 2 and 2.1 will show up as newer version.
* Additional authentication image list filtering by registry.
* Ignore duplicates
