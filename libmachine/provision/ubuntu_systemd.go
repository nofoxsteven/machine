package provision

import (
	"fmt"
	"strconv"

	"github.com/rancher/machine/libmachine/auth"
	"github.com/rancher/machine/libmachine/drivers"
	"github.com/rancher/machine/libmachine/engine"
	"github.com/rancher/machine/libmachine/log"
	"github.com/rancher/machine/libmachine/mcnutils"
	"github.com/rancher/machine/libmachine/provision/pkgaction"
	"github.com/rancher/machine/libmachine/provision/serviceaction"
	"github.com/rancher/machine/libmachine/swarm"
)

func init() {
	Register("Ubuntu-SystemD", &RegisteredProvisioner{
		New: NewUbuntuSystemdProvisioner,
	})
}

func NewUbuntuSystemdProvisioner(d drivers.Driver) Provisioner {
	return &UbuntuSystemdProvisioner{
		NewSystemdProvisioner("ubuntu", d),
	}
}

type UbuntuSystemdProvisioner struct {
	SystemdProvisioner
}

func (provisioner *UbuntuSystemdProvisioner) String() string {
	return "ubuntu(systemd)"
}

func (provisioner *UbuntuSystemdProvisioner) CompatibleWithHost() bool {
	const FirstUbuntuSystemdVersion = 15.04

	isUbuntu := provisioner.OsReleaseInfo.ID == provisioner.OsReleaseID
	if !isUbuntu {
		return false
	}
	versionNumber, err := strconv.ParseFloat(provisioner.OsReleaseInfo.VersionID, 64)
	if err != nil {
		return false
	}
	return versionNumber >= FirstUbuntuSystemdVersion

}

func (provisioner *UbuntuSystemdProvisioner) Package(name string, action pkgaction.PackageAction) error {
	var packageAction string

	updateMetadata := true

	switch action {
	case pkgaction.Install, pkgaction.Upgrade:
		packageAction = "install"
	case pkgaction.Remove:
		packageAction = "remove"
		updateMetadata = false
	case pkgaction.Purge:
		packageAction = "purge"
		updateMetadata = false
	}

	switch name {
	case "docker":
		name = "docker-ce"
	}

	if updateMetadata {
		if err := waitForLockAptGetUpdate(provisioner); err != nil {
			return err
		}
	}

	lockTimeout := 300
	command := fmt.Sprintf("DEBIAN_FRONTEND=noninteractive sudo -E apt-get -o DPkg::Lock::Timeout=%d %s -y  %s", lockTimeout, packageAction, name)

	log.Debugf("package: action=%s name=%s", action.String(), name)

	_, err := provisioner.SSHCommand(command)
	if err != nil {
		return fmt.Errorf("command failed: %s", err.Error())
	}

	return nil
}

func (provisioner *UbuntuSystemdProvisioner) dockerDaemonResponding() bool {
	log.Debug("checking docker daemon")

	if out, err := provisioner.SSHCommand("sudo docker version"); err != nil {
		log.Warnf("Error getting SSH command to check if the daemon is up: %s", err)
		log.Debugf("'sudo docker version' output:\n%s", out)
		return false
	}

	// The daemon is up if the command worked.  Carry on.
	return true
}

func (provisioner *UbuntuSystemdProvisioner) Provision(swarmOptions swarm.Options, authOptions auth.Options, engineOptions engine.Options) error {
	provisioner.SwarmOptions = swarmOptions
	provisioner.AuthOptions = authOptions
	provisioner.EngineOptions = engineOptions
	swarmOptions.Env = engineOptions.Env

	log.Debug("waiting for cloud-init to complete")
	err := waitForCloudInit(provisioner)
	if err != nil {
		return err
	}

	storageDriver, err := decideStorageDriver(provisioner, DefaultStorageDriver, engineOptions.StorageDriver)
	if err != nil {
		return err
	}
	provisioner.EngineOptions.StorageDriver = storageDriver

	log.Debug("setting hostname")
	if err := provisioner.SetHostname(provisioner.Driver.GetMachineName()); err != nil {
		return err
	}

	log.Debug("installing base packages")
	for _, pkg := range provisioner.Packages {
		if err := provisioner.Package(pkg, pkgaction.Install); err != nil {
			return err
		}
	}

	if err := installDockerGeneric(provisioner, provisioner.EngineOptions.InstallURL, provisioner.EngineOptions.Version); err != nil {
		return err
	}

	log.Debug("waiting for docker daemon")
	if err := mcnutils.WaitFor(provisioner.dockerDaemonResponding); err != nil {
		return err
	}

	provisioner.AuthOptions = setRemoteAuthOptions(provisioner)

	log.Debug("configuring auth")
	if err := ConfigureAuth(provisioner); err != nil {
		return err
	}

	log.Debug("configuring swarm")
	if err := configureSwarm(provisioner, swarmOptions, provisioner.AuthOptions); err != nil {
		return err
	}

	// enable in systemd
	log.Debug("enabling docker in systemd")
	err = provisioner.Service("docker", serviceaction.Enable)
	return err
}
