// SPDX-License-Identifier: BSD-3-Clause

package metal

import (
	"errors"
	"fmt"
	"io/fs"
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
	"sigs.k8s.io/yaml"
)

const (
	dockerConfigDir = "/etc/docker"
	consumerToken   = "24e70949af5ecd17fe8e867b335fc88e7de8bd4ad617c0403d8769a376ddea72"
	defaultOS       = "ubuntu_20_04"
	defaultMetro    = "dc"
)

var (
	// version is set by goreleaser at build time
	version = "devel"

	driverName = "metal"

	envAuthToken       = "%s_AUTH_TOKEN"
	envProjectID       = "%s_PROJECT_ID"
	envOS              = "%s_OS"
	envFacilityCode    = "%s_FACILITY_CODE"
	envMetroCode       = "%s_METRO_CODE"
	envPlan            = "%s_PLAN"
	envHwId            = "%s_HW_ID"
	envBillingCycle    = "%s_BILLING_CYCLE"
	envUserdata        = "%s_USERDATA"
	envSpotInstance    = "%s_SPOT_INSTANCE"
	envSpotPriceMax    = "%s_SPOT_PRICE_MAX"
	envTerminationTime = "%s_TERMINATION_TIME"
	envUAPrefix        = "%s_UA_PREFIX"

	// build time check that the Driver type implements the Driver interface
	_ drivers.Driver = &Driver{}
)

func prefix(f string) string {
	return driverName + f
}

func envPrefix(f string) string {
	return fmt.Sprintf(f, strings.ToUpper(driverName))
}

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
			Name:   prefix("-api-key"),
			Usage:  "Equinix Metal API Key",
			EnvVar: envPrefix(envAuthToken),
		},
		mcnflag.StringFlag{
			Name:   prefix("-project-id"),
			Usage:  "Equinix Metal Project Id",
			EnvVar: envPrefix(envProjectID),
		},
		mcnflag.StringFlag{
			Name:   prefix("-os"),
			Usage:  "Equinix Metal OS",
			Value:  defaultOS,
			EnvVar: envPrefix(envOS),
		},
		mcnflag.StringFlag{
			Name:   prefix("-facility-code"),
			Usage:  "Equinix Metal facility code",
			EnvVar: envPrefix(envFacilityCode),
		},
		mcnflag.StringFlag{
			Name:   prefix("-metro-code"),
			Usage:  fmt.Sprintf("Equinix Metal metro code (%q is used if empty and facility is not set)", defaultMetro),
			EnvVar: envPrefix(envMetroCode),
			// We don't set Value because Facility was previously required and
			// defaulted. Existing configurations with "Facility" should not
			// break. Setting a default metro value would break those
			// configurations.
		},
		mcnflag.StringFlag{
			Name:   prefix("-plan"),
			Usage:  "Equinix Metal Server Plan",
			Value:  "c3.small.x86",
			EnvVar: envPrefix(envPlan),
		},
		mcnflag.StringFlag{
			Name:   prefix("-hw-reservation-id"),
			Usage:  "Equinix Metal Reserved hardware ID",
			EnvVar: envPrefix(envHwId),
		},
		mcnflag.StringFlag{
			Name:   prefix("-billing-cycle"),
			Usage:  "Equinix Metal billing cycle, hourly or monthly",
			Value:  "hourly",
			EnvVar: envPrefix(envBillingCycle),
		},
		mcnflag.StringFlag{
			Name:   prefix("-userdata"),
			Usage:  "Path to file with cloud-init user-data",
			EnvVar: envPrefix(envUserdata),
		},
		mcnflag.BoolFlag{
			Name:   prefix("-spot-instance"),
			Usage:  "Request a Equinix Metal Spot Instance",
			EnvVar: envPrefix(envSpotInstance),
		},
		mcnflag.StringFlag{
			Name:   prefix("-spot-price-max"),
			Usage:  "The maximum Equinix Metal Spot Price",
			EnvVar: envPrefix(envSpotPriceMax),
		},
		mcnflag.StringFlag{
			Name:   prefix("-termination-time"),
			Usage:  "The Equinix Metal Instance Termination Time",
			EnvVar: envPrefix(envTerminationTime),
		},
		mcnflag.StringFlag{
			Name:   prefix("-ua-prefix"),
			Usage:  fmt.Sprintf("Prefix the User-Agent in Equinix Metal API calls with some 'product/version' %s %s", version, driverName),
			EnvVar: envPrefix(envUAPrefix),
		},
	}
}

