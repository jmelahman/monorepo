package systemd

import (
	"fmt"

	"github.com/godbus/dbus/v5"
)

func EnableUnitFiles(obj dbus.BusObject, files []string) error {
	var enableChanged bool
	result := make([][]interface{}, 0)
	err := obj.Call("org.freedesktop.systemd1.Manager.EnableUnitFiles", 0, files, false, true).Store(&enableChanged, &result)
	if err != nil {
		return fmt.Errorf("Failed to enable service %v: %v", files, err)
	}
	return nil
}

func StartUnit(obj dbus.BusObject, serviceName string) error {
	var jobPath dbus.ObjectPath
	err := obj.Call("org.freedesktop.systemd1.Manager.StartUnit", 0, serviceName, "replace").Store(&jobPath)
	if err != nil {
		return fmt.Errorf("Failed to start service %v: %v", serviceName, err)
	}
	return nil
}

func DisableUnitFiles(obj dbus.BusObject, files []string) error {
	result := make([][]interface{}, 0)
	err := obj.Call("org.freedesktop.systemd1.Manager.DisableUnitFiles", 0, files, true).Store(&result)
	if err != nil {
		return fmt.Errorf("Failed to enable service %v: %v", files, err)
	}
	return nil
}

func StopUnit(obj dbus.BusObject, serviceName string) error {
	var jobPath dbus.ObjectPath
	err := obj.Call("org.freedesktop.systemd1.Manager.StopUnit", 0, serviceName, "replace").Store(&jobPath)
	if err != nil {
		return fmt.Errorf("Failed to start service %s: %v", serviceName, err)
	}
	return nil
}
