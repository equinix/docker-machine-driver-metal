# docker-machine-driver-metal

[![GitHub release](https://img.shields.io/github/release/equinix/docker-machine-driver-metal/all.svg?style=flat-square)](https://github.com/equinix/docker-machine-driver-metal/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/equinix/docker-machine-driver-metal)](https://goreportcard.com/report/github.com/equinix/docker-machine-driver-metal)
[![Equinix Community](https://img.shields.io/badge/Equinix%20Community%20-%20%23E91C24?logo=equinixmetal)](https://community.equinix.com)

The [Equinix Metal](https://metal.equinix.com) cloud bare-metal machine driver for Docker.

# Note
With the upcoming EoL of Equinix Metal on June 30, 2026, this repo is being archived on February 28, 2026.

## Usage

Provision bare-metal hosts by either building and installing this docker-machine driver or downloading the latest [prebuilt release asset](https://github.com/equinix/docker-machine-driver-metal/releases) for your platform. The binaries must be placed in your `$PATH`.

Test that the installation worked by typing in:

```sh
docker-machine create --driver metal
```

You can find the supported arguments by running `docker-machine create -d metal --help` (Equinix Metal specific arguments are shown below):

| Argument                    | Default        | Description                                                                  | Environment              | Config                  |
| --------------------------- | -------------- | ---------------------------------------------------------------------------- | ------------------------ | ----------------------- |
| `--metal-api-key`           |                | Deprecated API Key flag (use auth token)                                    | `METAL_API_KEY`          |
| `--metal-auth-token`        |                | Equinix Metal Authentication Token                                           | `METAL_AUTH_TOKEN`       | `token` or `auth-token` |
| `--metal-billing-cycle`     | `hourly`       | Equinix Metal billing cycle, hourly or monthly                               | `METAL_BILLING_CYCLE`    |
| `--metal-facility-code`     |                | Equinix Metal facility code                                                  | `METAL_FACILITY_CODE`    | `facility`              |
| `--metal-hw-reservation-id` |                | Equinix Metal Reserved hardware ID                                           | `METAL_HW_ID`            |
| `--metal-metro-code`        |                | Equinix Metal metro code ("dc" is used if empty and facility is not set)     | `METAL_METRO_CODE`       | `metro`                 |
| `--metal-os`                | `ubuntu_20_04` | Equinix Metal OS                                                             | `METAL_OS`               | `operating-system`      |
| `--metal-plan`              | `c3.small.x86` | Equinix Metal Server Plan                                                    | `METAL_PLAN`             | `plan`                  |
| `--metal-project-id`        |                | Equinix Metal Project Id                                                     | `METAL_PROJECT_ID`       | `project`               |
| `--metal-spot-instance`     |                | Request a Equinix Metal Spot Instance                                        | `METAL_SPOT_INSTANCE`    |
| `--metal-spot-price-max`    |                | The maximum Equinix Metal Spot Price                                         | `METAL_SPOT_PRICE_MAX`   |
| `--metal-termination-time`  |                | The Equinix Metal Instance Termination Time                                  | `METAL_TERMINATION_TIME` |
| `--metal-ua-prefix`         |                | Prefix the User-Agent in Equinix Metal API calls with some 'product/version' | `METAL_UA_PREFIX`        |
| `--metal-userdata`          |                | Path to file with cloud-init user-data                                       | `METAL_USERDATA`         |

Where denoted, values may be loaded from the environment or from the `~/.config/equinix/metal.yaml` file which can be created with the [Equinix Metal CLI](https://github.com/equinix/metal-cli#metal-cli).

In order to support existing installations, a Packet branded binary is also available with each [release](https://github.com/equinix/docker-machine-driver-metal/releases) (after v0.5.0). When the `packet` binary is used, all `METAL` environment variables and `metal` arguments should be substituted for `PACKET` and `packet`, respectively.

### Example usage

This creates the following:

- c3.small.x86 machine
- in the NY metro
- with Ubuntu 20.04
- in project $PROJECT
- Using $API_KEY - [get yours from the Portal](https://console.equinix.com/users/me/api-keys)

```sh
$ docker-machine create sloth \
  --driver metal --metal-api-key=$API_KEY --metal-os=ubuntu_20_04 --metal-project-id=$PROJECT --metal-metro-code "ny" --metal-plan "c3.small.x86"

Creating CA: /home/alex/.docker/machine/certs/ca.pem
Creating client certificate: /home/alex/.docker/machine/certs/cert.pem
Running pre-create checks...
Creating machine...
(sloth) Creating SSH key...
(sloth) Provisioning Equinix Metal server...
(sloth) Created device ID $DEVICE, IP address 147.x.x.x
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

- Install the Golang SDK [https://golang.org/dl/](https://golang.org/dl/) (at least 1.11 required for [modules](https://github.com/golang/go/wiki/Modules) support

- Download the source-code with `git clone http://github.com/equinix/docker-machine-driver-metal.git`

- Build and install the driver:

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
  --metal-auth-token=$METAL_AUTH_TOKEN \
  --metal-project-id=$METAL_PROJECT \
  foo
```

### Release Process

This project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

Releases are handled by [GitHub Workflows](.github/workflows/release.yml) and [goreleaser](.goreleaser.yml).

To push a new release, checkout the commit that you want released and: `make tag version=v0.2.3`. Robots handle the rest.

Maintainers should verify that the release notes convey to users all of the notable changes between releases, in a human readable way.
The format for each release should be based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/).

## Releases and Changes

See <https://github.com/equinix/docker-machine-driver-metal/releases> for the latest releases, install archives, and the project changelog.
