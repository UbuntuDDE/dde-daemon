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

package audio

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	dbus "github.com/godbus/dbus"
	"golang.org/x/xerrors"
	"pkg.deepin.io/dde/daemon/common/dsync"
	gio "pkg.deepin.io/gir/gio-2.0"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/dbusutil/gsprop"
	"pkg.deepin.io/lib/pulse"
)

const (
	gsSchemaAudio                 = "com.deepin.dde.audio"
	gsKeyFirstRun                 = "first-run"
	gsKeyInputVolume              = "input-volume"
	gsKeyOutputVolume             = "output-volume"
	gsKeyHeadphoneOutputVolume    = "headphone-output-volume"
	gsKeyHeadphoneUnplugAutoPause = "headphone-unplug-auto-pause"
	gsKeyVolumeIncrease           = "volume-increase"
	gsKeyReduceNoise              = "reduce-input-noise"

	gsSchemaSoundEffect  = "com.deepin.dde.sound-effect"
	gsKeyEnabled         = "enabled"
	gsKeyDisableAutoMute = "disable-auto-mute"

	dbusServiceName = "com.deepin.daemon.Audio"
	dbusPath        = "/com/deepin/daemon/Audio"
	dbusInterface   = dbusServiceName

	cmdSystemctl  = "systemctl"
	cmdPulseaudio = "pulseaudio"

	increaseMaxVolume = 1.5
	normalMaxVolume   = 1.0
)

var (
	defaultInputVolume           = 0.1
	defaultOutputVolume          = 0.5
	defaultHeadphoneOutputVolume = 0.17
	gMaxUIVolume                 float64
)

//go:generate dbusutil-gen -type Audio,Sink,SinkInput,Source,Meter -import github.com/godbus/dbus audio.go sink.go sinkinput.go source.go meter.go
//go:generate dbusutil-gen em -type Audio,Sink,SinkInput,Source,Meter

func objectPathSliceEqual(v1, v2 []dbus.ObjectPath) bool {
	if len(v1) != len(v2) {
		return false
	}
	for i, e1 := range v1 {
		if e1 != v2[i] {
			return false
		}
	}
	return true
}

func isStrvEqual(l1, l2 []string) bool {
	if len(l1) != len(l2) {
		return false
	}

	sort.Strings(l1)
	sort.Strings(l2)
	for i, v := range l1 {
		if v != l2[i] {
			return false
		}
	}
	return true
}

type Audio struct {
	service *dbusutil.Service
	PropsMu sync.RWMutex
	// dbusutil-gen: equal=objectPathSliceEqual
	SinkInputs []dbus.ObjectPath
	// dbusutil-gen: equal=objectPathSliceEqual
	Sinks []dbus.ObjectPath
	// dbusutil-gen: equal=objectPathSliceEqual
	Sources                 []dbus.ObjectPath
	DefaultSink             dbus.ObjectPath
	DefaultSource           dbus.ObjectPath
	Cards                   string
	CardsWithoutUnavailable string
	BluetoothAudioMode      string // ????????????
	// dbusutil-gen: equal=isStrvEqual
	BluetoothAudioModeOpts []string // ?????????????????????

	// dbusutil-gen: ignore
	IncreaseVolume gsprop.Bool `prop:"access:rw"`
	// dbusutil-gen: ignore
	ReduceNoise  gsprop.Bool `prop:"access:rw"`
	defaultPaCfg defaultPaConfig

	// ????????????
	MaxUIVolume float64 // readonly

	headphoneUnplugAutoPause bool

	settings  *gio.Settings
	ctx       *pulse.Context
	eventChan chan *pulse.Event
	stateChan chan int

	// ?????????????????????????????????
	sinkInputs        map[uint32]*SinkInput
	defaultSink       *Sink
	defaultSource     *Source
	sinks             map[uint32]*Sink
	sources           map[uint32]*Source
	defaultSinkName   string
	defaultSourceName string
	meters            map[string]*Meter
	mu                sync.Mutex
	quit              chan struct{}

	oldCards CardList // cards??????????????????????????????????????????Port?????????????????????
	cards    CardList

	isSaving     bool
	sourceIdx    uint32 //used to disable source if select a2dp profile
	saverLocker  sync.Mutex
	enableSource bool //can not enable a2dp Source if card profile is "a2dp"

	portLocker sync.Mutex

	syncConfig     *dsync.Config
	sessionSigLoop *dbusutil.SignalLoop

	noRestartPulseAudio bool

	// ??????????????????
	inputCardName string
	inputPortName string
	// ???????????????????????????
	inputAutoSwitchCount int
	// ??????????????????
	outputCardName string
	outputPortName string
	// ???????????????????????????
	outputAutoSwitchCount int

	// nolint
	signals *struct {
		PortEnabledChanged struct {
			cardId   uint32
			portName string
			enabled  bool
		}
	}
}

