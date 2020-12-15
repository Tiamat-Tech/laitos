package handler

import (
	"net/http"
	"testing"
)

func TestProcessExplorer_Handle(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "", nil)
	if err != nil {
		t.Fatal(err)
	}

}
