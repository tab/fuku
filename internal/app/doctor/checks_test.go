package doctor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"fuku/internal/model"
)

func Test_timed(t *testing.T) {
	r := timed(func() model.Result {
		return model.Result{ID: "x", Severity: model.SeverityOK}
	})

	assert.Equal(t, model.CheckID("x"), r.ID)
	assert.GreaterOrEqual(t, r.Duration, time.Duration(0))
}

func Test_skipped(t *testing.T) {
	r := skipped(model.CheckRuntimePorts, model.CategoryRuntime, "config did not load")

	assert.Equal(t, model.Result{
		ID:       model.CheckRuntimePorts,
		Category: model.CategoryRuntime,
		Severity: model.SeverityIdle,
		Summary:  "skipped (config did not load)",
	}, r)
}
