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

package background

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestScanner(t *testing.T) {
	Convey("getBgFilesInDir", t, func(c C) {
		c.So(getBgFilesInDir("testdata/Theme1/wallpapers"), ShouldResemble,
			[]string{
				"testdata/Theme1/wallpapers/desktop.jpg",
			})
		c.So(getBgFilesInDir("testdata/Theme2/wallpapers"), ShouldBeNil)
	})
}

func TestFileInDirs(t *testing.T) {
	Convey("Test file whether in dirs", t, func(c C) {
		var dirs = []string{
			"/tmp/backgrounds",
			"/tmp/wallpapers",
		}

		c.So(isFileInDirs("/tmp/backgrounds/1.jpg", dirs),
			ShouldEqual, true)
		c.So(isFileInDirs("/tmp/wallpapers/1.jpg", dirs),
			ShouldEqual, true)
		c.So(isFileInDirs("/tmp/background/1.jpg", dirs),
			ShouldEqual, false)
	})
}

func TestGetBgFiles(t *testing.T) {
	files := getSysBgFiles()
	t.Log(files)
}
