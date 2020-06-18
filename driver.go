// SPDX-License-Identifier: BSD-3-Clause

package packet

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/docker/machine/libmachine/drivers"
	"github.com/docker/machine/libmachine/log"
	"github.com/docker/machine/libmachine/mcnflag"
	"github.com/docker/machine/libmachine/ssh"
	"github.com/docker/machine/libmachine/state"
	"github.com/packethost/packngo"
)

const (
	dockerConfigDir = "/etc/docker"
	consumerToken   = "24e70949af5ecd17fe8e867b335fc88e7de8bd4ad617c0403d8769a376ddea72"
)

var _ drivers.Driver = &Driver{}

type Driver struct {
	*drivers.BaseDriver
	ApiKey                  string
	ProjectID               string
	Plan                    string
	HardwareReserverationID string
	Facility                string
	OperatingSystem         string
	BillingCycle            string
	DeviceID                string
	UserData                string
	Tags                    []string
	CaCertPath              string
	SSHKeyID                string
	UserDataFile            string
	SpotInstance            bool
	SpotPriceMax            float64
	TerminationTime         *packngo.Timestamp
}

// NewDriver is a backward compatible Driver factory method.  Using
// new(packet.Driver) is preferred.
func NewDriver(hostName, storePath string) *Driver {
	return &Driver{
		BaseDriver: &drivers.BaseDriver{
			MachineName: hostName,
			StorePath:   storePath,
		},
	}
}

func (d *Driver) GetCreateFlags() []mcnflag.Flag {
	return []mcnflag.Flag{
		mcnflag.StringFlag{
			Name:   "packet-api-key",
			Usage:  "Packet api key",
			EnvVar: "PACKET_API_KEY",
		},
		mcnflag.StringFlag{
			Name:   "packet-project-id",
			Usage:  "Packet Project Id",
			EnvVar: "PACKET_PROJECT_ID",
		},
		mcnflag.StringFlag{
			Name:   "packet-os",
			Usage:  "Packet OS",
			Value:  "ubuntu_16_04",
			EnvVar: "PACKET_OS",
		},
		mcnflag.StringFlag{
			Name:   "packet-facility-code",
			Usage:  "Packet facility code",
			Value:  "ewr1",
			EnvVar: "PACKET_FACILITY_CODE",
		},
		mcnflag.StringFlag{
			Name:   "packet-plan",
			Usage:  "Packet Server Plan",
			Value:  "baremetal_0",
			EnvVar: "PACKET_PLAN",
		},
		mcnflag.StringFlag{
			Name:   "packet-hw-reservation-id",
			Usage:  "Packet Reserved hardware ID",
			EnvVar: "PACKET_HW_ID",
		},
		mcnflag.StringFlag{
			Name:   "packet-billing-cycle",
			Usage:  "Packet billing cycle, hourly or monthly",
			Value:  "hourly",
			EnvVar: "PACKET_BILLING_CYCLE",
		},
		mcnflag.StringFlag{
			Name:   "packet-userdata",
			Usage:  "Path to file with cloud-init user-data",
			EnvVar: "PACKET_USERDATA",
		},
		mcnflag.BoolFlag{
			Name:   "packet-spot-instance",
			Usage:  "Request a Packet Spot Instance",
			EnvVar: "PACKET_SPOT_INSTANCE",
		},
		mcnflag.StringFlag{
			Name:   "packet-spot-price-max",
			Usage:  "The maximum Packet Spot Price",
			EnvVar: "PACKET_SPOT_PRICE_MAX",
		},
		mcnflag.StringFlag{
			Name:   "packet-termination-time",
			Usage:  "The Packet Instance Termination Time",
			EnvVar: "PACKET_TERMINATION_TIME",
		},
	}
}

func (d *Driver) DriverName() string {
	return "packet"
}

