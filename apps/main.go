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

package apps

import (
	"pkg.deepin.io/dde/daemon/loader"
	"pkg.deepin.io/lib/log"
)

//go:generate dbusutil-gen em -type ALRecorder,DFWatcher

var logger = log.NewLogger("daemon/apps")

func init() {
	loader.Register(NewDaemon(logger))
}

type Daemon struct {
	*loader.ModuleBase
	recorder *ALRecorder
	watcher  *DFWatcher
}

func NewDaemon(logger *log.Logger) *Daemon {
	daemon := new(Daemon)
	daemon.ModuleBase = loader.NewModuleBase("apps", daemon, logger)
	return daemon
}

func (d *Daemon) Start() error {
	service := loader.GetService()

	var err error
	d.watcher, err = newDFWatcher(service)
	if err != nil {
		return err
	}

	d.recorder, err = newALRecorder(d.watcher)
	if err != nil {
		return err
	}

	// export recorder and watcher
	err = service.Export(dbusPath, d.recorder, d.watcher)
	if err != nil {
		return err
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		return err
	}
	d.recorder.emitServiceRestarted()

	return nil
}

func (d *Daemon) Stop() error {
	// TODO
	return nil
}

func (d *Daemon) GetDependencies() []string {
	return []string{}
}

func (d *Daemon) Name() string {
	return "apps"
}
