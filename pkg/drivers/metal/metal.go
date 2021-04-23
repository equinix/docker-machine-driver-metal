// SPDX-License-Identifier: BSD-3-Clause

package metal

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/carmo-evan/strtotime"
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
	defaultOS       = "ubuntu_20_04"
	defaultMetro    = "DC"
)

var (
	// version is set by goreleaser at build time
	version = "devel"

	// build time check that the Driver type implements the Driver interface
	_ drivers.Driver = &Driver{}
)

type Driver struct {
	*drivers.BaseDriver
	ApiKey                  string
	ProjectID               string
	Plan                    string
	HardwareReserverationID string
	Facility                string
	Metro                   string
	OperatingSystem         string
	BillingCycle            string
	DeviceID                string
	UserData                string
	Tags                    []string
	CaCertPath              string
	SSHKeyID                string
	UserDataFile            string
	UserAgentPrefix         string
	SpotInstance            bool
	SpotPriceMax            float64
	TerminationTime         *packngo.Timestamp
}

// NewDriver is a backward compatible Driver factory method.  Using
// new(metal.Driver) is preferred.
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
			Name:   "metal-api-key",
			Usage:  "Equinix Metal API Key",
			EnvVar: "METAL_AUTH_TOKEN",
		},
		mcnflag.StringFlag{
			Name:   "metal-project-id",
			Usage:  "Equinix Metal Project Id",
			EnvVar: "METAL_PROJECT_ID",
		},
		mcnflag.StringFlag{
			Name:   "metal-os",
			Usage:  "Equinix Metal OS",
			Value:  defaultOS,
			EnvVar: "METAL_OS",
		},
		mcnflag.StringFlag{
			Name:   "metal-facility-code",
			Usage:  "Equinix Metal facility code",
			EnvVar: "METAL_FACILITY_CODE",
		},
		mcnflag.StringFlag{
			Name:   "metal-metro-code",
			Usage:  fmt.Sprintf("Equinix Metal metro code (%q is used if empty and facility is not set)", defaultMetro),
			EnvVar: "METAL_METRO_CODE",
			// We don't set Value because Facility was previously required and
			// defaulted. Existing configurations with "Facility" should not
			// break. Setting a default metro value would break those
			// configurations.
		},
		mcnflag.StringFlag{
			Name:   "metal-plan",
			Usage:  "Equinix Metal Server Plan",
			Value:  "baremetal_0",
			EnvVar: "METAL_PLAN",
		},
		mcnflag.StringFlag{
			Name:   "metal-hw-reservation-id",
			Usage:  "Equinix Metal Reserved hardware ID",
			EnvVar: "METAL_HW_ID",
		},
		mcnflag.StringFlag{
			Name:   "metal-billing-cycle",
			Usage:  "Equinix Metal billing cycle, hourly or monthly",
			Value:  "hourly",
			EnvVar: "METAL_BILLING_CYCLE",
		},
		mcnflag.StringFlag{
			Name:   "metal-userdata",
			Usage:  "Path to file with cloud-init user-data",
			EnvVar: "METAL_USERDATA",
		},
		mcnflag.BoolFlag{
			Name:   "metal-spot-instance",
			Usage:  "Request a Equinix Metal Spot Instance",
			EnvVar: "METAL_SPOT_INSTANCE",
		},
		mcnflag.StringFlag{
			Name:   "metal-spot-price-max",
			Usage:  "The maximum Equinix Metal Spot Price",
			EnvVar: "METAL_SPOT_PRICE_MAX",
		},
		mcnflag.StringFlag{
			Name:   "metal-termination-time",
			Usage:  "The Equinix Metal Instance Termination Time",
			EnvVar: "METAL_TERMINATION_TIME",
		},
		mcnflag.StringFlag{
			EnvVar: "METAL_UA_PREFIX",
			Name:   "metal-ua-prefix",
			Usage:  "Prefix the User-Agent in Equinix Metal API calls with some 'product/version'",
		},
	}
}

func (d *Driver) DriverName() string {
	return "metal"
}

