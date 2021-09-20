package systeminfo

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

var xmlData string = `<?xml version="1.0" standalone="yes" ?>
<!-- generated by lshw- -->
<!-- GCC 8.3.0 -->
<!-- Linux 4.19.0-6-amd64 #1 SMP Uos 4.19.67-11eagle (2020-03-21) x86_64 -->
<!-- GNU libc 2 (glibc 2.28) -->
<list>
  <node id="memory" claimed="true" class="memory" handle="DMI:0034">
   <description>System Memory</description>
   <physid>34</physid>
   <slot>System board or motherboard</slot>
   <size units="bytes">3221225472</size>
   <hints>
    <hint name="icon" value="memory" />
   </hints>
    <node id="bank:0" claimed="true" class="memory" handle="DMI:0035">
     <description>SODIMM DDR3 Synchronous 1333 MHz (0.8 ns)</description>
     <product>EBJ40UG8BBU0-GN-F</product>
     <vendor>Elpida</vendor>
     <physid>0</physid>
     <serial>[REMOVED]</serial>
     <slot>ChannelA-DIMM0</slot>
     <size units="bytes">4294967296</size>
     <width units="bits">64</width>
     <clock units="Hz">1333000000</clock>
     <hints>
      <hint name="icon" value="memory" />
     </hints>
    </node>
    <node id="bank:1" claimed="true" class="memory" handle="DMI:0036">
     <description>SODIMM DDR3 Synchronous 1333 MHz (0.8 ns)</description>
     <product>M471B5773DH0-CK0</product>
     <vendor>Samsung</vendor>
     <physid>1</physid>
     <serial>[REMOVED]</serial>
     <slot>ChannelB-DIMM0</slot>
     <size units="bytes">2147483648</size>
     <width units="bits">64</width>
     <clock units="Hz">1333000000</clock>
     <hints>
      <hint name="icon" value="memory" />
     </hints>
    </node>
  </node>
</list>`

var dmidecodeData string = `# dmidecode 3.2
Getting SMBIOS data from sysfs.
SMBIOS 2.7 present.

Handle 0x0033, DMI type 4, 42 bytes
Processor Information
        Socket Designation: SOCKET 0
        Type: Central Processor
        Family: Core i5
        Manufacturer: Intel
        ID: C3 06 03 00 FF FB EB BF
        Signature: Type 0, Family 6, Model 60, Stepping 3
        Flags:
                FPU (Floating-point unit on-chip)
                VME (Virtual mode extension)
                DE (Debugging extension)
                PSE (Page size extension)
                TSC (Time stamp counter)
                MSR (Model specific registers)
                PAE (Physical address extension)
                MCE (Machine check exception)
                CX8 (CMPXCHG8 instruction supported)
                APIC (On-chip APIC hardware supported)
                SEP (Fast system call)
                MTRR (Memory type range registers)
                PGE (Page global enable)
                MCA (Machine check architecture)
                CMOV (Conditional move instruction supported)
                PAT (Page attribute table)
                PSE-36 (36-bit page size extension)
                CLFSH (CLFLUSH instruction supported)
                DS (Debug store)
                ACPI (ACPI supported)
                MMX (MMX technology supported)
                FXSR (FXSAVE and FXSTOR instructions supported)
                SSE (Streaming SIMD extensions)
                SSE2 (Streaming SIMD extensions 2)
                SS (Self-snoop)
                HTT (Multi-threading)
                TM (Thermal monitor supported)
                PBE (Pending break enabled)
        Version: Intel(R) Core(TM) i5-4570 CPU @ 3.20GHz
        Voltage: 1.2 V
        External Clock: 100 MHz
        Max Speed: 3800 MHz
        Current Speed: 3200 MHz
        Status: Populated, Enabled
        Upgrade: Socket BGA1155
        L1 Cache Handle: 0x0034
        L2 Cache Handle: 0x0035
        L3 Cache Handle: 0x0036
        Serial Number: Not Specified
        Asset Tag: Fill By OEM
        Part Number: Fill By OEM
        Core Count: 4
        Core Enabled: 4
        Thread Count: 4
        Characteristics:
                64-bit capable
`

func Test_parseXml(t *testing.T) {
	Convey("parseXml", t, func(c C) {
		value, err := parseXml([]byte(xmlData))
		v := value.Size
		t.Log(v)
		c.So(v, ShouldEqual, uint64(3221225472))
		c.So(err, ShouldBeNil)
	})
}

func Test_parseCurrentSpeed(t *testing.T) {
	Convey("parseCurrentSpeed", t, func(c C) {
		value, err := parseCurrentSpeed(dmidecodeData, 64)
		v := value
		t.Log(v)
		c.So(v, ShouldEqual, 3200)
		c.So(err, ShouldBeNil)
	})
}

func Test_filterUnNumber(t *testing.T) {
	Convey("filterUnNumber", t, func(c C) {
		str := "Current Speed: 3200 MHz"
		value := filterUnNumber(str)
		t.Log(value)
		c.So(value, ShouldEqual, "3200")
	})
}

func Test_formatFileSize(t *testing.T) {
	var infos = []struct {
		sizeInt uint64
		sizeString string
	}{
		{(1 << 10) - 1, "1023.00B"},
		{(1 << 20) - (1 << 10), "1023.00KB"},
		{(1 << 30) - (1 << 20), "1023.00MB"},
		{(1 << 40) - (1 << 30), "1023.00GB"},
		{(1 << 50) - (1 << 40), "1023.00TB"},
		{(1 << 60) - (1 << 50), "1023.00EB"},
	}

	Convey("formatFileSize", t, func(c C) {
		for _, info := range infos {
			value := formatFileSize(info.sizeInt)
			t.Log(value)
			c.So(value, ShouldEqual, info.sizeString)
		}
	})
}