func (d *Driver) SetConfigFromFlags(flags drivers.DriverOptions) error {
	if strings.Contains(flags.String("packet-os"), "coreos") {
		d.SSHUser = "core"
	}
	if strings.Contains(flags.String("packet-os"), "rancher") {
		d.SSHUser = "rancher"
	}

	d.ApiKey = flags.String("packet-api-key")
	d.ProjectID = flags.String("packet-project-id")
	d.OperatingSystem = flags.String("packet-os")
	d.Facility = flags.String("packet-facility-code")
	d.BillingCycle = flags.String("packet-billing-cycle")
	d.UserDataFile = flags.String("packet-userdata")

	d.Plan = flags.String("packet-plan")
	d.HardwareReserverationID = flags.String("packet-hw-reservation-id")

	d.SpotInstance = flags.Bool("packet-spot-instance")

	if d.SpotInstance == true {
		SpotPriceMax := flags.String("packet-spot-price-max")
		if SpotPriceMax == "" {
			d.SpotPriceMax = -1
		} else {
			SpotPriceMax, err := strconv.ParseFloat(SpotPriceMax, 64)
			if err != nil {
				return err
			}
			d.SpotPriceMax = SpotPriceMax
		}

		TerminationTime := flags.String("packet-termination-time")
		if TerminationTime == "" {
			d.TerminationTime = nil
		} else {
			layout := "2006-01-02T15:04:05.000Z"
			TerminationTime, err := time.Parse(layout, TerminationTime)
			if err != nil {
				return err
			}
			d.TerminationTime.Time = TerminationTime
		}
	}

	if d.ApiKey == "" {
		return fmt.Errorf("packet driver requires the --packet-api-key option")
	}
	if d.ProjectID == "" {
		return fmt.Errorf("packet driver requires the --packet-project-id option")
	}

	return nil
}

func (d *Driver) GetSSHHostname() (string, error) {
	return d.GetIP()
}

func (d *Driver) PreCreateCheck() error {
	if d.UserDataFile != "" {
		if _, err := os.Stat(d.UserDataFile); os.IsNotExist(err) {
			return fmt.Errorf("user-data file %s could not be found", d.UserDataFile)
		}
	}

	flavors, err := d.getOsFlavors()
	if err != nil {
		return err
	}
	if !stringInSlice(d.OperatingSystem, flavors) {
		return fmt.Errorf("specified --packet-os not one of %v", strings.Join(flavors, ", "))
	}

	if d.Facility == "any" {
		return nil
	}

	client := d.getClient()
	facilities, _, err := client.Facilities.List(nil)
	if err != nil {
		return err
	}
	for _, facility := range facilities {
		if facility.Code == d.Facility {
			return nil
		}
	}

	return fmt.Errorf("packet requires a valid facility")
}

func (d *Driver) Create() error {
	var userdata string
	if d.UserDataFile != "" {
		buf, err := ioutil.ReadFile(d.UserDataFile)
		if err != nil {
			return err
		}
		userdata = string(buf)
	}

	log.Info("Creating SSH key...")

	key, err := d.createSSHKey()
	if err != nil {
		return err
	}

	d.SSHKeyID = key.ID

	hardwareReservationId := ""
	//check if hardware reservation requested
	if d.HardwareReserverationID != "" {
		hardwareReservationId = d.HardwareReserverationID
	}

	client := d.getClient()
	createRequest := &packngo.DeviceCreateRequest{
		Hostname:              d.MachineName,
		Plan:                  d.Plan,
		HardwareReservationID: hardwareReservationId,
		Facility:              []string{d.Facility},
		OS:                    d.OperatingSystem,
		BillingCycle:          d.BillingCycle,
		ProjectID:             d.ProjectID,
		UserData:              userdata,
		Tags:                  d.Tags,
		SpotInstance:          d.SpotInstance,
		SpotPriceMax:          -1,
	}

	log.Info("Provisioning Packet server...")
	newDevice, _, err := client.Devices.Create(createRequest)
	if err != nil {
		//cleanup ssh keys if device faild
		if _, err := client.SSHKeys.Delete(d.SSHKeyID); err != nil {
			if er, ok := err.(*packngo.ErrorResponse); !ok || er.Response.StatusCode != 404 {
				return err
			}
		}
		return err
	}
	t0 := time.Now()

	d.DeviceID = newDevice.ID

	for {
		newDevice, _, err = client.Devices.Get(d.DeviceID, nil)
		if err != nil {
			return err
		}

		for _, ip := range newDevice.Network {
			if ip.Public && ip.AddressFamily == 4 {
				d.IPAddress = ip.Address
			}
		}

		if d.IPAddress != "" {
			break
		}

		time.Sleep(1 * time.Second)
	}

	log.Infof("Created device ID %s, IP address %s",
		newDevice.ID,
		d.IPAddress)

	log.Info("Waiting for Provisioning...")
	stage := float32(0)
	for {
		newDevice, _, err = client.Devices.Get(d.DeviceID, nil)
		if err != nil {
			return err
		}
		if newDevice.State == "provisioning" && stage != newDevice.ProvisionPer {
			stage = newDevice.ProvisionPer
			log.Debugf("Provisioning %v%% complete", newDevice.ProvisionPer)
		}
		if newDevice.State == "active" {
			log.Debugf("Device State: %s", newDevice.State)
			break
		}
		time.Sleep(10 * time.Second)
	}

	log.Debugf("Provision time: %v.", time.Since(t0))

	log.Debug("Waiting for SSH...")
	if err := drivers.WaitForSSH(d); err != nil {
		return err
	}

	return nil
}