func (d *Driver) SetConfigFromFlags(flags drivers.DriverOptions) error {
	if strings.Contains(flags.String("metal-os"), "coreos") {
		d.SSHUser = "core"
	}
	if strings.Contains(flags.String("metal-os"), "rancher") {
		d.SSHUser = "rancher"
	}

	d.ApiKey = flags.String("metal-api-key")
	d.ProjectID = flags.String("metal-project-id")
	d.OperatingSystem = flags.String("metal-os")
	d.Facility = flags.String("metal-facility-code")
	d.Metro = flags.String("metal-metro-code")
	d.BillingCycle = flags.String("metal-billing-cycle")
	d.UserAgentPrefix = flags.String("metal-ua-prefix")
	d.UserDataFile = flags.String("metal-userdata")

	d.Plan = flags.String("metal-plan")
	d.HardwareReserverationID = flags.String("metal-hw-reservation-id")

	d.SpotInstance = flags.Bool("metal-spot-instance")

	if d.SpotInstance {
		SpotPriceMax := flags.String("metal-spot-price-max")
		if SpotPriceMax == "" {
			d.SpotPriceMax = -1
		} else {
			SpotPriceMax, err := strconv.ParseFloat(SpotPriceMax, 64)
			if err != nil {
				return err
			}
			d.SpotPriceMax = SpotPriceMax
		}

		TerminationTime := flags.String("metal-termination-time")
		if TerminationTime == "" {
			d.TerminationTime = nil
		} else {
			Timestamp, err := strtotime.Parse(TerminationTime, time.Now().Unix())
			if err != nil {
				return err
			}
			if Timestamp <= time.Now().Unix() {
				return fmt.Errorf("--metal-termination-time cannot be in the past")
			}
			d.TerminationTime = &packngo.Timestamp{Time: time.Unix(Timestamp, 0)}
		}
	}

	if d.ApiKey == "" {
		return fmt.Errorf("metal driver requires the --metal-api-key option")
	}
	if d.ProjectID == "" {
		return fmt.Errorf("metal driver requires the --metal-project-id option")
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
		return fmt.Errorf("specified --metal-os not one of %v", strings.Join(flavors, ", "))
	}

	if d.Metro == "" && d.Facility == "" {
		d.Metro = defaultMetro
	}

	if d.Metro != "" && d.Facility != "" {
		return fmt.Errorf("facility and metro can not be used together")
	}

	client := d.getClient()

	if d.Metro != "" {
		return validateMetro(client, d.Metro)
	}

	return validateFacility(client, d.Facility)
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
		Metro:                 d.Metro,
		OS:                    d.OperatingSystem,
		BillingCycle:          d.BillingCycle,
		ProjectID:             d.ProjectID,
		UserData:              userdata,
		Tags:                  d.Tags,
		SpotInstance:          d.SpotInstance,
		SpotPriceMax:          d.SpotPriceMax,
		TerminationTime:       d.TerminationTime,
	}

	if d.Facility != "" {
		createRequest.Facility = []string{d.Facility}
	}

	log.Info("Provisioning Equinix Metal server...")
	newDevice, _, err := client.Devices.Create(createRequest)
	if err != nil {
		//cleanup ssh keys if device faild
		if _, err := client.SSHKeys.Delete(d.SSHKeyID); err != nil {
			if er, ok := err.(*packngo.ErrorResponse); !ok || er.Response.StatusCode != http.StatusNotFound {
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

	if _, err := client.Devices.Delete(d.DeviceID, false); err != nil {
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
	client := packngo.NewClientWithAuth(consumerToken, d.ApiKey, nil)
	userAgent := fmt.Sprintf("docker-machine-driver-%s/%s %s", d.DriverName(), version, client.UserAgent)

	if len(d.UserAgentPrefix) > 0 {
		userAgent = fmt.Sprintf("%s %s", d.UserAgentPrefix, userAgent)
	}

	client.UserAgent = userAgent
	return client
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

func validateFacility(client *packngo.Client, facility string) error {
	if facility == "any" {
		return nil
	}

	facilities, _, err := client.Facilities.List(nil)
	if err != nil {
		return err
	}
	for _, f := range facilities {
		if f.Code == facility {
			return nil
		}
	}

	return fmt.Errorf("metal requires a valid facility")
}

func validateMetro(client *packngo.Client, metro string) error {
	metros, _, err := client.Metros.List(nil)
	if err != nil {
		return err
	}
	for _, m := range metros {
		if m.Code == metro {
			return nil
		}
	}

	return fmt.Errorf("metal requires a valid metro")
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