func newAudio(service *dbusutil.Service) *Audio {
	a := &Audio{
		service:      service,
		meters:       make(map[string]*Meter),
		MaxUIVolume:  pulse.VolumeUIMax,
		enableSource: true,
	}

	a.settings = gio.NewSettings(gsSchemaAudio)
	a.settings.Reset(gsKeyInputVolume)
	a.settings.Reset(gsKeyOutputVolume)
	a.IncreaseVolume.Bind(a.settings, gsKeyVolumeIncrease)
	a.ReduceNoise.Bind(a.settings, gsKeyReduceNoise)
	a.headphoneUnplugAutoPause = a.settings.GetBoolean(gsKeyHeadphoneUnplugAutoPause)
	if a.IncreaseVolume.Get() {
		a.MaxUIVolume = increaseMaxVolume
	} else {
		a.MaxUIVolume = normalMaxVolume
	}
	gMaxUIVolume = a.MaxUIVolume
	a.listenGSettingReduceNoiseChanged()
	a.listenGSettingVolumeIncreaseChanged()
	a.sessionSigLoop = dbusutil.NewSignalLoop(service.Conn(), 10)
	a.syncConfig = dsync.NewConfig("audio", &syncConfig{a: a},
		a.sessionSigLoop, dbusPath, logger)
	a.sessionSigLoop.Start()
	return a
}

func startPulseaudio() error {
	var errBuf bytes.Buffer
	cmd := exec.Command(cmdSystemctl, "--user", "start", "pulseaudio")
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		logger.Warningf("failed to start pulseaudio via systemd: err: %v, stderr: %s",
			err, errBuf.Bytes())
	}
	errBuf.Reset()

	err = exec.Command(cmdPulseaudio, "--check").Run()
	if err == nil {
		return nil
	}

	cmd = exec.Command(cmdPulseaudio, "--start")
	cmd.Stderr = &errBuf
	err = cmd.Run()
	if err != nil {
		logger.Warningf("failed to start pulseaudio via `pulseaudio --start`: err: %v, stderr: %s",
			err, errBuf.Bytes())
		return xerrors.Errorf("cmd `pulseaudio --start` error: %w", err)
	}
	return nil
}

func getCtx() (ctx *pulse.Context, err error) {
	ctx = pulse.GetContextForced()
	if ctx == nil {
		err = errors.New("failed to get pulse context")
		return
	}
	return
}

func (a *Audio) refreshCards() {
	a.cards = newCardList(a.ctx.GetCardList())
	a.setPropCards(a.cards.string())
	a.setPropCardsWithoutUnavailable(a.cards.stringWithoutUnavailable())
}

// ??????????????????sink,?????????pulse???Sink
func (a *Audio) addSink(sinkInfo *pulse.Sink) {
	sink := newSink(sinkInfo, a)
	a.sinks[sinkInfo.Index] = sink
	sinkPath := sink.getPath()
	err := a.service.Export(sinkPath, sink)
	if err != nil {
		logger.Warning(err)
	}
	a.updatePropSinks()
}

// ??????????????????source,?????????pulse???Source
func (a *Audio) addSource(sourceInfo *pulse.Source) {
	source := newSource(sourceInfo, a)
	a.sources[sourceInfo.Index] = source
	sourcePath := source.getPath()
	err := a.service.Export(sourcePath, source)
	if err != nil {
		logger.Warning(err)
	}
	a.updatePropSources()
}

// ??????????????????sink-input,?????????pulse???SinkInput
func (a *Audio) addSinkInput(sinkInputInfo *pulse.SinkInput) {
	logger.Debug("new")
	sinkInput := newSinkInput(sinkInputInfo, a)
	logger.Debug("new done")
	a.sinkInputs[sinkInputInfo.Index] = sinkInput
	sinkInputPath := sinkInput.getPath()
	err := a.service.Export(sinkInputPath, sinkInput)
	if err != nil {
		logger.Warning(err)
	}
	logger.Debug("updatePropSinkInputs")
	a.updatePropSinkInputs()
	logger.Debug("updatePropSinkInputs done")
}

func (a *Audio) refreshSinks() {
	if a.sinks == nil {
		a.sinks = make(map[uint32]*Sink)
	}

	// ???????????????sinks
	sinkInfoMap := make(map[uint32]*pulse.Sink)
	sinkInfoList := a.ctx.GetSinkList()

	for _, sinkInfo := range sinkInfoList {
		sinkInfoMap[sinkInfo.Index] = sinkInfo
		sink, exist := a.sinks[sinkInfo.Index]
		if exist {
			// ???????????????
			logger.Debugf("update sink #%d", sinkInfo.Index)
			sink.update(sinkInfo)
		} else {
			// ??????????????????
			logger.Debugf("add sink #%d", sinkInfo.Index)
			a.addSink(sinkInfo)
		}
	}

	// ?????????????????????sink
	for key, sink := range a.sinks {
		_, exist := sinkInfoMap[key]
		if !exist {
			logger.Debugf("delete sink #%d", key)
			a.service.StopExport(sink)
			delete(a.sinks, key)
		}
	}
}

func (a *Audio) refreshSources() {
	if a.sources == nil {
		a.sources = make(map[uint32]*Source)
	}

	// ???????????????sources
	sourceInfoMap := make(map[uint32]*pulse.Source)
	sourceInfoList := a.ctx.GetSourceList()

	for _, sourceInfo := range sourceInfoList {
		sourceInfoMap[sourceInfo.Index] = sourceInfo
		source, exist := a.sources[sourceInfo.Index]
		if exist {
			// ???????????????
			logger.Debugf("update source #%d", sourceInfo.Index)
			source.update(sourceInfo)
		} else {
			// ??????????????????
			logger.Debugf("add source #%d", sourceInfo.Index)
			a.addSource(sourceInfo)
		}
	}

	// ?????????????????????source
	for key, source := range a.sources {
		_, exist := sourceInfoMap[key]
		if !exist {
			logger.Debugf("delete source #%d", key)
			a.service.StopExport(source)
			delete(a.sources, key)
		}
	}
}

