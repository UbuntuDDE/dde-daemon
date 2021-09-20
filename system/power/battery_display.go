/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
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

package power

import (
	"time"

	"pkg.deepin.io/dde/api/powersupply/battery"
)

func (m *Manager) refreshBatteryDisplay() {
	logger.Debug("refreshBatteryDisplay")
	defer func() {
		timestamp := time.Now().Unix()
		err := m.service.Emit(m, "BatteryDisplayUpdate", timestamp)
		if err != nil {
			logger.Warning(err)
		}
	}()

	var percentage float64
	var status battery.Status
	var timeToEmpty, timeToFull uint64
	var energyCapacity, energyFullTotal, energyFullDesignTotal float64

	batteryCount := len(m.batteries)
	if batteryCount == 0 {
		m.resetBatteryDisplay()
		return
	} else if batteryCount == 1 {
		var bat0 *Battery
		for _, bat := range m.batteries {
			bat0 = bat
			break
		}

		// copy from bat0
		bat0.PropsMu.RLock()
		percentage = bat0.Percentage
		status = bat0.Status
		timeToEmpty = bat0.TimeToEmpty
		timeToFull = bat0.TimeToFull
		energyFullTotal = bat0.EnergyFull
		energyFullDesignTotal = bat0.EnergyFullDesign
		bat0.PropsMu.RUnlock()
	} else {
		var energyTotal, energyRateTotal float64
		statusSlice := make([]battery.Status, 0, batteryCount)
		for _, bat := range m.batteries {
			bat.PropsMu.RLock()
			energyTotal += bat.Energy
			energyFullTotal += bat.EnergyFull
			energyFullDesignTotal += bat.EnergyFullDesign
			energyRateTotal += bat.EnergyRate
			statusSlice = append(statusSlice, bat.Status)
			bat.PropsMu.RUnlock()
		}
		logger.Debugf("energyTotal: %v", energyTotal)
		logger.Debugf("energyFullTotal: %v", energyFullTotal)
		logger.Debugf("energyRateTotal: %v", energyRateTotal)

		percentage = rightPercentage(energyTotal / energyFullTotal * 100.0)
		status = battery.GetDisplayStatus(statusSlice)

		if energyRateTotal > 0 {
			if status == battery.StatusDischarging {
				timeToEmpty = uint64(3600 * (energyTotal / energyRateTotal))
			} else if status == battery.StatusCharging {
				timeToFull = uint64(3600 * ((energyFullTotal - energyTotal) / energyRateTotal))
			}
		}

		/* check the remaining thime is under a set limit, to deal with broken
		primary batteries rate */
		if timeToEmpty > 240*60*60 { /* ten days for discharging */
			timeToEmpty = 0
		}
		if timeToFull > 20*60*60 { /* 20 hours for charging */
			timeToFull = 0
		}
	}
	m.changeBatteryLowByBatteryPercentage(percentage)
	// report
	m.PropsMu.Lock()
	m.setPropHasBattery(true)
	m.setPropBatteryPercentage(percentage)
	m.setPropBatteryStatus(status)
	m.setPropBatteryTimeToEmpty(timeToEmpty)
	m.setPropBatteryTimeToFull(timeToFull)
	energyCapacity = rightPercentage(energyFullTotal / energyFullDesignTotal * 100.0)
	m.setPropBatteryCapacity(energyCapacity)
	m.PropsMu.Unlock()

	logger.Debugf("BatteryCapacity %.1f%%", energyCapacity)
	logger.Debugf("percentage: %.1f%%", percentage)
	logger.Debug("status:", status, uint32(status))
	logger.Debugf("timeToEmpty %v (%vs), timeToFull %v (%vs)",
		time.Duration(timeToEmpty)*time.Second,
		timeToEmpty,
		time.Duration(timeToFull)*time.Second,
		timeToFull)
}

func (m *Manager) changeBatteryLowByBatteryPercentage(percentage float64) {
	logger.Debug("changeBatteryLowByBatteryPercentage, battery percentage: ", percentage)
	batteryLow := percentage <= lowBatteryThreshold
	if m.batteryLow != batteryLow {
		m.batteryLow = batteryLow
		m.updatePowerSavingMode()
	}
}

func (m *Manager) resetBatteryDisplay() {
	m.PropsMu.Lock()
	m.setPropHasBattery(false)
	m.setPropBatteryPercentage(0)
	m.setPropBatteryStatus(battery.StatusUnknown)
	m.setPropBatteryTimeToEmpty(0)
	m.setPropBatteryTimeToFull(0)
	m.setPropBatteryCapacity(0)
	m.PropsMu.Unlock()
}

func rightPercentage(val float64) float64 {
	if val < 0.0 {
		val = 0.0
	} else if val > 100.0 {
		val = 100.0
	}
	return val
}
