package appearance

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	backgroundDBusPath = dbusPath + "/Background"
)

type syncConfig struct {
	m *Manager
}

func (sc *syncConfig) Get() (interface{}, error) {
	var v syncData
	v.Version = "1.0"
	v.FontSize = sc.m.FontSize.Get()
	v.GTK = sc.m.GtkTheme.Get()
	v.Icon = sc.m.IconTheme.Get()
	v.Cursor = sc.m.CursorTheme.Get()
	v.FontStandard = sc.m.StandardFont.Get()
	v.FontMonospace = sc.m.MonospaceFont.Get()
	return &v, nil
}

func (sc *syncConfig) Set(data []byte) error {
	var v syncData
	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}

	m := sc.m
	if m.FontSize.Get() != v.FontSize {
		err = m.doSetFontSize(v.FontSize)
		if err != nil {
			logger.Warning("failed to set font size:", err)
		} else {
			m.FontSize.Set(v.FontSize)
		}
	}

	if m.GtkTheme.Get() != v.GTK {
		err = m.doSetGtkTheme(v.GTK)
		if err != nil {
			logger.Warning("failed to set gtk theme:", err)
		} else {
			m.GtkTheme.Set(v.GTK)
		}
	}

	if m.IconTheme.Get() != v.Icon {
		err = m.doSetIconTheme(v.Icon)
		if err != nil {
			logger.Warning("failed to set icon theme:", err)
		} else {
			m.IconTheme.Set(v.Icon)
		}
	}

	if m.CursorTheme.Get() != v.Cursor {
		err = m.doSetCursorTheme(v.Cursor)
		if err != nil {
			logger.Warning("failed to set cursor theme:", err)
		} else {
			m.CursorTheme.Set(v.Cursor)
		}
	}

	if m.StandardFont.Get() != v.FontStandard {
		err = m.doSetStandardFont(v.FontStandard)
		if err != nil {
			logger.Warning("failed to set standard font:", err)
		} else {
			m.StandardFont.Set(v.FontStandard)
		}
	}

	if m.MonospaceFont.Get() != v.FontMonospace {
		err = m.doSetMonospaceFont(v.FontMonospace)
		if err != nil {
			logger.Warning("failed to set monospace font:", err)
		} else {
			m.MonospaceFont.Set(v.FontMonospace)
		}
	}

	return nil
}

// version: 1.0
type syncData struct {
	Version       string  `json:"version"`
	FontSize      float64 `json:"font_size"`
	GTK           string  `json:"gtk"`
	Icon          string  `json:"icon"`
	Cursor        string  `json:"cursor"`
	FontStandard  string  `json:"font_standard"`
	FontMonospace string  `json:"font_monospace"`
}

type backgroundSyncConfig struct {
	m *Manager
}

func (sc *backgroundSyncConfig) Get() (interface{}, error) {
	var v backgroundSyncData
	v.Version = "2.0"
	v.GreeterBackground = sc.m.greeterBg
	slideShow := sc.m.WallpaperSlideShow.Get()
	var cfgSlideshow mapMonitorWorkspaceWSPolicy
	var err error
	if slideShow != "" { //??????slideshow????????????????????????????????????
		cfgSlideshow, err = doUnmarshalWallpaperSlideshow(slideShow) // slideShow?????????map ???????????? "HDMI-0&&1":"600" ?????????????????????&&??????????????????????????????????????????
		if err != nil {
			logger.Warning(err)
			return nil, err
		}
	}

	uploadSlideShow := make(mapMonitorWorkspaceWSPolicy)
	for k, value := range cfgSlideshow { // ????????????????????????(??????"HDMI-0"???"VGA-0")?????????????????????????????????(Primary???Subsidiary0/Subsidiary1???)
		keySlice := strings.Split(k, "&&")
		if len(keySlice) < 2 {
			continue
		}
		index, err := strconv.Atoi(keySlice[1])
		if err != nil {
			logger.Warning(err)
			return nil, err
		}
		if int32(index) < 1 {
			return nil, errors.New("invalid workspace index")
		}
		monitorName := sc.m.monitorMap[keySlice[0]]
		key := genMonitorKeyString(monitorName, keySlice[1])
		uploadSlideShow[key] = value
	}
	wallpaperURIsString := sc.m.WallpaperURIs.Get()
	var cfgWallpaperURIs mapMonitorWorkspaceWallpaperURIs
	if wallpaperURIsString != "" {
		cfgWallpaperURIs, err = doUnmarshalMonitorWorkspaceWallpaperURIs(wallpaperURIsString)
		if err != nil {
			logger.Warning(err)
			return nil, err
		}
	}

	uploadWallpaperURIs := make(mapMonitorWorkspaceWallpaperURIs)
	for key, value := range cfgWallpaperURIs { // ??????????????????????????????????????????,????????????????????????????????????????????????(???????????????????????????????????????),????????????????????????
		if uploadSlideShow[key] == "" {
			uploadWallpaperURIs[key] = value
		}
	}
	v.WallpaperURIs = uploadWallpaperURIs
	v.SlideShowConfig = uploadSlideShow

	return &v, nil
}

