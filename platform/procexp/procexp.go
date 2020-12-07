package procexp

import (
	"path/filepath"
	"strconv"
	"strings"
)

/*
GetProcIDs returns the list of all process IDs visible to this program according to the
information available from procfs.
*/
func GetProcIDs() (ret []int) {
	ret = make([]int, 0)
	pidsUnderProcfs, err := filepath.Glob("/proc/[1-9]*")
	if err != nil {
		return
	}
	for _, pidPath := range pidsUnderProcfs {
		// Remove the prefix /proc/ from each of the return value
		id, _ := strconv.Atoi(strings.TrimPrefix(pidPath, "/proc/"))
		if id > 0 {
			ret = append(ret, id)
		}
	}
	return
}

// GetProcStatus uses /proc/XXXX to discover the general status and resource usage information about a process.
func GetProcStatus(pid int) (status ProcessStatus, err error) {
	return
}
