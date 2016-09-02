#!/usr/bin/env bash
rocker build --auth $DOCKER_USER:$DOCKER_PASSWORD --push -var GIT_COMMIT=${TRAVIS_COMMIT} -var ARCH=amd64 -var ${VERSIONARGS} .
rocker build --auth $DOCKER_USER:$DOCKER_PASSWORD --push -var GIT_COMMIT=${TRAVIS_COMMIT} -var ARCH=arm ${VERSIONARGS} .
    