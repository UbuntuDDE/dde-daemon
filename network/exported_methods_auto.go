// Code generated by "dbusutil-gen em -type Manager,SecretAgent"; DO NOT EDIT.

package network

import (
	"pkg.deepin.io/lib/dbusutil"
)

func (v *Manager) GetExportedMethods() dbusutil.ExportedMethods {
	return dbusutil.ExportedMethods{
		{
			Name:    "ActivateAccessPoint",
			Fn:      v.ActivateAccessPoint,
			InArgs:  []string{"uuid", "apPath", "devPath"},
			OutArgs: []string{"connection"},
		},
		{
			Name:    "ActivateConnection",
			Fn:      v.ActivateConnection,
			InArgs:  []string{"uuid", "devPath"},
			OutArgs: []string{"cpath"},
		},
		{
			Name:   "DeactivateConnection",
			Fn:     v.DeactivateConnection,
			InArgs: []string{"uuid"},
		},
		{
			Name:   "DebugChangeAPChannel",
			Fn:     v.DebugChangeAPChannel,
			InArgs: []string{"band"},
		},
		{
			Name:   "DeleteConnection",
			Fn:     v.DeleteConnection,
			InArgs: []string{"uuid"},
		},
		{
			Name:   "DisableWirelessHotspotMode",
			Fn:     v.DisableWirelessHotspotMode,
			InArgs: []string{"devPath"},
		},
		{
			Name:   "DisconnectDevice",
			Fn:     v.DisconnectDevice,
			InArgs: []string{"devPath"},
		},
		{
			Name:   "EnableDevice",
			Fn:     v.EnableDevice,
			InArgs: []string{"devPath", "enabled"},
		},
		{
			Name:   "EnableWirelessHotspotMode",
			Fn:     v.EnableWirelessHotspotMode,
			InArgs: []string{"devPath"},
		},
		{
			Name:    "GetAccessPoints",
			Fn:      v.GetAccessPoints,
			InArgs:  []string{"path"},
			OutArgs: []string{"apsJSON"},
		},
		{
			Name:    "GetActiveConnectionInfo",
			Fn:      v.GetActiveConnectionInfo,
			OutArgs: []string{"acinfosJSON"},
		},
		{
			Name:    "GetAutoProxy",
			Fn:      v.GetAutoProxy,
			OutArgs: []string{"proxyAuto"},
		},
		{
			Name:    "GetProxy",
			Fn:      v.GetProxy,
			InArgs:  []string{"proxyType"},
			OutArgs: []string{"host", "port"},
		},
		{
			Name:    "GetProxyIgnoreHosts",
			Fn:      v.GetProxyIgnoreHosts,
			OutArgs: []string{"ignoreHosts"},
		},
		{
			Name:    "GetProxyMethod",
			Fn:      v.GetProxyMethod,
			OutArgs: []string{"proxyMode"},
		},
		{
			Name:    "GetSupportedConnectionTypes",
			Fn:      v.GetSupportedConnectionTypes,
			OutArgs: []string{"types"},
		},
		{
			Name:    "IsDeviceEnabled",
			Fn:      v.IsDeviceEnabled,
			InArgs:  []string{"devPath"},
			OutArgs: []string{"enabled"},
		},
		{
			Name:    "IsWirelessHotspotModeEnabled",
			Fn:      v.IsWirelessHotspotModeEnabled,
			InArgs:  []string{"devPath"},
			OutArgs: []string{"enabled"},
		},
		{
			Name:    "ListDeviceConnections",
			Fn:      v.ListDeviceConnections,
			InArgs:  []string{"devPath"},
			OutArgs: []string{"connections"},
		},
		{
			Name:   "RequestIPConflictCheck",
			Fn:     v.RequestIPConflictCheck,
			InArgs: []string{"ip", "ifc"},
		},
		{
			Name: "RequestWirelessScan",
			Fn:   v.RequestWirelessScan,
		},
		{
			Name:   "SetAutoProxy",
			Fn:     v.SetAutoProxy,
			InArgs: []string{"proxyAuto"},
		},
		{
			Name:   "SetDeviceManaged",
			Fn:     v.SetDeviceManaged,
			InArgs: []string{"devPathOrIfc", "managed"},
		},
		{
			Name:   "SetProxy",
			Fn:     v.SetProxy,
			InArgs: []string{"proxyType", "host", "port"},
		},
		{
			Name:   "SetProxyIgnoreHosts",
			Fn:     v.SetProxyIgnoreHosts,
			InArgs: []string{"ignoreHosts"},
		},
		{
			Name:   "SetProxyMethod",
			Fn:     v.SetProxyMethod,
			InArgs: []string{"proxyMode"},
		},
	}
}
func (v *SecretAgent) GetExportedMethods() dbusutil.ExportedMethods {
	return dbusutil.ExportedMethods{
		{
			Name:   "CancelGetSecrets",
			Fn:     v.CancelGetSecrets,
			InArgs: []string{"connectionPath", "settingName"},
		},
		{
			Name:   "DeleteSecrets",
			Fn:     v.DeleteSecrets,
			InArgs: []string{"connectionData", "connectionPath"},
		},
		{
			Name:    "GetSecrets",
			Fn:      v.GetSecrets,
			InArgs:  []string{"connectionData", "connectionPath", "settingName", "hints", "flags"},
			OutArgs: []string{"secretsData"},
		},
		{
			Name:   "SaveSecrets",
			Fn:     v.SaveSecrets,
			InArgs: []string{"connectionData", "connectionPath"},
		},
		{
			Name:   "SaveSecretsDeepin",
			Fn:     v.SaveSecretsDeepin,
			InArgs: []string{"connectionData", "connectionPath"},
		},
	}
}
