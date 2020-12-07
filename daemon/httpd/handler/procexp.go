package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/toolbox"
)

/*
ProcessExplorer is an HTTP handler that responds with process IDs that are running on the system, and when given a PID as query
parameter, the handler inspects the process for its current status and activities for the response.
*/
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(pids)
	}
}