func (a *Audio) refershSinkInputs() {
	if a.sinkInputs == nil {
		a.sinkInputs = make(map[uint32]*SinkInput)
	}

	// ???????????????sink-inputs
	sinkInputInfoMap := make(map[uint32]*pulse.SinkInput)
	sinkInputInfoList := a.ctx.GetSinkInputList()

	for _, sinkInputInfo := range sinkInputInfoList {
		sinkInputInfoMap[sinkInputInfo.Index] = sinkInputInfo
		sinkInput, exist := a.sinkInputs[sinkInputInfo.Index]
		if exist {
			logger.Debugf("update sink-input #%d", sinkInputInfo.Index)
			sinkInput.update(sinkInputInfo)
		} else {
			logger.Debugf("add sink-input #%d", sinkInputInfo.Index)
			a.addSinkInput(sinkInputInfo)
		}
	}

	// ?????????????????????sink-inputs
	for key, sinkInput := range a.sinkInputs {
		_, exist := sinkInputInfoMap[key]
		if !exist {
			logger.Debugf("delete sink-input #%d", key)
			a.service.StopExport(sinkInput)
			delete(a.sinkInputs, key)
		}
	}
}

func (a *Audio) refreshDefaultSinkSource() {
	defaultSink := a.ctx.GetDefaultSink()
	defaultSource := a.ctx.GetDefaultSource()

	if a.defaultSink != nil && a.defaultSink.Name != defaultSink {
		logger.Debugf("update default sink to %s", defaultSink)
		a.updateDefaultSink(defaultSink)
	} else {
		logger.Debugf("keep default as %s", defaultSink)
	}

	if a.defaultSource != nil && a.defaultSource.Name != defaultSource {
		logger.Debugf("update default source to %s", defaultSource)
		a.updateDefaultSource(defaultSource)
	} else {
		logger.Debugf("keep default as %s", defaultSource)
	}
}

func (a *Audio) refresh() {
	logger.Debug("refresh cards")
	a.refreshCards()
	logger.Debug("refresh sinks")
	a.refreshSinks()
	logger.Debug("refresh sources")
	a.refreshSources()
	logger.Debug("refresh sinkinputs")
	a.refershSinkInputs()
	logger.Debug("refresh default")
	a.refreshDefaultSinkSource()
	logger.Debug("refresh bluetooth mode opts")
	a.refreshBluetoothOpts()
	logger.Debug("refresh done")
}

func (a *Audio) init() error {
	if a.settings.GetBoolean(gsKeyDisableAutoMute) {
		err := disableAutoMuteMode()
		if err != nil {
			logger.Warning(err)
		}
	}
	a.initDefaultVolumes()
	ctx, err := getCtx()
	if err != nil {
		return xerrors.Errorf("failed to get context: %w", err)
	}

	a.defaultPaCfg = loadDefaultPaConfig(defaultPaFile)
	logger.Debugf("defaultPaConfig: %+v", a.defaultPaCfg)

	a.ctx = ctx

	// ??????????????????
	a.refresh()

	serverInfo, err := a.ctx.GetServer()
	if err == nil {
		a.mu.Lock()
		a.defaultSourceName = serverInfo.DefaultSourceName
		a.defaultSinkName = serverInfo.DefaultSinkName

		for _, sink := range a.sinks {
			if sink.Name == a.defaultSinkName {
				a.defaultSink = sink
				a.PropsMu.Lock()
				a.setPropDefaultSink(sink.getPath())
				a.PropsMu.Unlock()
			}
		}

		for _, source := range a.sources {
			if source.Name == a.defaultSourceName {
				a.defaultSource = source
				a.PropsMu.Lock()
				a.setPropDefaultSource(source.getPath())
				a.PropsMu.Unlock()
			}
		}
		a.mu.Unlock()
	} else {
		logger.Warning(err)
	}

	GetBluezAudioManager().Load()
	GetConfigKeeper().Load()

	logger.Debug("init cards")
	a.PropsMu.Lock()
	a.setPropCards(a.cards.string())
	a.setPropCardsWithoutUnavailable(a.cards.stringWithoutUnavailable())
	a.PropsMu.Unlock()

	a.eventChan = make(chan *pulse.Event, 100)
	a.stateChan = make(chan int, 10)
	a.quit = make(chan struct{})
	a.ctx.AddEventChan(a.eventChan)
	a.ctx.AddStateChan(a.stateChan)
	a.inputAutoSwitchCount = 0
	a.outputAutoSwitchCount = 0

	// priorities.Load(globalPrioritiesFilePath, a.cards) // TODO: ??????
	GetPriorityManager().Init(a.cards)
	GetPriorityManager().Print()

	go a.handleEvent()
	go a.handleStateChanged()
	logger.Debug("init done")

	firstRun := a.settings.GetBoolean(gsKeyFirstRun)
	if firstRun {
		logger.Info("first run, Will remove old audio config")
		removeConfig()
		a.settings.SetBoolean(gsKeyFirstRun, false)
	}

	a.resumeSinkConfig(a.defaultSink)
	a.resumeSourceConfig(a.defaultSource, isPhysicalDevice(a.defaultSourceName))
	a.autoSwitchPort()

	a.fixActivePortNotAvailable()
	a.moveSinkInputsToDefaultSink()

	err = a.setReduceNoise(a.ReduceNoise.Get())
	if err != nil {
		logger.Warning("set reduce noise fail:", err)
	}

	// ?????????????????????
	a.setPropBluetoothAudioModeOpts([]string{"a2dp", "headset"})

	return nil
}

