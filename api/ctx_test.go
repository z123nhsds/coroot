package api

import (
	"testing"

	"github.com/coroot/coroot/cache"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
)

func TestRenderStatusFormatsPrometheusLagWarning(t *testing.T) {
	project := &db.Project{
		Id:   db.ProjectId("project-1"),
		Name: "project-1",
		Prometheus: db.IntegrationPrometheus{
			Url:             "http://prometheus.example",
			RefreshInterval: 30 * timeseries.Second,
		},
	}

	status := renderStatus(project, &cache.Status{
		LagMax: 6 * 30 * timeseries.Second,
		LagAvg: 121 * timeseries.Second,
	}, nil, nil)

	assert.Equal(t, model.WARNING, status.Status)
	assert.Equal(t, model.WARNING, status.Prometheus.Status)
	assert.Equal(t, "wait", status.Prometheus.Action)
	assert.Contains(t, status.Prometheus.Message, "2 minutes")
}
