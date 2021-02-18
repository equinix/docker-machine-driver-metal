# docker-machine-driver-metal

[![GitHub release](https://img.shields.io/github/release/equinix/docker-machine-driver-metal/all.svg?style=flat-square)](https://github.com/equinix/docker-machine-driver-metal/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/equinix/docker-machine-driver-metal)](https://goreportcard.com/report/github.com/equinix/docker-machine-driver-metal)
[![Slack](https://slack.equinixmetal.com/badge.svg)](https://slack.equinixmetal.com)
[![Twitter Follow](https://img.shields.io/twitter/follow/equinixmetal.svg?style=social&label=Follow)](https://twitter.com/intent/follow?screen_name=equinixmetal)
![](https://img.shields.io/badge/Stability-Maintained-green.svg)

The [Equinix Metal](https://metal.equinix.com) cloud bare-metal machine driver for Docker.

This repository is [Maintained](https://github.com/packethost/standards/blob/master/maintained-statement.md) meaning that this software is supported by Equinix Metal and its community - available to use in production environments.

## Usage

You can provision bare-metal hosts once you have built and installed the docker-machine driver. The binary will be placed in your `$PATH` directory.

Test that the installation worked by typing in:

```sh
docker-machine create --driver metal
```

### Example usage

This creates the following:

* Type0 machine
* in the EWR region (NJ)
* with Ubuntu 16.04
* in project $PROJECT
* Using $API_KEY - [get yours from the Portal](https://console.equinix.com/users/me/api-keys)

```sh
$ docker-machine create sloth \
  --driver metal --metal-api-key=$API_KEY --metal-os=ubuntu_16_04 --metal-project-id=$PROJECT --metal-facility-code "ewr1" --metal-plan "baremetal_0"
  
Creating CA: /home/alex/.docker/machine/certs/ca.pem
Creating client certificate: /home/alex/.docker/machine/certs/cert.pem
Running pre-create checks...
Creating machine...
(sloth) Creating SSH key...
(sloth) Provisioning Equinix Metal server...
(sloth) Created device ID $PROJECT, IP address 147.x.x.x
(sloth) Waiting for Provisioning...
Waiting for machine to be running, this may take a few minutes...
Detecting operating system of created instance...
Waiting for SSH to be available...
Detecting the provisioner...
Provisioning with ubuntu(systemd)...
Installing Docker...
Copying certs to the local machine directory...
Copying certs to the remote machine...
Setting Docker configuration on the remote daemon...
Checking connection to Docker...
Docker is up and running!
To see how to connect your Docker Client to the Docker Engine running on this virtual machine, run: docker-machine env sloth
```

> Provision time can take several minutes

At this point you can now `docker-machine env sloth` and then start using your Docker bare-metal host!

## Development

### Building

Pre-reqs: `docker-machine` and `make`

* Install the Golang SDK [https://golang.org/dl/](https://golang.org/dl/) (at least 1.11 required for [modules](https://github.com/golang/go/wiki/Modules) support

* Download the source-code with `git clone http://github.com/equinix/docker-machine-driver-metal.git`

* Build and install the driver:

```sh
cd docker-machine-driver-metal
make
sudo make install
```

Now you will now be able to specify a `-driver` of `metal` to `docker-machine` commands.

### Debugging

To monitor the Docker debugging details and the Equinix Metal API calls:

```sh
go build
PACKNGO_DEBUG=1 PATH=`pwd`:$PATH docker-machine \
  --debug create -d metal \
  --metal-api-key=$METAL_TOKEN \
  --metal-project-id=$METAL_PROJECT \
  foo
```

### Release Process

This project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

Releases are handled by [GitHub Workflows](.github/workflows/release.yml) and [goreleaser](.goreleaser.yml).

To push a new release, checkout the commit that you want released and: `make tag version=v0.2.3`.  Robots handle the rest.

Maintainers should verify that the release notes convey to users all of the notable changes between releases, in a human readable way.
The format for each release should be based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/).

## Releases and Changes

See <https://github.com/equinix/docker-machine-driver-metal/releases> for the latest releases, install archives, and the project changelog.
