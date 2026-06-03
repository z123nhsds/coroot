# Coroot 跨模块回归用例落地文档

## 概述

本文档描述在 Coroot 源码目录（Go1.25 + ClickHouse + eBPF + Vue）中落地的三项跨模块回归用例，涵盖 notifications/prom/rbac/stats/utils/watchers 模块的集成验证。

## 用例文件

| 用例 | 文件 | 涉及模块 |
|------|------|----------|
| 用例1 | `regression/rca_slo_alert_linkage_test.go` | auditor, model, notifications |
| 用例2 | `regression/mcp_rbac_stats_memory_growth_test.go` | auditor, rbac, stats, model |
| 用例3 | `regression/utils_watchers_prom_health_test.go` | utils, prom, rbac, model |

---

## 用例1：go vet 语法整改后校验 RCA 摘要与 prom SLO 告警联动

### 目标
确保 `go vet` 语法整改后，RCA（根因分析）摘要生成流程与 Prometheus SLO 告警规则的联动不被破坏。

### 测试场景

| 子测试 | 验证点 |
|--------|--------|
| `SLOAvailabilityCheckProducesValidSummaryForAlert` | 构造 AvailabilitySLI 数据，经 `auditor.Audit()` 审计后，SLO 报告中存在 SLOAvailability 检查且带有摘要消息 |
| `SLOLatencyCheckProducesValidSummaryForAlert` | 构造 LatencySLI 直方图数据，经审计后，SLOLatency 检查存在且带有摘要消息 |
| `SLOCheckStatusFlowsToAlertingRuleMatch` | 验证 AlertingRule 的 `Matches()` 方法能正确匹配应用，且 `AlertSourceTypeCheck` 与 `CheckSource.CheckId` 链路完整 |
| `AlertDetailPromQLExcludedFromNotification` | 验证通知发送时 PromQL/PromQLChart 类型的 AlertDetail 被过滤，不泄露查询表达式 |
| `BurnRateFormatSLOStatusConsistent` | 验证 `BurnRate.FormatSLOStatus()` 输出包含消耗速率倍数和时间窗口，确保 RCA 摘要可读性 |
| `SLOCheckIdConsistentWithAlertingRuleSource` | 验证 `Checks.SLOAvailability.Id` 和 `Checks.SLOLatency.Id` 与 `init()` 反射赋值一致，确保告警规则源映射不漂移 |

### 跨模块数据流
```
model.AvailabilitySLI / LatencySLI
  → auditor.slo() 审计
    → model.AuditReport (SLO) + model.Check (SLOAvailability/SLOLatency)
      → model.AlertingRule.Matches() 匹配应用
        → model.Alert 生成（含 AlertDetail）
          → notifications 过滤 PromQL 详情后发送
```

### 关键代码引用
- [auditor/slo.go](auditor/slo.go) - SLO 审计逻辑
- [model/sli.go](model/sli.go) - SLI 数据结构
- [model/alert.go](model/alert.go) - BurnRate 与 AlertDetail
- [model/alerting_rule.go](model/alerting_rule.go) - AlertingRule.Matches()
- [model/check.go](model/check.go) - CheckId 反射赋值

---

## 用例2：MCP（rbac+stats）日志聚类受 MemoryGrowthPct 部署追踪的影响校验

### 目标
确保 `MemoryGrowthPct` 计算结果正确传播至部署追踪的 `MetricsSnapshot.MemoryLeakPercent`，且 MCP 调用链路中 rbac 权限和 stats 统计不受影响。

### 测试场景

| 子测试 | 验证点 |
|--------|--------|
| `MemoryGrowthPctPropagatesToDeploymentMetricsSnapshot` | 构造单调递增 RSS 时序数据，验证 `MemoryGrowthPct` 返回正值，且 Container.MemoryRss 正确设置 |
| `MemoryGrowthPctZeroForStableMemory` | 构造恒定 RSS 数据，验证 `MemoryGrowthPct` 返回 0 |
| `MemoryGrowthPctZeroForEmptyTimeSeries` | 空 TimeSeries 输入返回 0，防止空指针 |
| `StatsCollectorTracksMcpCallsByTool` | 验证 `stats.Stats.UX.McpCalls` 按 tool 名追踪调用次数 |
| `RbacPermissionsGateMcpProjectAccess` | Admin 角色可 View/Edit Alerts，Viewer 角色仅可 View Alerts |
| `RbacPermissionsGateMcpLogAccess` | Editor/Viewer 角色均可 View Logs，符合 MCP 日志聚类需求 |
| `LogClusteringSeverityMapping` | 验证 `model.SeverityError` 级别的 LogMessages 可正确挂载到 Application |
| `DeploymentSummaryReflectsMemoryLeakFromMetricsSnapshot` | 构造 `MemoryLeakPercent=15` 的 MetricsSnapshot，验证 `CalcApplicationDeploymentSummary` 生成包含 "memory leak detected" 的 Memory 报告摘要 |

