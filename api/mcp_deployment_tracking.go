package api

import (
	"context"
	"sort"

	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/rbac"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/mark3labs/mcp-go/mcp"
	"k8s.io/klog"
)

type mcpDeploymentInfo struct {
	Id               string  `json:"id"`
	Version          string  `json:"version"`
	DeployedAgo      string  `json:"deployed_ago"`
	Status           string  `json:"status"`
	State            string  `json:"state"`
	Message          string  `json:"message,omitempty"`
	MemoryLeakPct    float32 `json:"memory_leak_pct,omitempty"`
	MemoryLeakStatus string  `json:"memory_leak_status,omitempty"`
	MemoryUsage      int64   `json:"memory_usage,omitempty"`
	OOMKills         int64   `json:"oom_kills,omitempty"`
	Requests         int64   `json:"requests,omitempty"`
	Errors           int64   `json:"errors,omitempty"`
	Restarts         int64   `json:"restarts,omitempty"`
}

type mcpDeploymentDetail struct {
	Id             string                    `json:"id"`
	Version        string                    `json:"version"`
	Status         string                    `json:"status"`
	State          string                    `json:"state"`
	Message        string                    `json:"message,omitempty"`
	StartedAt      string                    `json:"started_at"`
	FinishedAt     string                    `json:"finished_at,omitempty"`
	Lifetime       string                    `json:"lifetime"`
	Summary        []mcpDeploymentSummary    `json:"summary,omitempty"`
	MemoryAnalysis *mcpDeploymentMemory      `json:"memory_analysis,omitempty"`
}

type mcpDeploymentSummary struct {
	Report  string `json:"report"`
	Ok      bool   `json:"ok"`
	Message string `json:"message"`
	Emoji   string `json:"emoji"`
}

type mcpDeploymentMemory struct {
	LeakPercent       float32            `json:"leak_percent"`
	LeakStatus        string             `json:"leak_status"`
	LeakMessage       string             `json:"leak_message"`
	MemoryUsage       int64              `json:"memory_usage"`
	MemoryLimitMax    float32            `json:"memory_limit_max,omitempty"`
	HasLimit          bool               `json:"has_limit"`
	OOMKills          int64              `json:"oom_kills"`
	ContainerCount    int                `json:"container_count"`
	InstanceCount     int                `json:"instance_count"`
	PerContainer      map[string]float32 `json:"per_container,omitempty"`
}

func (h *MCPHandler) registerDeploymentTools() {
	h.AddTool(
		mcp.NewTool("list_deployments",
			mcp.WithDescription("List recent deployments for an application, including memory leak analysis for each deployment. Returns deployment id, version, status, memory leak percentage, OOM kills, and request counts."),
			mcp.WithString("app_id",
				mcp.Required(),
				mcp.Description("Application id from list_applications (4-part 'cluster_id:namespace:Kind:name')."),
			),
			mcp.WithNumber("hours",
				mcp.Description("Look-back window in hours. Default: 168 (7 days), max: 720 (30 days)."),
			),
			mcp.WithNumber("limit",
				mcp.Description("Max deployments to return. Default: 20, max: 100."),
			),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
		),
		h.toolListDeployments,
	)
	h.AddTool(
		mcp.NewTool("get_deployment_detail",
			mcp.WithDescription("Get detailed information about a specific deployment, including full summary reports and memory leak analysis with per-container breakdown."),
			mcp.WithString("app_id",
				mcp.Required(),
				mcp.Description("Application id from list_applications."),
			),
			mcp.WithString("deployment_id",
				mcp.Required(),
				mcp.Description("Deployment id from list_deployments."),
			),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
		),
		h.toolGetDeploymentDetail,
	)
	h.AddTool(
		mcp.NewTool("analyze_deployment_memory",
			mcp.WithDescription("Run a detailed memory leak analysis for all instances/containers of an application. Returns per-container memory growth percentage, leak severity classification, and estimated time to double memory usage."),
			mcp.WithString("app_id",
				mcp.Required(),
				mcp.Description("Application id from list_applications."),
			),
			mcp.WithString("deployment_id",
				mcp.Description("Specific deployment id from list_deployments. If omitted, analyzes current state."),
			),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
		),
		h.toolAnalyzeDeploymentMemory,
	)
}