func (a *Audio) destroyCtxRelated() {
	a.mu.Lock()
	a.ctx.RemoveEventChan(a.eventChan)
	a.ctx.RemoveStateChan(a.stateChan)
	close(a.quit)
	a.ctx = nil

	for _, sink := range a.sinks {
		err := a.service.StopExportByPath(sink.getPath())
		if err != nil {
			logger.Warningf("failed to stop export sink #%d: %v", sink.index, err)
		}
	}
	a.sinks = nil

	for _, source := range a.sources {
		err := a.service.StopExportByPath(source.getPath())
		if err != nil {
			logger.Warningf("failed to stop export source #%d: %v", source.index, err)
		}
	}
	a.sources = nil

	for _, sinkInput := range a.sinkInputs {
		err := a.service.StopExportByPath(sinkInput.getPath())
		if err != nil {
			logger.Warningf("failed to stop export sink input #%d: %v", sinkInput.index, err)
		}
	}
	a.sinkInputs = nil

	for _, meter := range a.meters {
		err := a.service.StopExport(meter)
		if err != nil {
			logger.Warning(err)
		}
	}
	a.mu.Unlock()
}

func (a *Audio) destroy() {
	a.settings.Unref()
	a.sessionSigLoop.Stop()
	a.syncConfig.Destroy()
	a.destroyCtxRelated()
}

func (a *Audio) initDefaultVolumes() {
	inVolumePer := float64(a.settings.GetInt(gsKeyInputVolume)) / 100.0
	outVolumePer := float64(a.settings.GetInt(gsKeyOutputVolume)) / 100.0
	headphoneOutVolumePer := float64(a.settings.GetInt(gsKeyHeadphoneOutputVolume)) / 100.0
	defaultInputVolume = inVolumePer
	defaultOutputVolume = outVolumePer
	defaultHeadphoneOutputVolume = headphoneOutVolumePer
}

func (a *Audio) findSinkByCardIndexPortName(cardId uint32, portName string) *pulse.Sink {
	for _, sink := range a.ctx.GetSinkList() {
		if isPortExists(portName, sink.Ports) && sink.Card == cardId {
			return sink
		}
	}
	return nil
}

func (a *Audio) findSourceByCardIndexPortName(cardId uint32, portName string) *pulse.Source {
	for _, source := range a.ctx.GetSourceList() {
		if isPortExists(portName, source.Ports) && source.Card == cardId {
			return source
		}
	}
	return nil
}

// set default sink and sink active port
func (a *Audio) setDefaultSinkWithPort(cardId uint32, portName string) error {
	_, portConfig := GetConfigKeeper().GetCardAndPortConfig(a.getCardNameById(cardId), portName)
	if !portConfig.Enabled {
		return fmt.Errorf("card #%d port %q is disabled", cardId, portName)
	}
	logger.Debugf("setDefaultSinkWithPort card #%d port %q", cardId, portName)
	sink := a.findSinkByCardIndexPortName(cardId, portName)
	if sink == nil {
		return fmt.Errorf("cannot find valid sink for card #%d and port %q",
			cardId, portName)
	}
	if sink.ActivePort.Name != portName {
		logger.Debugf("set sink #%d port %s", sink.Index, portName)
		a.ctx.SetSinkPortByIndex(sink.Index, portName)
	}
	if a.getDefaultSinkName() != sink.Name {
		logger.Debugf("set default sink #%d %s", sink.Index, sink.Name)
		a.ctx.SetDefaultSink(sink.Name)
	}
	return nil
}

func (a *Audio) getDefaultSinkActivePortName() string {
	defaultSink := a.getDefaultSink()
	if defaultSink == nil {
		return ""
	}

	defaultSink.PropsMu.RLock()
	name := defaultSink.ActivePort.Name
	defaultSink.PropsMu.RUnlock()
	return name
}

func (a *Audio) getDefaultSourceActivePortName() string {
	defaultSource := a.getDefaultSource()
	if defaultSource == nil {
		return ""
	}

	defaultSource.PropsMu.RLock()
	name := defaultSource.ActivePort.Name
	defaultSource.PropsMu.RUnlock()
	return name
}

// set default source and source active port
func (a *Audio) setDefaultSourceWithPort(cardId uint32, portName string) error {
	_, portConfig := GetConfigKeeper().GetCardAndPortConfig(a.getCardNameById(cardId), portName)
	if !portConfig.Enabled {
		return fmt.Errorf("card #%d port %q is disabled", cardId, portName)
	}
	logger.Debugf("setDefault card #%d port %q", cardId, portName)
	source := a.findSourceByCardIndexPortName(cardId, portName)
	if source == nil {
		return fmt.Errorf("cannot find valid source for card #%d and port %q",
			cardId, portName)
	}

	if source.ActivePort.Name != portName {
		logger.Debugf("set source #%d port %s", source.Index, portName)
		a.ctx.SetSourcePortByIndex(source.Index, portName)
	}

	if a.getDefaultSourceName() != source.Name {
		logger.Debugf("set default source #%d %s", source.Index, source.Name)
		a.ctx.SetDefaultSource(source.Name)
	}

	return nil
}

