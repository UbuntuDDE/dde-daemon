/*
 * Copyright (C) 2014 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package bluetooth

import (
	"sort"
	"strings"
	"time"

	"pkg.deepin.io/lib/utils"
)

type config struct {
	core utils.Config

	Adapters map[string]*adapterConfig // use adapter hardware address as key
	Devices  map[string]*deviceConfig  // use adapter address/device address as key

	Discoverable bool `json:"discoverable"`
}

type adapterConfig struct {
	Powered bool
}

type deviceConfig struct {
	// add icon info to mark device type
	Icon string
	// device connect status
	Connected bool
	// record latest time to do compare with other devices
	LatestTime int64
}

// add address message
type deviceConfigWithAddress struct {
	Icon       string
	Connected  bool
	LatestTime int64
	Address    string
}

func newConfig() (c *config) {
	c = &config{}
	c.core.SetConfigName("bluetooth")
	logger.Info("load bluetooth config file:", c.core.GetConfigFile())
	c.Adapters = make(map[string]*adapterConfig)
	c.Devices = make(map[string]*deviceConfig)
	c.Discoverable = true
	c.load()
	return
}

func (c *config) load() {
	err := c.core.Load(c)
	if err != nil {
		logger.Warning(err)
	}
}

func (c *config) save() {
	err := c.core.Save(c)
	if err != nil {
		logger.Warning(err)
	}
}

func newAdapterConfig() (ac *adapterConfig) {
	ac = &adapterConfig{Powered: false}
	return
}

func (c *config) clearSpareConfig(b *Bluetooth) {
	var adapterAddresses []string
	// key is adapter address
	var adapterDevicesMap = make(map[string][]*device)

	b.adaptersLock.Lock()
	for _, adapter := range b.adapters {
		adapterAddresses = append(adapterAddresses, adapter.address)
	}
	b.adaptersLock.Unlock()

	for _, adapterAddr := range adapterAddresses {
		adapterDevicesMap[adapterAddr] = b.getAdapterDevices(adapterAddr)
	}

	c.core.Lock()
	// remove spare adapters
	for addr := range c.Adapters {
		if !isStringInArray(addr, adapterAddresses) {
			delete(c.Adapters, addr)
		}
	}

	// remove spare devices
	for addr := range c.Devices {
		addrParts := strings.SplitN(addr, "/", 2)
		if len(addrParts) != 2 {
			delete(c.Devices, addr)
			continue
		}
		adapterAddr := addrParts[0]
		deviceAddr := addrParts[1]

		devices := adapterDevicesMap[adapterAddr]
		var foundDevice bool
		for _, device := range devices {
			if device.Address == deviceAddr {
				foundDevice = true
				break
			}
		}

		if !foundDevice {
			delete(c.Devices, addr)
			continue
		}
	}

	c.core.Unlock()
}

func (c *config) addAdapterConfig(address string) {
	if c.isAdapterConfigExists(address) {
		return
	}

	c.core.Lock()
	c.Adapters[address] = newAdapterConfig()
	c.core.Unlock()
	c.save()
}

func (c *config) isAdapterConfigExists(address string) (ok bool) {
	c.core.Lock()
	defer c.core.Unlock()
	_, ok = c.Adapters[address]
	return
}

func (c *config) getAdapterConfigPowered(address string) (powered bool) {
	c.core.Lock()
	defer c.core.Unlock()
	if ac, ok := c.Adapters[address]; ok {
		return ac.Powered
	}
	return false
}

func (c *config) setAdapterConfigPowered(address string, powered bool) {
	c.core.Lock()
	if ac, ok := c.Adapters[address]; ok {
		ac.Powered = powered
	}
	c.core.Unlock()
	c.save()
}

func newDeviceConfig() (ac *deviceConfig) {
	ac = &deviceConfig{Connected: false}
	return
}

func (c *config) isDeviceConfigExist(address string) (ok bool) {
	c.core.Lock()
	defer c.core.Unlock()
	_, ok = c.Devices[address]
	return
}

// add device detail info into config file
func (c *config) addDeviceConfig(addDevice *device) {
	// check if device exist
	if c.isDeviceConfigExist(addDevice.getAddress()) {
		return
	}
	c.core.Lock()
	// save device info
	deviceInfo := newDeviceConfig()
	deviceInfo.Icon = addDevice.Icon
	// connect status is set false as default,so device has not been connected yet
	deviceInfo.LatestTime = 0
	deviceInfo.Connected = addDevice.connected
	//add device info to devices map
	c.Devices[addDevice.getAddress()] = deviceInfo
	c.core.Unlock()
	c.save()
}

func (c *config) getDeviceConfig(address string) (dc *deviceConfig, ok bool) {
	c.core.Lock()
	defer c.core.Unlock()
	dc, ok = c.Devices[address]
	return
}

func (c *config) getDeviceConfigConnected(address string) (connected bool) {
	dc, ok := c.getDeviceConfig(address)
	if !ok {
		return
	}

	c.core.Lock()
	defer c.core.Unlock()
	return dc.Connected
}

func (c *config) setDeviceConfigConnected(device *device, connected bool) {
	if device == nil {
		return
	}
	dc, ok := c.getDeviceConfig(device.getAddress())
	if !ok {
		return
	}

	c.core.Lock()
	dc.Connected = connected
	// when status is connect, set connected status as true, update latest time
	dc.Connected = connected
	dc.Icon = device.Icon
	if connected {
		dc.LatestTime = time.Now().Unix()
	}

	c.core.Unlock()

	c.save()
}

// select latest devices from devAddressMap, each type only contain one device
func (c *config) filterDemandedTypeDevices(devAddressMap map[string]*device) []*device {
	var typeDeviceConfigSlice []*deviceConfigWithAddress

	// find latest devices to fill ordered type device
	for _, deviceUnit := range devAddressMap {
		// if device's address is empty, ignore this device
		if deviceUnit.getAddress() == "" {
			continue
		}

		// get device info from config devices according to address
		devConfig := c.Devices[deviceUnit.getAddress()]
		if devConfig == nil {
			continue
		}

		// only paired but not connected devices allowed to auto connect
		if !deviceUnit.Paired || deviceUnit.connected {
			continue
		}
		typeDeviceConfigSlice = append(typeDeviceConfigSlice, &deviceConfigWithAddress{
			Icon:       devConfig.Icon,
			Connected:  devConfig.Connected,
			LatestTime: devConfig.LatestTime,
			Address:    deviceUnit.getAddress(),
		})
	}

	// sort device according to latest connected time
	sort.SliceStable(typeDeviceConfigSlice, func(pre, next int) bool {
		return typeDeviceConfigSlice[pre].LatestTime > typeDeviceConfigSlice[next].LatestTime
	})

	// add all filtered devices to device list
	var deviceList []*device
	for _, devCfg := range typeDeviceConfigSlice {
		// check if type devices is nil
		if devCfg == nil {
			continue
		}
		logger.Debug("devAddressMap", devCfg.LatestTime, devCfg.Address, devCfg.Connected)
		// add auto connect device to list
		deviceList = append(deviceList, devAddressMap[devCfg.Address])
	}
	logger.Debugf("all auto connect device is %v", deviceList)

	return deviceList
}
