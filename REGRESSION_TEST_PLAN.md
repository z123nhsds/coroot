# 回归测试计划

## 概述
本文档描述了为验证三个关键功能而设计的回归测试用例，这些测试将确保在 go vet 修复后，系统的核心功能仍然正常工作。

---

## 测试环境
- **技术栈**: Go 1.25, ClickHouse, eBPF, Vue
- **主要模块**: notifications, prom, rbac, stats, timeseries, utils, watchers

---

## 测试用例 1: Notifications RCA 摘要与 SLO 告警关联测试

### 测试目标
验证 go vet 修复后，notifications 模块中的 RCA 摘要能够正确关联 prom 模块的 SLO 告警。

### 前置条件
1. 一个配置了 SLO 的项目
2. 启用了 notifications 模块
3. 有一个可用的 prom 配置

### 测试步骤
1. 模拟一个 SLO 违规场景，产生告警
2. 生成一个 RCA 摘要
3. 触发 notifications 模块的 incident notification
4. 验证通知中包含正确的 RCA 摘要
5. 验证 RCA 摘要与 SLO 告警正确关联

### 预期结果
- 通知中包含正确的 RCASummary 和 RCARemediations
- RCASummary 正确关联到 SLO 告警
- 通知能正常发送到配置的通知渠道

### 涉及文件/模块
- `notifications/incidents.go`: 尤其是 `incidentDetails()` 函数
- `auditor/slo.go`: SLO 检查逻辑
- `model/application_incident.go`: 事件和 RCA 结构

---

## 测试用例 2: MCP 服务器日志聚类与 MemoryGrowthPct 部署追踪交互测试

### 测试目标
验证 MCP 服务器（基于 rbac + stats）的日志聚类功能不受 MemoryGrowthPct 部署追踪的影响。

### 前置条件
1. 配置了 MCP 服务器
2. 启用了 RBAC
3. 有一个带有内存使用历史的应用程序
4. 该应用有部署历史

### 测试步骤
1. 创建一个应用，模拟内存增长并满足 MemoryGrowthPct 的条件
2. 触发一次部署，记录 MemoryGrowthPct 值
3. 同时进行日志收集和聚类（通过 MCP 服务器）
4. 验证日志聚类结果正确性
5. 验证 MemoryGrowthPct 被正确计算和记录
6. 验证部署追踪与日志聚类之间没有干扰

### 预期结果
- MemoryGrowthPct 被正确计算
- 部署追踪正常工作
- 日志聚类不受影响，结果正确
- MCP 服务器能够正常返回查询结果

### 涉及文件/模块
- `auditor/memory.go`: `MemoryGrowthPct()` 函数
- `watchers/deployments.go`: 部署追踪逻辑
- `api/mcp.go`: MCP 服务器实现
- `rbac/`: RBAC 权限检查模块
- `stats/`: 统计模块

---

## 测试用例 3: Utils 模块修复对 Prometheus 依赖健康检查影响测试

### 测试目标
验证 utils 模块的修复不会破坏 watchers 对 Prometheus 依赖的健康检查功能。

### 前置条件
1. 配置了 Prometheus 集成
2. 有一个 watcher 在监视 Prometheus 依赖
3. utils 模块有修复

### 测试步骤
1. 在正常的 Prometheus 连接下，运行健康检查
2. 模拟 Prometheus 连接失败场景
3. 验证健康检查正确失败
4. 验证 utils 模块中的关键功能（如 URL 处理、JSON 处理等）正常工作
5. 验证 watchers 中的 PromQL 查询评估不受影响

### 预期结果
- Prometheus 健康检查能正常工作
- 网络超时和错误被正确处理
- PromQL 查询评估正常
- utils 模块修复不影响上述功能

### 涉及文件/模块
- `watchers/alerts.go`: `evaluatePromQLAlerts()` 和 Prometheus 检查
- `prom/client.go`: Prometheus 客户端实现
- `utils/`: utils 模块的关键文件

---

## 测试执行流程

### 第1阶段: 单元测试
1. 为每个测试用例编写单元测试
2. 运行单元测试并验证
3. 修复发现的问题

### 第2阶段: 集成测试
1. 在集成测试环境中运行完整测试
2. 验证模块间交互
3. 性能和稳定性测试

### 第3阶段: 回归测试
1. 在完整代码库上运行所有测试
2. 验证修复后没有引入新问题
3. 端到端功能验证

---

## 验收标准
- 所有三个测试用例的所有子测试必须通过
- 测试覆盖率至少达到 80%
- 没有新的 go vet 警告
- 没有性能退化

---

## 风险与缓解措施

### 风险1: 测试数据准备复杂
- **缓解**: 创建可重用的测试数据生成工具，模拟各种场景

### 风险2: 集成测试环境复杂
- **缓解**: 使用 Docker 容器提供可重复的测试环境

### 风险3: 修复影响其他模块
- **缓解**: 充分的代码审查，完整的回归测试套件