// SetPort activate the port for the special card.
// The available sinks and sources will also change with the profile changing.
func (a *Audio) SetPort(cardId uint32, portName string, direction int32) *dbus.Error {
	logger.Debugf("Audio.SetPort card idx: %d, port name: %q, direction: %d",
		cardId, portName, direction)

	if !a.isPortEnabled(cardId, portName, direction) {
		return dbusutil.ToError(fmt.Errorf("card idx: %d, port name: %q is disabled", cardId, portName))
	}

	err := a.setPort(cardId, portName, int(direction))
	if err != nil {
		return dbusutil.ToError(err)
	}

	card, err := a.cards.get(cardId)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}

	if int(direction) == pulse.DirectionSink {
		logger.Debugf("output port %s %s now is first priority", card.core.Name, portName)

		// TODO: ??????????????????????????????????????????????????????
		// sink := a.getDefaultSink()
		// if sink == nil {
		// 	return dbusutil.ToError(fmt.Errorf("can not get default sink"))
		// }
		// sink.setMute(false)

		// TODO: ??????
		// priorities.SetOutputPortFirst(card.core.Name, portName)
		// err = priorities.Save(globalPrioritiesFilePath)
		// priorities.Print()
		GetPriorityManager().SetFirstOutputPort(card.core.Name, portName)
	} else {
		logger.Debugf("input port %s %s now is first priority", card.core.Name, portName)

		// TODO: ??????????????????????????????????????????????????????
		// source := a.getDefaultSource()
		// if source == nil {
		// 	return dbusutil.ToError(fmt.Errorf("can not get default source"))
		// }
		// source.setMute(false)

		// TODO: ??????
		// priorities.SetInputPortFirst(card.core.Name, portName)
		// err = priorities.Save(globalPrioritiesFilePath)
		// priorities.Print()
		GetPriorityManager().SetFirstInputPort(card.core.Name, portName)
	}

	return dbusutil.ToError(err)
}

func (a *Audio) findSinks(cardId uint32, activePortName string) []*Sink {
	sinks := make([]*Sink, 0)
	for _, sink := range a.sinks {
		if sink.Card == cardId && sink.ActivePort.Name == activePortName {
			sinks = append(sinks, sink)
		}
	}

	return sinks
}

func (a *Audio) findSources(cardId uint32, activePortName string) []*Source {
	sources := make([]*Source, 0)
	for _, source := range a.sources {
		if source.Card == cardId && source.ActivePort.Name == activePortName {
			sources = append(sources, source)
		}
	}

	return sources
}

func (a *Audio) SetPortEnabled(cardId uint32, portName string, enabled bool) *dbus.Error {
	if enabled {
		logger.Debugf("enable port<%d,%s>", cardId, portName)
	} else {
		logger.Debugf("disable port<%d,%s>", cardId, portName)
	}
	GetConfigKeeper().SetEnabled(a.getCardNameById(cardId), portName, enabled)

	err := a.service.Emit(a, "PortEnabledChanged", cardId, portName, enabled)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}

	a.setPropCards(a.cards.string())
	a.setPropCardsWithoutUnavailable(a.cards.stringWithoutUnavailable())
	GetPriorityManager().SetPorts(a.cards)
	a.autoSwitchPort()

	sinks := a.findSinks(cardId, portName)
	for _, sink := range sinks {
		sink.setMute(!enabled || GetConfigKeeper().Mute.MuteOutput)
	}

	sources := a.findSources(cardId, portName)
	for _, source := range sources {
		source.setMute(!enabled || GetConfigKeeper().Mute.MuteInput)
	}

	return nil
}

func (a *Audio) IsPortEnabled(cardId uint32, portName string) (enabled bool, busErr *dbus.Error) {
	// ???????????????????????????????????????Cards???CardsWithoutUnavailable????????????????????????
	logger.Debugf("check is port<%d,%s> enabled", cardId, portName)
	_, portConfig := GetConfigKeeper().GetCardAndPortConfig(a.getCardNameById(cardId), portName)
	return portConfig.Enabled, nil
}

func (a *Audio) setPort(cardId uint32, portName string, direction int) error {
	if a.ReduceNoise.Get() {
		// ???????????????????????????????????????????????????????????????
		// ???????????????????????????????????????
		// ?????????????????????????????????????????????????????????????????????????????????
		// ?????????????????????????????????????????????
		a.setReduceNoise(false)
	}
	a.portLocker.Lock()
	defer a.portLocker.Unlock()
	var (
		oppositePort      string
		oppositeDirection int
	)
	switch direction {
	case pulse.DirectionSink:
		oppositePort = a.getDefaultSourceActivePortName()
		oppositeDirection = pulse.DirectionSource
	case pulse.DirectionSource:
		oppositePort = a.getDefaultSinkActivePortName()
		oppositeDirection = pulse.DirectionSink
	default:
		return fmt.Errorf("invalid port direction: %d", direction)
	}

	a.mu.Lock()
	card, _ := a.cards.get(cardId)
	a.mu.Unlock()
	if card == nil {
		return fmt.Errorf("not found card #%d", cardId)
	}

	var err error
	targetPortInfo, err := card.Ports.Get(portName, direction)
	if err != nil {
		return err
	}

	setDefaultPort := func() error {
		if int(direction) == pulse.DirectionSink {
			return a.setDefaultSinkWithPort(cardId, portName)
		}
		return a.setDefaultSourceWithPort(cardId, portName)
	}

	if targetPortInfo.Profiles.Exists(card.ActiveProfile.Name) {
		// no need to change profile
		return setDefaultPort()
	}

	// match the common profile contain sinkPort and sourcePort
	oppositePortInfo, _ := card.Ports.Get(oppositePort, oppositeDirection)
	commonProfiles := getCommonProfiles(targetPortInfo, oppositePortInfo)
	var targetProfile string
	if len(commonProfiles) != 0 {
		targetProfile = commonProfiles[0].Name
	} else {
		name, err := card.tryGetProfileByPort(portName)
		if err != nil {
			return err
		}
		targetProfile = name
	}
	// workaround for bluetooth, set profile to 'a2dp_sink' when port direction is output
	if direction == pulse.DirectionSink && targetPortInfo.Profiles.Exists("a2dp_sink") {
		targetProfile = "a2dp_sink"
	}
	card.core.SetProfile(targetProfile)
	logger.Debug("set profile", targetProfile)
	return setDefaultPort()
}

