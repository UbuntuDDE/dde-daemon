// Code generated by "dbusutil-gen -type Manager network/manager.go"; DO NOT EDIT.

package network

func (v *Manager) setPropState(value uint32) (changed bool) {
	if v.State != value {
		v.State = value
		v.emitPropChangedState(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedState(value uint32) error {
	return v.service.EmitPropertyChanged(v, "State", value)
}

func (v *Manager) setPropConnectivity(value uint32) (changed bool) {
	if v.Connectivity != value {
		v.Connectivity = value
		v.emitPropChangedConnectivity(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedConnectivity(value uint32) error {
	return v.service.EmitPropertyChanged(v, "Connectivity", value)
}

func (v *Manager) setPropNetworkingEnabled(value bool) (changed bool) {
	if v.NetworkingEnabled != value {
		v.NetworkingEnabled = value
		v.emitPropChangedNetworkingEnabled(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedNetworkingEnabled(value bool) error {
	return v.service.EmitPropertyChanged(v, "NetworkingEnabled", value)
}

func (v *Manager) setPropVpnEnabled(value bool) (changed bool) {
	if v.VpnEnabled != value {
		v.VpnEnabled = value
		v.emitPropChangedVpnEnabled(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedVpnEnabled(value bool) error {
	return v.service.EmitPropertyChanged(v, "VpnEnabled", value)
}

func (v *Manager) setPropDevices(value string) (changed bool) {
	if v.Devices != value {
		v.Devices = value
		v.emitPropChangedDevices(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedDevices(value string) error {
	return v.service.EmitPropertyChanged(v, "Devices", value)
}

func (v *Manager) setPropConnections(value string) (changed bool) {
	if v.Connections != value {
		v.Connections = value
		v.emitPropChangedConnections(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedConnections(value string) error {
	return v.service.EmitPropertyChanged(v, "Connections", value)
}

func (v *Manager) setPropActiveConnections(value string) (changed bool) {
	if v.ActiveConnections != value {
		v.ActiveConnections = value
		v.emitPropChangedActiveConnections(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedActiveConnections(value string) error {
	return v.service.EmitPropertyChanged(v, "ActiveConnections", value)
}

func (v *Manager) setPropWirelessAccessPoints(value string) (changed bool) {
	if v.WirelessAccessPoints != value {
		v.WirelessAccessPoints = value
		v.emitPropChangedWirelessAccessPoints(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedWirelessAccessPoints(value string) error {
	return v.service.EmitPropertyChanged(v, "WirelessAccessPoints", value)
}