### 跨模块数据流
```
auditor.MemoryGrowthPct(rss, limit, to)
  → model.MetricsSnapshot.MemoryLeakPercent
    → model.CalcApplicationDeploymentSummary()
      → model.ApplicationDeploymentSummary (Report: Memory)

rbac.Roles[x].Permissions
  → rbac.PermissionSet.Allows(Actions.Project(id).Alerts().View/Edit())
    → MCP 调用鉴权

stats.Stats.UX.McpCalls
  → MCP 工具调用计数
```

### 关键代码引用
- [auditor/memory.go](auditor/memory.go) - MemoryGrowthPct 计算
- [model/application_deployment.go](model/application_deployment.go) - CalcApplicationDeploymentSummary
- [rbac/role.go](rbac/role.go) - 角色权限定义
- [rbac/actions.go](rbac/actions.go) - Actions 构建器
- [stats/stats.go](stats/stats.go) - Stats.UX.McpCalls

---

## 用例3：utils 函数修复不破坏 watchers 普罗米修斯健康探测

### 目标
确保 `utils` 包核心函数（GlobMatch、GlobValidate、NanoId、Truncate、StringSet、FormatFloat/Percentage/Bytes）的修复或重构不破坏 watchers 的 Prometheus 健康探测、告警规则匹配和 RBAC 权限校验。

### 测试场景

| 子测试 | 验证点 |
|--------|--------|
| `GlobMatchSupportsAlertingRuleApplicationPatterns` | 验证通配符 `*`、前缀匹配 `checkout*` 在应用 ID 模式下的正确性 |
| `GlobMatchSupportsRbacScopePatterns` | 验证 RBAC scope 模式匹配（`project.*`、`project.node`） |
| `GlobMatchSupportsRbacActionPatterns` | 验证 RBAC action 动词匹配（`view`、`edit`） |
| `GlobMatchSupportsRbacObjectPatterns` | 验证 RBAC object 值模式匹配（`foo*`） |
| `GlobValidateRejectsInvalidPatterns` | 验证 `GlobValidate` 拒绝非法模式 `[` |
| `AlertingRuleMatchesUsesGlobMatch` | 端到端验证 `AlertingRule.Matches()` 调用 GlobMatch 的 all/category/pattern 三种选择器 |
| `RbacPermissionAllowsUsesGlobMatch` | 端到端验证 `Permission.Allows()` 调用 GlobMatch 进行 scope/action/object 匹配 |
| `PromFilterLabelsKeepAllAllowsHealthProbeMetrics` | 验证 `FilterLabelsKeepAll` 允许 `up` 指标和 `__name__`、`instance` 标签通过（健康探测必需） |
| `PromFilterLabelsDropAllBlocksAllLabels` | 验证 `FilterLabelsDropAll` 阻止所有标签 |
| `NanoIdGeneratesUniqueIdsForAlerts` | 验证 `NanoId` 生成指定长度且唯一的 ID |
| `TruncatePreservesAlertDetailIntegrity` | 验证 `Truncate` 短字符串不变、长字符串截断并加省略号 |
| `StringSetOperationsForStatsCollection` | 验证 `StringSet` 去重、成员检测、排序输出 |
| `FormatFunctionsConsistentForPrometheusHealthProbeOutput` | 验证 `FormatFloat`、`FormatPercentage`、`FormatBytes` 输出格式一致性 |

