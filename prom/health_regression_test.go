package prom

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthProbeEndpoint(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestPrometheusHealthCheckViaQueryRange(t *testing.T) {
	data := `{"status":"success","data":{"resultType":"matrix","result":[]}}`

	h := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/query_range", r.URL.Path)
		w.Write([]byte(data))
	}
	ts := httptest.NewServer(http.HandlerFunc(h))
	defer ts.Close()

	cfg := &db.IntegrationPrometheus{
		Url:           ts.URL,
		TlsSkipVerify: true,
	}
	client, err := NewClient(cfg, nil)
	require.NoError(t, err)

	ctx := context.Background()
	now := timeseries.Now()
	res, err := client.QueryRange(ctx, `up`, nil, now.Add(-timeseries.Minute), now, timeseries.Duration(15))
	assert.NoError(t, err)
	assert.NotNil(t, res)
}

func TestClickHouseClientNilConfig(t *testing.T) {
	_, err := newClickHouse(nil, timeseries.Minute)
	assert.Error(t, err)
}

func TestPrometheusHTTPClientTimeout(t *testing.T) {
	cfg := &db.IntegrationPrometheus{
		Url:           "http://localhost:1",
		TlsSkipVerify: true,
	}
	client, err := NewClient(cfg, nil)
	require.NoError(t, err)

	ctx := context.Background()
	now := timeseries.Now()
	_, err = client.QueryRange(ctx, `up`, nil, now.Add(-timeseries.Minute), now, timeseries.Duration(15))
	assert.Error(t, err)
}

func TestPrintfFormatSafety(t *testing.T) {
	check := func(src, extraSelector, expected string) {
		actual, err := AddExtraSelector(src, extraSelector)
		assert.NoError(t, err)
		assert.Equal(t, expected, actual)
	}

	check(
		`rate(metric{label="%s"}[1m])`,
		`{cluster="test"}`,
		`rate(metric{cluster="test",label="%s"}[1m])`)

	check(
		`rate(metric{label="%d"}[1m])`,
		`{env="prod"}`,
		`rate(metric{env="prod",label="%d"}[1m])`)

	check(
		`rate(metric{label="%v"}[1m])`,
		`{region="us-east"}`,
		`rate(metric{region="us-east",label="%v"}[1m])`)
}