func (h *MCPHandler) loadWorldForApp(ctx context.Context, appIdStr string) (*model.World, *model.Application, *mcp.CallToolResult) {
	user, project, errResult := h.RequireUserAndProject(ctx)
	if errResult != nil {
		return nil, nil, errResult
	}
	parsed, err := model.NewApplicationIdFromString(appIdStr, string(project.Id))
	if err != nil {
		return nil, nil, mcp.NewToolResultError("invalid application id: " + err.Error())
	}
	if parsed.ClusterId != string(project.Id) {
		return nil, nil, mcp.NewToolResultError("application id does not match selected project")
	}
	now := timeseries.Now()
	world, _, err := h.Api.LoadWorld(ctx, project, now.Add(-timeseries.Hour), now)
	if err != nil {
		klog.Errorln("mcp: loadWorldForApp:", err)
		return nil, nil, mcp.NewToolResultError("failed to load world")
	}
	if world == nil {
		return nil, nil, mcp.NewToolResultError("no data available")
	}
	auditor.Audit(world, project, nil, nil)
	for _, app := range world.Applications {
		if app.Id.String() == appIdStr {
			if !h.Api.IsAllowed(user, rbac.Actions.Project(string(project.Id)).Application(app.Category, app.Id.Namespace, app.Id.Kind, app.Id.Name).View()) {
				return nil, nil, mcp.NewToolResultError("access denied")
			}
			return world, app, nil
		}
	}
	_ = parsed
	return nil, nil, mcp.NewToolResultError("application not found in current data")
}

