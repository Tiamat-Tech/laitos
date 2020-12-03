package handler

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/toolbox"
)

type ProcessExplorer struct {
	logger                     lalog.Logger
	stripURLPrefixFromResponse string
}

func (procexp *ProcessExplorer) Initialise(logger lalog.Logger, _ *toolbox.CommandProcessor, stripURLPrefixFromResponse string) error {
	procexp.logger = logger
	procexp.stripURLPrefixFromResponse = stripURLPrefixFromResponse
	return nil
}

func (_ *ProcessExplorer) GetRateLimitFactor() int {
	return 1
}

func (procexp *ProcessExplorer) SelfTest() error {
	return nil
}

func (procexp *ProcessExplorer) Handle(w http.ResponseWriter, r *http.Request) {
	NoCache(w)
	pidStr := r.FormValue("pid")
	pid, _ := strconv.Atoi(pidStr)
	if pid < 1 && pidStr != "self" {
		// Respond with a JSON array of PIDs available for choosing
		pidsUnderProcfs, err := filepath.Glob("/proc/[1-9]*")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if true {
		}
		pids := make([]int, 0)
		for _, pidPath := range pidsUnderProcfs {
			id, _ := strconv.Atoi(strings.TrimPrefix(pidPath, "/proc/"))
			if id > 0 {
				pids = append(pids, id)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(pids)
	}
	// Remove the prefix /proc/ from each of the return value
}