### 跨模块数据流
```
utils.GlobMatch
  → model.AlertingRule.Matches() (应用选择器匹配)
  → rbac.Permission.allows() (权限校验)
  → model.CheckConfigs.getRaw() (检查配置模式匹配)

utils.GlobValidate
  → model.AlertingRule 验证 (应用 ID 模式合法性)

prom.FilterLabelsKeepAll / FilterLabelsDropAll
  → prom.HttpClient.QueryRange() (Prometheus 健康探测查询)
  → prom.ClickHouse.QueryRange() (ClickHouse 健康探测查询)

utils.NanoId
  → model.Alert.Id 生成

utils.FormatFloat / FormatPercentage / FormatBytes
  → model.Check.Unit.FormatValue() (检查结果格式化)
  → model.ApplicationDeploymentSummary 摘要输出
```

### 关键代码引用
- [utils/glob.go](utils/glob.go) - GlobMatch / GlobValidate
- [utils/format.go](utils/format.go) - FormatFloat / FormatPercentage / FormatBytes / Truncate
- [utils/stringset.go](utils/stringset.go) - StringSet
- [utils/nanoid.go](utils/nanoid.go) - NanoId
- [prom/utils.go](prom/utils.go) - FilterLabelsKeepAll / FilterLabelsDropAll
- [prom/http.go](prom/http.go) - HttpClient 健康探测
- [prom/clickhouse.go](prom/clickhouse.go) - ClickHouse 健康探测

---

## CI 兼容性

### GitHub Actions（.github/workflows/ci.yml）

现有 CI 流水线已自动覆盖回归用例：

| CI 步骤 | 命令 | 覆盖范围 |
|---------|------|---------|
| go vet | `go vet ./...` | 包含 `regression/` 包的语法检查 |
| go test | `go test ./...` | 包含 `regression/` 包的测试执行 |
| go build | `go build -mod=readonly .` | 不受影响（regression 包仅为 `_test.go`） |

### Makefile

| 目标 | 命令 | 兼容性 |
|------|------|--------|
| `go-vet` | `go vet ./...` | ✅ 自动包含 |
| `go-test` | `go test ./...` | ✅ 自动包含 |
| `go-fmt` | `gofmt -w .` | ✅ 自动包含 |
| `go-imports` | `goimports -w .` | ✅ 自动包含 |

### Playwright E2E / nx 缓存 / knip 死代码检测

当前项目前端（`front/`）使用 vue-cli-service，未配置 Playwright E2E、nx 缓存或 knip 死代码检测。回归用例为纯 Go 测试，不引入前端变更，因此：

- **Playwright E2E**：无需额外配置，回归用例不涉及前端交互
- **nx 缓存**：项目未使用 nx，回归用例无影响
- **knip 死代码检测**：项目未使用 knip，回归用例仅引入测试代码（`_test.go`），不会产生死代码

### 运行方式

```bash
# 运行全部回归用例
go test ./regression/ -v

# 运行单个用例
go test ./regression/ -v -run TestRcaSummarySloAlertLinkage_AfterGoVet
go test ./regression/ -v -run TestMcpRbacStatsLogClustering_MemoryGrowthPctImpact
go test ./regression/ -v -run TestUtilsFunctions_WatchersPrometheusHealthProbe

# go vet 检查
go vet ./regression/
```

---

## 依赖关系

回归用例依赖以下现有模块（无新增外部依赖）：

| 模块 | 用途 |
|------|------|
| `github.com/coroot/coroot/auditor` | MemoryGrowthPct 计算、Audit 审计流程 |
| `github.com/coroot/coroot/db` | Project 数据结构 |
| `github.com/coroot/coroot/model` | SLI、Check、Alert、AlertingRule、ApplicationDeployment 等核心模型 |
| `github.com/coroot/coroot/timeseries` | 时序数据操作 |
| `github.com/coroot/coroot/rbac` | 权限校验 |
| `github.com/coroot/coroot/stats` | 统计收集 |
| `github.com/coroot/coroot/prom` | Prometheus 标签过滤 |
| `github.com/coroot/coroot/utils` | 工具函数 |
| `github.com/stretchr/testify` | 断言库（项目已有依赖） |

---

## 维护说明

1. **新增 CheckId 时**：需同步更新用例1的 `SLOCheckIdConsistentWithAlertingRuleSource` 测试
2. **修改 MemoryGrowthPct 算法时**：需同步更新用例2的三个 MemoryGrowthPct 子测试
3. **修改 GlobMatch 实现时**：需同步更新用例3的 GlobMatch 系列子测试
4. **修改 RBAC 角色定义时**：需同步更新用例2的 RbacPermissions 子测试
5. **修改 FilterLabels 签名时**：需同步更新用例3的 PromFilterLabels 子测试