func (sc *backgroundSyncConfig) Set(data []byte) error {
	var v backgroundSyncData
	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}
	m := sc.m
	if m.greeterBg != v.GreeterBackground {
		err = m.doSetGreeterBackground(v.GreeterBackground)
		if err != nil {
			logger.Warning(err)
		}
	}

	reverseMonitorMap := m.reverseMonitorMap() // ??????????????????????????????????????????map ?????????{"Primary": "HDMI-0"}
	if v.Version == "1.0" {                    // ??????v1.0???????????????
		primaryMonitor := reverseMonitorMap["Primary"]
		// ???????????????????????????
		wallpaperSlideShow := m.WallpaperSlideShow.Get()
		var slideshowConfig mapMonitorWorkspaceWSPolicy
		if wallpaperSlideShow != "" {
			slideshowConfig, err = doUnmarshalWallpaperSlideshow(wallpaperSlideShow)
			if err != nil {
				logger.Warning(err)
				return err
			}
		} else {
			slideshowConfig = make(mapMonitorWorkspaceWSPolicy)
		}

		workspaceCount, _ := m.wm.WorkspaceCount(0)
		for i := 1; i <= int(workspaceCount); i++ {
			key := genMonitorKeyString(primaryMonitor, i)
			slideshowConfig[key] = v.SlideShow
		}
		err := m.setPropertyWallpaperSlideShow(slideshowConfig)
		if err != nil {
			logger.Warning(err)
			return err
		}
		if v.SlideShow != "" {
			return nil
		}
		// ??????????????????
		wallpaperURIs := m.WallpaperURIs.Get()
		var monitorWorkspaceWallpaperURIs mapMonitorWorkspaceWallpaperURIs
		if wallpaperURIs != "" {
			monitorWorkspaceWallpaperURIs, err = doUnmarshalMonitorWorkspaceWallpaperURIs(wallpaperURIs)
			if err != nil {
				logger.Warning(err)
				return err
			}
		} else {
			monitorWorkspaceWallpaperURIs = make(mapMonitorWorkspaceWallpaperURIs)
		}

		for i, uri := range v.BackgroundURIs {
			err := m.wm.SetWorkspaceBackgroundForMonitor(0, int32(i+1), primaryMonitor, uri)
			if err != nil {
				err = fmt.Errorf("failed to set background:%v to workspace%v : %v", uri, i+1, err)
				logger.Warning(err)
				return err
			}
			key := genMonitorKeyString("Primary", i+1)
			monitorWorkspaceWallpaperURIs[key] = uri
		}
		err = m.setPropertyWallpaperURIs(monitorWorkspaceWallpaperURIs)
		if err != nil {
			logger.Warning(err)
			return err
		}
		return nil
	}

	slideShow := make(mapMonitorWorkspaceWSPolicy)
	for k, value := range v.SlideShowConfig {
		keySlice := strings.Split(k, "&&")
		if len(keySlice) < 2 {
			continue
		}
		monitorName := reverseMonitorMap[keySlice[0]] //????????????????????????(Primary???Subsidiary0/Subsidiary1???)??????????????????????????????(??????"HDMI-0"???"VGA-0")
		index, err := strconv.Atoi(keySlice[1])
		if err != nil {
			logger.Warning(err)
			return err
		}
		if int32(index) < 1 {
			return errors.New("invalid workspace index")
		}
		key := genMonitorKeyString(monitorName, keySlice[1])
		slideShow[key] = value
	}

	err = m.setPropertyWallpaperSlideShow(slideShow)
	if err != nil {
		return err
	}
	err = m.setPropertyWallpaperURIs(v.WallpaperURIs)
	if err != nil {
		return err
	}

	workspaceCount, _ := m.wm.WorkspaceCount(0) // ?????????????????????
	for key, value := range v.WallpaperURIs {
		keySlice := strings.Split(key, "&&")
		if len(keySlice) < 2 {
			continue
		}
		monitorName := reverseMonitorMap[keySlice[0]] // ????????????????????????(Primary???Subsidiary0/Subsidiary1???)??????????????????????????????(??????"HDMI-0"???"VGA-0")
		index, err := strconv.Atoi(keySlice[1])
		if err != nil {
			logger.Warning(err)
			return err
		}
		if monitorName == "" {
			continue
		}
		if int32(index) < 1 { // index???1??????????????????????????????????????????1??????????????????
			return errors.New("invalid workspace index")
		}
		if int32(index) > workspaceCount {
			continue
		}
		err = m.wm.SetWorkspaceBackgroundForMonitor(0, int32(index), monitorName, value)
		if err != nil {
			logger.Warning("failed to set WorkspaceBackgroundForMonitor:", err)
		}
	}
	return nil
}

// version: 2.0
type backgroundSyncData struct {
	Version           string                           `json:"version"`
	GreeterBackground string                           `json:"greeter_background"`
	SlideShowConfig   mapMonitorWorkspaceWSPolicy      `json:"slide_show_config"` // v2.0??????
	WallpaperURIs     mapMonitorWorkspaceWallpaperURIs `json:"wallpaper_uris"`    // v2.0??????
	BackgroundURIs    []string                         `json:"background_uris"`   // ??????v1.0?????????
	SlideShow         string                           `json:"slide_show"`        // ??????v1.0?????????
}
