/*
 * Copyright (C) 2013 ~ 2018 Deepin Technology Co., Ltd.
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

package timedate

import "pkg.deepin.io/lib/gsettings"

func (m *Manager) listenPropChanged() {
	m.td.InitSignalExt(m.systemSigLoop, true)
	err := m.td.CanNTP().ConnectChanged(func(hasValue bool, value bool) {
		if !hasValue {
			return
		}
		logger.Debug("property CanTTP changed to", value)

		m.PropsMu.Lock()
		m.setPropCanNTP(value)
		m.PropsMu.Unlock()
	})
	if err != nil {
		logger.Warning(err)
	}

	err = m.td.NTP().ConnectChanged(func(hasValue bool, value bool) {
		if !hasValue {
			return
		}
		logger.Debug("property NTP changed to", value)

		m.PropsMu.Lock()
		m.setPropNTP(value)
		m.PropsMu.Unlock()
	})
	if err != nil {
		logger.Warning(err)
	}

	err = m.td.LocalRTC().ConnectChanged(func(hasValue bool, value bool) {
		if !hasValue {
			return
		}
		logger.Debug("property LocalRTC changed to", value)

		m.PropsMu.Lock()
		m.setPropLocalRTC(value)
		m.PropsMu.Unlock()
	})
	if err != nil {
		logger.Warning(err)
	}

	err = m.td.Timezone().ConnectChanged(func(hasValue bool, value string) {
		if !hasValue {
			return
		}
		logger.Debug("property Timezone changed to", value)
		m.PropsMu.Lock()
		m.setPropTimezone(value)
		m.PropsMu.Unlock()

		m.AddUserTimezone(m.Timezone)
	})
	if err != nil {
		logger.Warning(err)
	}

	m.setter.InitSignalExt(m.systemSigLoop, true)
	err = m.setter.NTPServer().ConnectChanged(func(hasValue bool, value string) {
		if !hasValue {
			return
		}

		m.PropsMu.Lock()
		m.setPropNTPServer(value)
		m.PropsMu.Unlock()
	})
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) handleGSettingsChanged() {
	gsettings.ConnectChanged(timeDateSchema, settingsKey24Hour, func(key string) {
		value := m.settings.GetBoolean(settingsKey24Hour)
		err := m.userObj.SetUse24HourFormat(0, value)
		if err != nil {
			logger.Warning(err)
		}
	})
}
