# DVCHK
[![Build Status](https://travis-ci.com/aklimko/dvchk.svg?branch=master)](https://travis-ci.com/aklimko/dvchk)
[![Docker Pulls](https://img.shields.io/docker/pulls/aklimko/dvchk.svg)](https://hub.docker.com/r/aklimko/dvchk)

DVCHK is a tool that checks if there are newer versions of images for your running containers based on semver versioning.

## How to use
```shell
docker run -it --rm -v /var/run/docker.sock:/var/run/docker.sock aklimko/dvchk
```

## TODO
* Additional authentication image list filtering by registry.
* Ignore duplicates
