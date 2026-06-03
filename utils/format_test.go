package utils

import (
	"testing"

	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
)

func TestFormatDurationRoundsForPrometheusLagMessages(t *testing.T) {
	assert.Equal(t, "2 minutes", FormatDuration(121*timeseries.Second, 1))
}
