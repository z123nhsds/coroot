# Coroot 代码重构建议

## 1. 依赖隔离建议

### 当前 go.mod 分析
当前项目依赖可以分为以下几类：
- **核心依赖**: github.com/gorilla/mux, gopkg.in/alecthomas/kingpin.v2 等
- **监控/时序数据库**: github.com/prometheus/*, github.com/ClickHouse/*
- **通知**: github.com/slack-go/slack, github.com/PagerDuty/go-pagerduty 等
- **云集成**: cloud 相关包
- **工具**: github.com/gonum.org/v1/gonum, github.com/mark3labs/mcp-go 等

### 依赖隔离建议

#### 方案 1: 模块化 go.mod（推荐）
将项目拆分为多个独立的 Go 模块：
```
coroot/
├── go.mod (核心模块)
├── prom/
│   └── go.mod (Prometheus 模块)
├── clickhouse/
│   └── go.mod (ClickHouse 模块)
├── notifications/
│   └── go.mod (通知模块)
└── mcp/
    └── go.mod (MCP 模块)
```

#### 方案 2: 功能分组
保持单一 go.mod，但将依赖按功能分组管理：
```go
require (
    // Core framework
    github.com/gorilla/mux v1.8.0
    gopkg.in/alecthomas/kingpin.v2 v2.2.6

    // Observability & metrics
    github.com/prometheus/client_golang v1.20.5
    github.com/prometheus/common v0.60.1
    github.com/prometheus/prometheus v0.300.0

    // Databases
    github.com/ClickHouse/ch-go v0.62.0
    github.com/ClickHouse/clickhouse-go/v2 v2.8.3
    github.com/lib/pq v1.10.7
    github.com/mattn/go-sqlite3 v1.14.15

    // Notifications
    github.com/PagerDuty/go-pagerduty v1.6.0
    github.com/atc0005/go-teams-notify/v2 v2.13.0
    github.com/slack-go/slack v0.11.3
    github.com/opsgenie/opsgenie-go-sdk-v2 v1.2.14

    // MCP
    github.com/mark3labs/mcp-go v0.45.0
)
```

### 依赖更新建议
- 按最小化原则更新依赖：只更新必要的安全补丁和功能改进
- 使用 dependabot 或 renovate 自动化依赖更新
- 每个模块独立进行版本管理，降低回归风险

## 2. 通用 RCA 上下文集成方案

### 目标
在不破坏现有 prom 和 clickhouse 模块的前提下，引入通用的 RCA（Root Cause Analysis）上下文。

### 架构设计
```
┌─────────────────────────────────────────┐
│         RCA Context Layer               │
│  ┌─────────────┐  ┌─────────────────┐ │
│  │ RCA Engine  │  │ Context Manager │ │
│  └─────────────┘  └─────────────────┘ │
├─────────────────────────────────────────┤
│    Data Source Abstraction Layer       │
│  ┌─────────────┐  ┌─────────────────┐ │
│  │ Prometheus  │  │   ClickHouse    │ │
│  │   Adapter   │  │    Adapter      │ │
│  └─────────────┘  └─────────────────┘ │
├─────────────────────────────────────────┤
│         Existing Modules               │
│  ┌─────────────┐  ┌─────────────────┐ │
│  │   Prom      │  │   ClickHouse    │ │
│  └─────────────┘  └─────────────────┘ │
└─────────────────────────────────────────┘
```

### 实现步骤

#### 第一步：定义通用接口
创建 `/rca/` 包，定义通用接口：
```go
// rca/context.go
package rca

type DataSource interface {
    Query(ctx context.Context, query string, from, to time.Time) (interface{}, error)
}

type Context struct {
    ProjectID   string
    StartTime   time.Time
    EndTime     time.Time
    DataSources map[string]DataSource
}

func NewContext(projectID string, start, end time.Time) *Context {
    return &Context{
        ProjectID:   projectID,
        StartTime:   start,
        EndTime:     end,
        DataSources: make(map[string]DataSource),
    }
}

func (c *Context) RegisterDataSource(name string, ds DataSource) {
    c.DataSources[name] = ds
}
```

#### 第二步：为现有模块创建适配器
为 Prom 和 ClickHouse 创建适配器，保持现有模块不变：
```go
// rca/prom_adapter.go
package rca

import "github.com/coroot/coroot/prom"

type PromAdapter struct {
    client *prom.Client
}

func NewPromAdapter(client *prom.Client) *PromAdapter {
    return &PromAdapter{client: client}
}

func (a *PromAdapter) Query(ctx context.Context, query string, from, to time.Time) (interface{}, error) {
    // 适配现有 prom 包的查询方法
    return a.client.QueryRange(ctx, query, from, to)
}

// rca/clickhouse_adapter.go
package rca

import "github.com/coroot/coroot/clickhouse"

type ClickHouseAdapter struct {
    client *clickhouse.Client
}

func NewClickHouseAdapter(client *clickhouse.Client) *ClickHouseAdapter {
    return &ClickHouseAdapter{client: client}
}

func (a *ClickHouseAdapter) Query(ctx context.Context, query string, from, to time.Time) (interface{}, error) {
    // 适配现有 clickhouse 包的查询方法
    return a.client.Query(ctx, query, from, to)
}
```

#### 第三步：在 API 层集成
在 api/rca.go 中集成新的 RCA 上下文：
```go
func (api *Api) RCA(w http.ResponseWriter, r *http.Request, u *db.User, project *db.Project) {
    // 解析参数
    appID := mux.Vars(r)["app"]
    from, to := parseTimeRange(r)

    // 创建 RCA 上下文
    rcaCtx := rca.NewContext(string(project.Id), from, to)

    // 注册数据源适配器（不修改原有模块）
    if globalProm := api.globalPrometheus; globalProm != nil {
        rcaCtx.RegisterDataSource("prometheus", rca.NewPromAdapter(prom.NewClient(globalProm)))
    }
    if globalCH := api.globalClickHouse; globalCH != nil {
        rcaCtx.RegisterDataSource("clickhouse", rca.NewClickHouseAdapter(clickhouse.NewClient(globalCH)))
    }

    // 执行 RCA 分析
    result, err := rcaEngine.Analyze(rcaCtx, appID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // 返回结果
    json.NewEncoder(w).Encode(result)
}
```

### 关键优势
- 无需修改现有 prom 和 clickhouse 模块的代码
- 使用适配器模式保持向后兼容
- 灵活可扩展，未来可以轻松添加新的数据源
- 统一的 RCA 接口，便于维护和测试

## 3. MCP 服务器重构成果
- 创建新的 `/mcp/` 包，包含完整的服务器实现
- 支持多版本 Go（1.25+）
- 内置 SLSA3 provenance 支持，包含构建元数据
- 保持与现有 API 向后兼容

## 4. MemoryGrowthPct 抽象成果
- 创建新的 `/analysis/` 公共包
- 删除了 `auditor/memory.go` 中重复的代码
- 统一了 `auditor` 和 `watchers` 包中的实现
