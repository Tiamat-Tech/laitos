package procexp

import (
	"encoding/binary"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

const (
	// SizeOfUint is the size of an unsigned integer - either 32 or 64.
	SizeOfUint uint = 32 << (^uint(0) >> 63)
)

var (
	regexStatusKeyValue  = regexp.MustCompile(`^(\w+)\s*:\s*(.*)$`)
	regexSchedstatFields = regexp.MustCompile(`^(\d+)\s+(\d+)\s+(\d+).*$`)
	// PID, executable base name (up to 16 characters long), state, and 35 more fields.
	// See https://man7.org/linux/man-pages/man5/procfs.5.html for the complete list of fields.
	regexStatFields = regexp.MustCompile(`^(\d+)\s+\((.*)\)\s+(\S+)\s+` + strings.Repeat(`(\S+)\s+`, 35) + `.*$`)

	// sysconfClockTick is the cached value of the number of times kernel timer interrupts each second,
	// the value is going to be calculated by function getClockTicksPerSecondOnce.
	sysconfClockTick           int = 0
	getClockTicksPerSecondOnce     = new(sync.Once)
)

type ProcessStatus struct {
	// General
	Name                  string // status
	Umask                 string
	State                 string
	ThreadGroupID         int
	ProcessID             int
	ParentPID             int
	ProcessGroupID        int
	StartedAtUptimeSec    int
	ProcessPrivilege      ProcessPrivilege
	ProcessMemUsage       ProcessMemUsage
	ProcessCPUUsage       ProcessCPUUsage
	ProcessSchedulerStats ProcessSchedulerStats
}

type ProcessPrivilege struct {
	// status
	RealUID      int
	EffectiveUID int
	RealGID      int
	EffectiveGID int
}

type ProcessMemUsage struct {
	// stat
	VirtualMemSizeBytes     int
	ResidentSetMemSizeBytes int
}

type ProcessCPUUsage struct {
	// stat
	NumUserModeSecInclChildren int
	NumSysModeSecInclChildren  int
}

type ProcessSchedulerStats struct {
	// status
	NumVoluntaryCtxSwitches    int
	NumNonVoluntaryCtxSwitches int
	// schedstat
	NumRunSec  int
	NumWaitSec int
}

// atoiOr0 returns the integer converted from the input string, or 0 if the input string does not represent a valid integer.
func atoiOr0(str string) int {
	ret, _ := strconv.Atoi(str)
	return ret
}

// strSliceElemOrEmpty retrieves the string element at index I from the input slice, or "" if the slice does not contain an index I.
func strSliceElemOrEmpty(slice []string, i int) string {
	if len(slice) > i {
		return slice[i]
	}
	return ""
}

// getDACIDsFromProcfs returns the real, effective, and saved UID/GID from the input space-separated string fields.
func getDACIDsFromProcfs(in string) []int {
	ids := regexp.MustCompile(`\s+`).Split(in, -1)
	return []int{
		atoiOr0(strSliceElemOrEmpty(ids, 0)),
		atoiOr0(strSliceElemOrEmpty(ids, 1)),
		atoiOr0(strSliceElemOrEmpty(ids, 2)),
	}
}

// getClockTicksPerSecond returns the number of times kernel timer interrupts every second.
func getClockTicksPerSecond() int {
	getClockTicksPerSecondOnce.Do(func() {
		// The function body is heavily inspired by github.com/tklauser/go-sysconf
		auxv, err := ioutil.ReadFile("/proc/self/auxv")
		if err == nil {
			bufPos := int(SizeOfUint / 8)
		loop:
			for i := 0; i < len(auxv)-bufPos*2; i += bufPos * 2 {
				var tag, value uint
				switch SizeOfUint {
				case 32:
					tag = uint(binary.LittleEndian.Uint32(auxv[i:]))
					value = uint(binary.LittleEndian.Uint32(auxv[i+bufPos:]))
				case 64:
					tag = uint(binary.LittleEndian.Uint64(auxv[i:]))
					value = uint(binary.LittleEndian.Uint64(auxv[i+bufPos:]))
				}
				switch tag {
				// See asm/auxvec.h for the definition of constant AT_CLKTCK ("frequency at which times() increments"), which is an integer 17.
				case 17:
					sysconfClockTick = int(value)
					break loop
				}
			}
		}
		// Apparently 100 HZ is a very common value of _SC_CLK_TCK, it seems to be this way with Linux kernel 5.4.0 on x86-64.
		sysconfClockTick = 100
	})
	return sysconfClockTick
}

func getProcStatus(statusContent, schedstatContent, statContent string) ProcessStatus {
	// Collect key-value pairs from /proc/XXXX/status
	statusKeyValue := make(map[string]string)
	for _, line := range strings.Split(statusContent, "\n") {
		submatches := regexStatusKeyValue.FindStringSubmatch(line)
		if len(submatches) > 2 {
			statusKeyValue[submatches[1]] = submatches[2]
		}
	}
	// Collect fields of strings from /proc/XXXX/schedstat
	schedstatFields := regexSchedstatFields.FindStringSubmatch(schedstatContent)
	// Collect fields of various data types from /proc/XXXX/stat
	statFields := regexStatFields.FindStringSubmatch(statContent)

	// Put the information together
	uids := getDACIDsFromProcfs(statusKeyValue["Uid"])
	gids := getDACIDsFromProcfs(statusKeyValue["Gid"])
	return ProcessStatus{
		Name:               statusKeyValue["Name"],
		Umask:              statusKeyValue["Umask"],
		ThreadGroupID:      atoiOr0(statusKeyValue["Tgid"]),
		ProcessID:          atoiOr0(statusKeyValue["Pid"]),
		ParentPID:          atoiOr0(statusKeyValue["PPid"]),
		ProcessGroupID:     atoiOr0(strSliceElemOrEmpty(statFields, 5)),
		StartedAtUptimeSec: atoiOr0(strSliceElemOrEmpty(statFields, 22)) / getClockTicksPerSecond(),
		ProcessPrivilege: ProcessPrivilege{
			RealUID:      uids[0],
			EffectiveUID: uids[1],
			RealGID:      gids[0],
			EffectiveGID: gids[1],
		},
		ProcessMemUsage: ProcessMemUsage{
			VirtualMemSizeBytes:     atoiOr0(strSliceElemOrEmpty(statFields, 23)),
			ResidentSetMemSizeBytes: atoiOr0(strSliceElemOrEmpty(statFields, 24)) * os.Getpagesize(),
		},
		ProcessCPUUsage: ProcessCPUUsage{
			NumUserModeSecInclChildren: (atoiOr0(strSliceElemOrEmpty(statFields, 14)) + atoiOr0(strSliceElemOrEmpty(statFields, 16))) / getClockTicksPerSecond(),
			NumSysModeSecInclChildren:  (atoiOr0(strSliceElemOrEmpty(statFields, 15)) + atoiOr0(strSliceElemOrEmpty(statFields, 17))) / getClockTicksPerSecond(),
		},
		ProcessSchedulerStats: ProcessSchedulerStats{
			NumVoluntaryCtxSwitches:    atoiOr0(statusKeyValue["voluntary_ctxt_switches"]),
			NumNonVoluntaryCtxSwitches: atoiOr0(statusKeyValue["nonvoluntary_ctxt_switches"]),
			NumRunSec:                  atoiOr0(strSliceElemOrEmpty(schedstatFields, 1)) / getClockTicksPerSecond(),
			NumWaitSec:                 atoiOr0(strSliceElemOrEmpty(schedstatFields, 2)) / getClockTicksPerSecond(),
		},
	}
}