func (a *Audio) resetSinksVolume() {
	logger.Debug("reset sink volume", defaultOutputVolume)
	for _, s := range a.ctx.GetSinkList() {
		a.ctx.SetSinkMuteByIndex(s.Index, false)
		curPort := s.ActivePort.Name
		portList := s.Ports
		sidx := s.Index
		for _, port := range portList {
			a.ctx.SetSinkPortByIndex(sidx, port.Name)
			// wait port active
			time.Sleep(time.Millisecond * 100)
			s, _ = a.ctx.GetSink(sidx)
			pname := strings.ToLower(port.Name)
			var cv pulse.CVolume
			if strings.Contains(pname, "headphone") || strings.Contains(pname, "headset") {
				cv = s.Volume.SetAvg(defaultHeadphoneOutputVolume).SetBalance(s.ChannelMap,
					0).SetFade(s.ChannelMap, 0)
			} else {
				cv = s.Volume.SetAvg(defaultOutputVolume).SetBalance(s.ChannelMap,
					0).SetFade(s.ChannelMap, 0)
			}
			a.ctx.SetSinkVolumeByIndex(sidx, cv)
			time.Sleep(time.Millisecond * 100)
		}
		a.ctx.SetSinkPortByIndex(sidx, curPort)
	}
}

func (a *Audio) resetSourceVolume() {
	logger.Debug("reset source volume", defaultInputVolume)
	for _, s := range a.ctx.GetSourceList() {
		a.ctx.SetSourceMuteByIndex(s.Index, false)
		cv := s.Volume.SetAvg(defaultInputVolume).SetBalance(s.ChannelMap,
			0).SetFade(s.ChannelMap, 0)
		a.ctx.SetSourceVolumeByIndex(s.Index, cv)
	}
}

func (a *Audio) Reset() *dbus.Error {
	a.resetSinksVolume()
	a.resetSourceVolume()
	gsSoundEffect := gio.NewSettings(gsSchemaSoundEffect)
	gsSoundEffect.Reset(gsKeyEnabled)
	gsSoundEffect.Unref()
	return nil
}

func (a *Audio) moveSinkInputsToSink(sinkId uint32) {
	a.mu.Lock()
	if len(a.sinkInputs) == 0 {
		a.mu.Unlock()
		return
	}
	var list []uint32
	for _, sinkInput := range a.sinkInputs {
		if sinkInput.getPropSinkIndex() == sinkId {
			continue
		}

		list = append(list, sinkInput.index)
	}
	a.mu.Unlock()
	if len(list) == 0 {
		return
	}
	logger.Debugf("move sink inputs %v to sink #%d", list, sinkId)
	a.ctx.MoveSinkInputsByIndex(list, sinkId)
}

func isPortExists(name string, ports []pulse.PortInfo) bool {
	for _, port := range ports {
		if port.Name == name {
			return true
		}
	}
	return false
}

func (*Audio) GetInterfaceName() string {
	return dbusInterface
}

func (a *Audio) resumeSinkConfig(s *Sink) {
	if s == nil {
		logger.Warning("nil sink")
		return
	}

	logger.Debugf("resume sink %s %s", a.getCardNameById(s.Card), s.ActivePort.Name)
	_, portConfig := GetConfigKeeper().GetCardAndPortConfig(a.getCardNameById(s.Card), s.ActivePort.Name)

	a.IncreaseVolume.Set(portConfig.IncreaseVolume)
	if portConfig.IncreaseVolume {
		a.MaxUIVolume = increaseMaxVolume
	} else {
		a.MaxUIVolume = normalMaxVolume
	}

	err := s.setVBF(portConfig.Volume, portConfig.Balance, 0.0)
	if err != nil {
		logger.Warning(err)
	}

	s.setMute(GetConfigKeeper().Mute.MuteOutput)

	if !portConfig.Enabled {
		// ?????????????????????????????????????????????????????????????????????
		s.setMute(true)
	}
}

func (a *Audio) resumeSourceConfig(s *Source, isPhyDev bool) {
	if s == nil {
		logger.Warning("nil source")
		return
	}

	logger.Debugf("resume source %s %s", a.getCardNameById(s.Card), s.ActivePort.Name)
	_, portConfig := GetConfigKeeper().GetCardAndPortConfig(a.getCardNameById(s.Card), s.ActivePort.Name)

	err := s.setVBF(portConfig.Volume, portConfig.Balance, 0.0)
	if err != nil {
		logger.Warning(err)
	}

	s.setMute(GetConfigKeeper().Mute.MuteInput)

	// ??????????????????????????????????????????
	if isPhyDev {
		a.ReduceNoise.Set(portConfig.ReduceNoise)
		logger.Debugf("physical source, set reduce noise %v", portConfig.ReduceNoise)
	} else if !portConfig.ReduceNoise {
		a.ReduceNoise.Set(portConfig.ReduceNoise)
		logger.Debugf("reduce noise source, set reduce noise %v", portConfig.ReduceNoise)
	}

	if !portConfig.Enabled {
		// ?????????????????????????????????????????????????????????????????????
		s.setMute(true)
	}
}

