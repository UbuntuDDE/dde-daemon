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

package gesture

import (
	"pkg.deepin.io/dde/daemon/loader"
	"pkg.deepin.io/lib/log"
)

type Daemon struct {
	*loader.ModuleBase
	manager *Manager
}

const (
	dbusServiceName = "com.deepin.daemon.Gesture"
	dbusServicePath = "/com/deepin/daemon/Gesture"
	dbusServiceIFC  = dbusServiceName
)

var (
	logger = log.NewLogger("gesture")
)

func NewDaemon() *Daemon {
	daemon := new(Daemon)
	daemon.ModuleBase = loader.NewModuleBase("gesture", daemon, logger)
	return daemon
}

func init() {
	loader.Register(NewDaemon())
}

func (*Daemon) GetDependencies() []string {
	return []string{}
}

func (d *Daemon) Start() error {
	if d.manager != nil {
		return nil
	}

	var err error
	d.manager, err = newManager()
	if err != nil {
		logger.Error("failed to initialize gesture manager:", err)
		return err
	}

	service := loader.GetService()
	err = service.Export(dbusServicePath, d.manager)
	if err != nil {
		logger.Error("failed to export gesture:", err)
		return err
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		logger.Error("failed to request gesture name:", err)
		d.manager.destroy()
		err1 := service.StopExport(d.manager)
		if err1 != nil {
			logger.Error("failed to StopExport:", err1)
		}
		return err
	}

	d.manager.init()

	return nil
}

func (d *Daemon) Stop() error {
	if xconn != nil {
		xconn.Close()
		xconn = nil
	}

	if d.manager == nil {
		return nil
	}

	d.manager.destroy()
	d.manager = nil
	return nil
}