func (d *Driver) DriverName() string {
	return driverName
}

func (d *Driver) setConfigFromFile() error {
	configFile := getConfigFile()

	config := metalSnakeConfig{}

	if raw, err := ioutil.ReadFile(configFile); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	} else if jsonErr := yaml.Unmarshal(raw, &config); jsonErr != nil {
		return jsonErr
	}
	d.Plan = config.Plan
	d.ApiKey = config.AuthToken
	if config.Token != "" {
		d.ApiKey = config.Token
	}
	d.Facility = config.Facility
	d.Metro = config.Metro
	d.OperatingSystem = config.OS
	d.ProjectID = config.ProjectID
	return nil
}

func (d *Driver) SetConfigFromFlags(flags drivers.DriverOptions) error {
	if err := d.setConfigFromFile(); err != nil {
		return err
	}
	// override config file values with command-line values
	for k, p := range map[string]*string{
		prefix("-os"):            &d.OperatingSystem,
		prefix("-api-key"):       &d.ApiKey,
		prefix("-project-id"):    &d.ProjectID,
		prefix("-metro-code"):    &d.Metro,
		prefix("-facility-code"): &d.Facility,
		prefix("-plan"):          &d.Plan,
	} {
		if v := flags.String(k); v != "" {
			*p = v
		}
	}

	if strings.Contains(d.OperatingSystem, "coreos") {
		d.SSHUser = "core"
	}
	if strings.Contains(d.OperatingSystem, "rancher") {
		d.SSHUser = "rancher"
	}

	d.BillingCycle = flags.String(prefix("-billing-cycle"))
	d.UserAgentPrefix = flags.String(prefix("-ua-prefix"))
	d.UserDataFile = flags.String(prefix("-userdata"))
	d.HardwareReserverationID = flags.String(prefix("-hw-reservation-id"))
	d.SpotInstance = flags.Bool(prefix("-spot-instance"))

	if d.SpotInstance {
		SpotPriceMax := flags.String(prefix("-spot-price-max"))
		if SpotPriceMax == "" {
			d.SpotPriceMax = -1
		} else {
			SpotPriceMax, err := strconv.ParseFloat(SpotPriceMax, 64)
			if err != nil {
				return err
			}
			d.SpotPriceMax = SpotPriceMax
		}

		TerminationTime := flags.String(prefix("-termination-time"))
		if TerminationTime == "" {
			d.TerminationTime = nil
		} else {
			Timestamp, err := strtotime.Parse(TerminationTime, time.Now().Unix())
			if err != nil {
				return err
			}
			if Timestamp <= time.Now().Unix() {
				return fmt.Errorf("--%s-termination-time cannot be in the past", driverName)
			}
			d.TerminationTime = &packngo.Timestamp{Time: time.Unix(Timestamp, 0)}
		}
	}

	if d.ApiKey == "" {
		return fmt.Errorf("%s driver requires the --%s-api-key option", driverName, driverName)
	}
	if d.ProjectID == "" {
		return fmt.Errorf("%s driver requires the --%s-project-id option", driverName, driverName)
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
		return fmt.Errorf("specified --%s-os not one of %v", driverName, strings.Join(flavors, ", "))
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

func ignoreStatusCodes(err error, codes ...int) error {
	e, ok := err.(*packngo.ErrorResponse)
	if !ok || e.Response == nil {
		return err
	}

	for _, c := range codes {
		if e.Response.StatusCode == c {
			return nil
		}
	}
	return err
}

func (d *Driver) Remove() error {
	client := d.getClient()
	if _, err := client.SSHKeys.Delete(d.SSHKeyID); ignoreStatusCodes(err, 403, 404) != nil {
		return err
	}

	_, err := client.Devices.Delete(d.DeviceID, false)
	return ignoreStatusCodes(err, 403, 404)
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

	return fmt.Errorf("%s requires a valid facility", driverName)
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

	return fmt.Errorf("%s requires a valid metro", driverName)
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