func (a *Audio) refreshBluetoothOpts() {
	if a.defaultSink == nil {
		return
	}
	card, err := a.cards.get(a.defaultSink.Card)
	if err != nil {
		logger.Warning(err)
		return
	}

	a.setPropBluetoothAudioModeOpts(card.BluezModeOpts())
	a.setPropBluetoothAudioMode(card.BluezMode())
}

func (a *Audio) updateDefaultSink(sinkName string) {
	sinkInfo := a.getSinkInfoByName(sinkName)

	if sinkInfo == nil {
		logger.Warning("failed to get sinkInfo for name:", sinkName)
		a.setPropDefaultSink("/")
		return
	}
	logger.Debugf("updateDefaultSink #%d %s", sinkInfo.Index, sinkName)
	a.moveSinkInputsToSink(sinkInfo.Index)
	if !isPhysicalDevice(sinkName) {
		sinkInfo = a.getSinkInfoByName(sinkInfo.PropList["device.master_device"])
		if sinkInfo == nil {
			logger.Warning("failed to get virtual device sinkInfo for name:", sinkName)
			return
		}
	}
	a.mu.Lock()
	sink, ok := a.sinks[sinkInfo.Index]
	a.mu.Unlock()
	if !ok {
		// a.sinks ???????????? sink ?????????????????? sink ??????????????????????????? pulseaudio ?????? sink ??????
		logger.Warningf("update sink %d", sinkInfo.Index)
		sink = a.updateSinks(sinkInfo.Index)
		logger.Debugf("updated sink %d", sinkInfo.Index)
		if sink == nil {
			logger.Warningf("not found sink #%d", sinkInfo.Index)
			a.setPropDefaultSink("/")
			return
		}
	}

	a.defaultSink = sink
	defaultSinkPath := sink.getPath()

	a.PropsMu.Lock()
	a.setPropDefaultSink(defaultSinkPath)
	a.PropsMu.Unlock()

	logger.Debug("set prop default sink:", defaultSinkPath)
	a.resumeSinkConfig(sink)
}

func (a *Audio) updateSources(index uint32) (source *Source) {
	sourceInfoList := a.ctx.GetSourceList()
	for _, sourceInfo := range sourceInfoList {
		//??????????????????????????????????????????monitor
		if strings.HasSuffix(sourceInfo.Name, ".monitor") {
			logger.Debugf("skip %s source update", sourceInfo.Name)
			continue
		}
		// ?????? pulseaudio ??? source ??????????????????????????????????????? source ??????
		if sourceInfo.Index == index {
			logger.Debug("get same source index:", index)
			source := newSource(sourceInfo, a)
			a.sources[index] = source
			sourcePath := source.getPath()
			err := a.service.Export(sourcePath, source)
			if err != nil {
				logger.Warning(err)
			}
			return source
		}
	}
	return nil
}

func (a *Audio) updateSinks(index uint32) (sink *Sink) {
	sinkInfoList := a.ctx.GetSinkList()
	for _, sinkInfo := range sinkInfoList {
		// ??????pulseaudio???sink???????????????????????????????????????sink??????
		if sinkInfo.Index == index {
			logger.Debug("get same sink index:", index)
			sink := newSink(sinkInfo, a)
			logger.Debug("done")
			a.mu.Lock()
			a.sinks[index] = sink
			a.mu.Unlock()
			sinkPath := sink.getPath()
			err := a.service.Export(sinkPath, sink)
			if err != nil {
				logger.Warning(err)
			}
			return sink
		}
	}
	return nil
}

func (a *Audio) updateDefaultSource(sourceName string) {
	sourceInfo := a.getSourceInfoByName(sourceName)
	if sourceInfo == nil {
		logger.Warning("failed to get sourceInfo for name:", sourceName)
		a.setPropDefaultSource("/")
		return
	}
	logger.Debugf("updateDefaultSource #%d %s", sourceInfo.Index, sourceName)
	a.mu.Lock()

	source, ok := a.sources[sourceInfo.Index]
	if !ok {
		// a.sources ???????????? source ?????????????????? source ??????????????????????????? pulseaudio ?????? source ??????
		source = a.updateSources(sourceInfo.Index)
		if source == nil {
			a.mu.Unlock()
			logger.Warningf("not found source #%d", sourceInfo.Index)
			a.setPropDefaultSource("/")
			return
		}
	}
	a.defaultSource = source
	defaultSourcePath := source.getPath()
	a.mu.Unlock()

	a.PropsMu.Lock()
	a.setPropDefaultSource(defaultSourcePath)
	a.PropsMu.Unlock()

	logger.Debug("set prop default source:", defaultSourcePath)
	a.resumeSourceConfig(source, isPhysicalDevice(sourceName))
}

func (a *Audio) context() *pulse.Context {
	a.mu.Lock()
	c := a.ctx
	a.mu.Unlock()
	return c
}