func (d *Driver) createSSHKey() (*packngo.SSHKey, error) {
	sshKeyPath := d.GetSSHKeyPath()
	log.Debugf("Writing SSH Key to %s", sshKeyPath)

	if err := ssh.GenerateSSHKey(sshKeyPath); err != nil {
		return nil, err
	}

	publicKey, err := ioutil.ReadFile(sshKeyPath + ".pub")
	if err != nil {
		return nil, err
	}

	createRequest := &packngo.SSHKeyCreateRequest{
		Label: fmt.Sprintf("docker machine: %s", d.MachineName),
		Key:   string(publicKey),
	}

	key, _, err := d.getClient().SSHKeys.Create(createRequest)
	if err != nil {
		return key, err
	}

	return key, nil
}

func (d *Driver) GetURL() (string, error) {
	ip, err := d.GetIP()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("tcp://%s:2376", ip), nil
}

func (d *Driver) GetIP() (string, error) {
	if d.IPAddress == "" {
		return "", fmt.Errorf("IP address is not set")
	}
	return d.IPAddress, nil
}

func (d *Driver) GetState() (state.State, error) {
	device, _, err := d.getClient().Devices.Get(d.DeviceID, nil)
	if err != nil {
		return state.Error, err
	}

	switch device.State {
	case "queued", "provisioning", "powering_on":
		return state.Starting, nil
	case "active":
		return state.Running, nil
	case "powering_off":
		return state.Stopping, nil
	case "inactive":
		return state.Stopped, nil
	}
	return state.None, nil
}

func (d *Driver) Start() error {
	_, err := d.getClient().Devices.PowerOn(d.DeviceID)
	return err
}

func (d *Driver) Stop() error {
	_, err := d.getClient().Devices.PowerOff(d.DeviceID)
	return err
}

func (d *Driver) Remove() error {
	client := d.getClient()

	if _, err := client.SSHKeys.Delete(d.SSHKeyID); err != nil {
		if er, ok := err.(*packngo.ErrorResponse); !ok || er.Response.StatusCode != 404 {
			return err
		}
	}

	if _, err := client.Devices.Delete(d.DeviceID); err != nil {
		if er, ok := err.(*packngo.ErrorResponse); !ok || er.Response.StatusCode != 404 {
			return err
		}
	}
	return nil
}

func (d *Driver) Restart() error {
	_, err := d.getClient().Devices.Reboot(d.DeviceID)
	return err
}

func (d *Driver) Kill() error {
	_, err := d.getClient().Devices.PowerOff(d.DeviceID)
	return err
}

func (d *Driver) GetDockerConfigDir() string {
	return dockerConfigDir
}

func (d *Driver) getClient() *packngo.Client {
	return packngo.NewClientWithAuth(consumerToken, d.ApiKey, nil)
}

func (d *Driver) getOsFlavors() ([]string, error) {
	operatingSystems, _, err := d.getClient().OperatingSystems.List()
	if err != nil {
		return nil, err
	}

	supportedDistros := []string{
		"centos",
		"coreos",
		"debian",
		"opensuse",
		"rancher",
		"ubuntu",
	}
	flavors := make([]string, 0, len(operatingSystems))
	for _, flavor := range operatingSystems {
		if stringInSlice(flavor.Distro, supportedDistros) {
			flavors = append(flavors, flavor.Slug)
		}
	}
	return flavors, nil
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