func (h *MCPHandler) toolListDeployments(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	appId, err := req.RequireString("app_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	world, app, errResult := h.loadWorldForApp(ctx, appId)
	if errResult != nil {
		return errResult, nil
	}
	_ = world

	hours := int64(168)
	if h, ok, _ := req.GetNumber("hours"); ok {
		hours = clampInt64(int64(h), 1, 720)
	}
	limit := int64(20)
	if l, ok, _ := req.GetNumber("limit"); ok {
		limit = clampInt64(int64(l), 1, 100)
	}

	cutoff := timeseries.Now().Add(-timeseries.Duration(hours) * timeseries.Hour)
	now := timeseries.Now()

	statuses := model.CalcApplicationDeploymentStatuses(app, world.CheckConfigs, now)

	var result []mcpDeploymentInfo
	for i := len(statuses) - 1; i >= 0 && int64(len(result)) < limit; i-- {
		ds := statuses[i]
		if ds.Deployment.StartedAt < cutoff {
			continue
		}
		info := mcpDeploymentInfo{
			Id:          ds.Deployment.Id(),
			Version:     ds.Deployment.Version(),
			DeployedAgo: utils.FormatDuration(now.Sub(ds.Deployment.StartedAt), 1) + " ago",
			Status:      ds.Status.String(),
			State:       deploymentStateToString(ds.State),
			Message:     ds.Message,
		}
		if ds.Deployment.MetricsSnapshot != nil {
			info.MemoryLeakPct = ds.Deployment.MetricsSnapshot.MemoryLeakPercent
			info.MemoryUsage = ds.Deployment.MetricsSnapshot.MemoryUsage
			info.OOMKills = ds.Deployment.MetricsSnapshot.OOMKills
			info.Requests = ds.Deployment.MetricsSnapshot.Requests
			info.Errors = ds.Deployment.MetricsSnapshot.Errors
			info.Restarts = ds.Deployment.MetricsSnapshot.Restarts

			severity, _ := auditor.FormatMemoryLeakStatus(ds.Deployment.MetricsSnapshot.MemoryLeakPercent)
			info.MemoryLeakStatus = severity
		}
		result = append(result, info)
	}

	if result == nil {
		result = []mcpDeploymentInfo{}
	}
	return MCPJSON(result)
}

func (h *MCPHandler) toolGetDeploymentDetail(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	appId, err := req.RequireString("app_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	deploymentId, err := req.RequireString("deployment_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	world, app, errResult := h.loadWorldForApp(ctx, appId)
	if errResult != nil {
		return errResult, nil
	}
	now := timeseries.Now()

	statuses := model.CalcApplicationDeploymentStatuses(app, world.CheckConfigs, now)
	for _, ds := range statuses {
		if ds.Deployment.Id() != deploymentId {
			continue
		}
		detail := mcpDeploymentDetail{
			Id:        ds.Deployment.Id(),
			Version:   ds.Deployment.Version(),
			Status:    ds.Status.String(),
			State:     deploymentStateToString(ds.State),
			Message:   ds.Message,
			StartedAt: ds.Deployment.StartedAt.ToStandard().Format("2006-01-02T15:04:05Z"),
			Lifetime:  utils.FormatDuration(ds.Lifetime, 1),
		}
		if !ds.Deployment.FinishedAt.IsZero() {
			detail.FinishedAt = ds.Deployment.FinishedAt.ToStandard().Format("2006-01-02T15:04:05Z")
		}
		for _, s := range ds.Summary {
			detail.Summary = append(detail.Summary, mcpDeploymentSummary{
				Report:  string(s.Report),
				Ok:      s.Ok,
				Message: s.Message,
				Emoji:   s.Emoji(),
			})
		}
		if detail.Summary == nil {
			detail.Summary = []mcpDeploymentSummary{}
		}

		if ds.Deployment.MetricsSnapshot != nil {
			memAnalysis := auditor.AnalyzeDeploymentMemory(app, now)
			leakPct := ds.Deployment.MetricsSnapshot.MemoryLeakPercent
			severity, msg := auditor.FormatMemoryLeakStatus(leakPct)
			detail.MemoryAnalysis = &mcpDeploymentMemory{
				LeakPercent:    leakPct,
				LeakStatus:     severity,
				LeakMessage:    msg,
				MemoryUsage:    memAnalysis.MemoryUsage,
				MemoryLimitMax: memAnalysis.MemoryLimitMax,
				HasLimit:       memAnalysis.HasLimit,
				OOMKills:       memAnalysis.OOMKills,
				ContainerCount: memAnalysis.ContainerCount,
				InstanceCount:  memAnalysis.InstanceCount,
				PerContainer:   memAnalysis.MemoryGrowthRss,
			}
		}

		return MCPJSON(detail)
	}
	return mcp.NewToolResultError("deployment not found"), nil
}

func (h *MCPHandler) toolAnalyzeDeploymentMemory(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	appId, err := req.RequireString("app_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	world, app, errResult := h.loadWorldForApp(ctx, appId)
	if errResult != nil {
		return errResult, nil
	}

	now := timeseries.Now()
	deploymentId := req.GetString("deployment_id", "")

	var targetDeployment *model.ApplicationDeployment
	if deploymentId != "" {
		statuses := model.CalcApplicationDeploymentStatuses(app, world.CheckConfigs, now)
		for _, ds := range statuses {
			if ds.Deployment.Id() == deploymentId {
				targetDeployment = ds.Deployment
				break
			}
		}
	}

	result := auditor.AnalyzeDeploymentMemory(app, now)
	severity, msg := auditor.FormatMemoryLeakStatus(result.MemoryLeakPct)

	perContainer := make([]map[string]any, 0, len(result.MemoryGrowthRss))
	type containerEntry struct {
		name string
		pct  float32
	}
	var entries []containerEntry
	for name, pct := range result.MemoryGrowthRss {
		entries = append(entries, containerEntry{name: name, pct: pct})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].pct > entries[j].pct
	})
	for _, e := range entries {
		s, _ := auditor.FormatMemoryLeakStatus(e.pct)
		perContainer = append(perContainer, map[string]any{
			"container":     e.name,
			"growth_pct":    e.pct,
			"leak_severity": s,
		})
	}

	response := map[string]any{
		"app_id":            appId,
		"leak_percent":      result.MemoryLeakPct,
		"leak_severity":     severity,
		"leak_message":      msg,
		"memory_usage":      result.MemoryUsage,
		"memory_limit_max":  result.MemoryLimitMax,
		"has_limit":         result.HasLimit,
		"oom_kills":         result.OOMKills,
		"container_count":   result.ContainerCount,
		"instance_count":    result.InstanceCount,
		"per_container":     perContainer,
	}

	if targetDeployment != nil {
		response["deployment_id"] = targetDeployment.Id()
		response["deployment_version"] = targetDeployment.Version()
		if targetDeployment.MetricsSnapshot != nil {
			response["snapshot_memory_leak_pct"] = targetDeployment.MetricsSnapshot.MemoryLeakPercent
		}
	}

	_ = world
	return MCPJSON(response)
}

func deploymentStateToString(s model.ApplicationDeploymentState) string {
	switch s {
	case model.ApplicationDeploymentStateStarted:
		return "started"
	case model.ApplicationDeploymentStateInProgress:
		return "in_progress"
	case model.ApplicationDeploymentStateStuck:
		return "stuck"
	case model.ApplicationDeploymentStateCancelled:
		return "cancelled"
	case model.ApplicationDeploymentStateDeployed:
		return "deployed"
	case model.ApplicationDeploymentStateSummary:
		return "summary"
	default:
		return "unknown"
	}
}

func clampInt64(v, min, max int64) int64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}