func (a *Audio) moveSinkInputsToDefaultSink() {
	a.mu.Lock()
	if a.defaultSink == nil {
		a.mu.Unlock()
		return
	}
	defaultSinkIndex := a.defaultSink.index
	a.mu.Unlock()
	a.moveSinkInputsToSink(defaultSinkIndex)
}

func (a *Audio) getDefaultSource() *Source {
	a.mu.Lock()
	v := a.defaultSource
	a.mu.Unlock()
	return v
}

func (a *Audio) getDefaultSourceName() string {
	source := a.getDefaultSource()
	if source == nil {
		return ""
	}

	source.PropsMu.RLock()
	v := source.Name
	source.PropsMu.RUnlock()
	return v
}

func (a *Audio) getDefaultSink() *Sink {
	a.mu.Lock()
	v := a.defaultSink
	a.mu.Unlock()
	return v
}

func (a *Audio) getDefaultSinkName() string {
	sink := a.getDefaultSink()
	if sink == nil {
		return ""
	}

	sink.PropsMu.RLock()
	v := sink.Name
	sink.PropsMu.RUnlock()
	return v
}

func (a *Audio) getSinkInfoByName(sinkName string) *pulse.Sink {
	for _, sinkInfo := range a.ctx.GetSinkList() {
		if sinkInfo.Name == sinkName {
			return sinkInfo
		}
	}
	return nil
}

func (a *Audio) getSourceInfoByName(sourceName string) *pulse.Source {
	for _, sourceInfo := range a.ctx.GetSourceList() {
		if sourceInfo.Name == sourceName {
			return sourceInfo
		}
	}
	return nil
}
func getBestPort(ports []pulse.PortInfo) pulse.PortInfo {
	var portUnknown pulse.PortInfo
	var portYes pulse.PortInfo
	for _, port := range ports {
		if port.Available == pulse.AvailableTypeYes {
			if port.Priority > portYes.Priority || portYes.Name == "" {
				portYes = port
			}
		} else if port.Available == pulse.AvailableTypeUnknow {
			if port.Priority > portUnknown.Priority || portUnknown.Name == "" {
				portUnknown = port
			}
		}
	}

	if portYes.Name != "" {
		return portYes
	}
	return portUnknown
}

func (a *Audio) fixActivePortNotAvailable() {
	sinkInfoList := a.ctx.GetSinkList()
	for _, sinkInfo := range sinkInfoList {
		activePort := sinkInfo.ActivePort

		if activePort.Available == pulse.AvailableTypeNo {
			newPort := getBestPort(sinkInfo.Ports)
			if newPort.Name != activePort.Name && newPort.Name != "" {
				logger.Info("auto switch to port", newPort.Name)
				a.ctx.SetSinkPortByIndex(sinkInfo.Index, newPort.Name)
				a.saveConfig()
			}
		}
	}
}

func (a *Audio) NoRestartPulseAudio() *dbus.Error {
	a.noRestartPulseAudio = true
	return nil
}

//?????????????????????????????????a2dp???,?????????????????????,?????????????????????,???????????????
func (a *Audio) disableBluezSourceIfProfileIsA2dp() {
	a.mu.Lock()
	source, ok := a.sources[a.sourceIdx]
	if !ok {
		a.mu.Unlock()
		return
	}
	delete(a.sources, a.sourceIdx)
	a.mu.Unlock()
	a.updatePropSources()

	err := a.service.StopExport(source)
	if err != nil {
		logger.Warning(err)
		return
	}
}

func (a *Audio) isPortEnabled(cardId uint32, portName string, direction int32) bool {
	// ??????cardId ?????? portName????????????
	a.mu.Lock()
	card, _ := a.cards.get(cardId)
	a.mu.Unlock()
	if card == nil {
		logger.Warningf("not found card #%d", cardId)
		return false
	}

	var err error
	_, err = card.Ports.Get(portName, int(direction))
	if err != nil {
		logger.Warningf("get port %s info failed: %v", portName, err)
		return false
	}

	_, portConfig := GetConfigKeeper().GetCardAndPortConfig(a.getCardNameById(cardId), portName)
	return portConfig.Enabled
}

// ??????????????????
func (a *Audio) SetBluetoothAudioMode(mode string) *dbus.Error {
	card, err := a.cards.get(a.defaultSink.Card)
	if err != nil {
		logger.Warning(err)
	}

	if !isBluezAudio(card.core.Name) {
		return dbusutil.ToError(fmt.Errorf("current card %s is not bluetooth audio device", card.core.Name))
	}

	for _, profile := range card.Profiles {
		/* ?????????????????????profile.Available???0?????????????????????0???????????? */
		logger.Debugf("check profile %s contains %s is %v && available != no is %v",
			profile.Name, mode, strings.Contains(strings.ToLower(profile.Name), mode),
			profile.Available != 0)
		if strings.Contains(strings.ToLower(profile.Name), mode) &&
			profile.Available != 0 {

			GetBluezAudioManager().SetMode(card.core.Name, mode)
			logger.Debugf("set profile %s", profile.Name)
			card.core.SetProfile(profile.Name)

			// ???????????????????????????headset???
			if mode == bluezModeHeadset {
				a.inputAutoSwitchCount = 0
				GetPriorityManager().Input.SetTheFirstType(PortTypeBluetooth)
			}
			return nil
		}
	}

	return dbusutil.ToError(fmt.Errorf("%s cannot support %s mode", card.core.Name, mode))
}
