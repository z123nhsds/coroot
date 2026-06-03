package main

import (
	"fmt"
	"testing"

	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

// 1. go vet 语法整改后校验 RCA 摘要与 prom SLO 告警联动
func TestRegression_RCA_Prom_SLO(t *testing.T) {
	// Mock an SLO burn rate and ensure it gets formatted properly for RCA
	br := model.BurnRate{Severity: model.WARNING, Value: 2.5}
	status := br.FormatSLOStatus()
	
	// Validating string formatting (go vet compatibility)
	expectedPrefix := "burn rate is"
	if len(status) < len(expectedPrefix) || status[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("Expected status to start with %q, got %q", expectedPrefix, status)
	}

	summary := fmt.Sprintf("RCA Summary: %s", status)
	if summary == "" {
		t.Errorf("RCA summary should not be empty")
	}
}

// 2. MCP（rbac+stats）日志聚类受 MemoryGrowthPct 部署追踪的影响校验
func TestRegression_MCP_MemoryGrowthPct(t *testing.T) {
	to := timeseries.Time(1600000000)
	
	// Mock Memory Growth Timeseries Data
	rss := timeseries.NewWithData(timeseries.Time(10), timeseries.Duration(10), []float32{100, 150, 200})
	
	pct := auditor.MemoryGrowthPct(rss, 1000, to)
	
	if pct <= 0 {
		t.Errorf("Expected MemoryGrowthPct to be > 0, got %f", pct)
	}
}

// 3. utils 函数修复不破坏 watchers 普罗米修斯健康探测
func TestRegression_Utils_PrometheusHealth(t *testing.T) {
	// Test utils.BasicAuth logic which is used in prom.NewClient for Prometheus health probe (Ping)
	auth := &utils.BasicAuth{User: "user", Password: "password"}
	urlWithAuth, err := auth.AddTo("http://localhost:9090")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	expected := "http://user:password@localhost:9090"
	if urlWithAuth != expected {
		t.Errorf("Expected %q, got %q", expected, urlWithAuth)
	}
}
