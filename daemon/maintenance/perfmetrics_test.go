package maintenance

import (
	"testing"

	"github.com/HouzuoGuo/laitos/misc"
)

func TestProcessExplorerMetrics(t *testing.T) {
	promInteg := []struct {
		enabled bool
	}{
		{true},
		{false},
	}

	for _, enabled := range promInteg {
		misc.EnablePrometheusIntegration = enabled.enabled
		metrics := NewProcessExplorerMetrics()
		if err := metrics.RegisterGlobally(); err != nil {
			t.Fatal(err)
		}
		if err := metrics.Refresh(); err != nil {
			t.Fatal(err)
		}
	}
}
