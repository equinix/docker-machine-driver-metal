# docker-machine-driver-packet
Packet bare-metal cloud driver for Docker Machine called 

> Driver name: `packet`

### Usage

You can provision bare-metal hosts once you have built and installed the docker-machine driver. The binary will be placed in your `$PATH` directory.

Test that the installation worked by typing in:

```
$ docker-machine create --driver packet
```

#### Example usage:

This creates the following: 

* Type0 machine
* in the EWR region (NJ)
* with Ubuntu 16.04
* in project $PROJECT
* Using $API_KEY - [get yours from the Portal](https://app.packet.net/users/me/api-keys)

```
$ docker-machine create sloth \
  --driver packet --packet-api-key=$API_KEY --packet-os=ubuntu_16_04 --packet-project-id=$PROJECT --packet-facility-code "ewr1" --packet-plan "baremetal_0"
  
Creating CA: /home/alex/.docker/machine/certs/ca.pem
Creating client certificate: /home/alex/.docker/machine/certs/cert.pem
Running pre-create checks...
Creating machine...
(sloth) Creating SSH key...
(sloth) Provisioning Packet server...
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

### Building

Pre-reqs: `docker-machine` and `make`

* Install the Golang SDK [https://golang.org/dl/](https://golang.org/dl/) (at least 1.11 required for [modules](https://github.com/golang/go/wiki/Modules) support

* Download the source-code with `git clone http://github.com/packethost/docker-machine-driver-packet.git`

* Build and install the driver:

```
$ cd docker-machine-driver-packet
$ make 
$ sudo make install
```

Now you will now be able to specify a `-driver` of `packet` to `docker-machine` commands.

### Release

Releases are handled by [GitHub Workflows](.github/workflows/release.yml) and [goreleaser](.goreleaser.yml).

To push a new release, checkout the commit that you want released and: `make tag version=v0.2.3`.  Robots handle the rest.

Releases are archived at <https://github.com/packethost/docker-machine-driver-packet/releases>

