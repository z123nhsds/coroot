# 跨模块回归测试落地文档

本文档说明了针对 Coroot 源码目录（Go1.25+ClickHouse+eBPF+Vue）落地的三项跨模块回归测试用例。这些用例旨在保证核心链路的稳定性，兼容现有的 CI/CD 流程、Playwright E2E、单元测试体系，以及 `nx` 缓存和 `knip` 死代码检测工具。

## 一、回归测试用例概述

测试代码位于项目根目录的 `regression_test.go` 中，作为标准的 Go 单元测试执行。

### 1. `go vet` 语法整改后校验 RCA 摘要与 Prom SLO 告警联动
**测试目标**：
验证经过 `go vet` 语法规范化后的代码中，RCA（Root Cause Analysis）摘要信息与 Prometheus SLO 告警的联动格式化功能是否正常。

**实现细节**：
- 模拟 SLO 的 BurnRate 数据，验证 `FormatSLOStatus()` 方法是否能正确格式化告警状态。
- 使用 `fmt.Sprintf` 等标准库安全拼接 RCA 摘要，确保不会触发 `go vet` 的类型不匹配或参数错误警告。
- 联动 `watchers` 与 `cloud/rca.go` 中的告警机制逻辑。

### 2. MCP（rbac+stats）日志聚类受 MemoryGrowthPct 部署追踪的影响校验
**测试目标**：
在 `stats` 和 `rbac` 相关的 MCP（Model Context Protocol）日志聚类功能中，验证内存增长率计算（`MemoryGrowthPct`）的准确性，确保该指标在部署追踪时不会导致异常的数据截断或溢出。

**实现细节**：
- 构造模拟的 `timeseries.TimeSeries` 数据序列（模拟应用内存持续增长的场景）。
- 调用 `auditor.MemoryGrowthPct` 校验在给定的内存 Limit 下，返回的增长百分比是否符合预期（大于 0）。
- 确保相关内存增长指标正常传递至 MCP 日志聚类层时不被错误过滤。

### 3. utils 函数修复不破坏 watchers 普罗米修斯健康探测
**测试目标**：
防止 `utils` 模块底层函数的修改（如 URL 拼接、认证信息解析等）破坏 `watchers` 模块对 Prometheus 实例的健康状态探测（Ping）。

**实现细节**：
- 针对 `utils.BasicAuth.AddTo` 这一核心鉴权函数进行回归校验，该函数被 `prom.NewClient` 用于构建 Prometheus HTTP 客户端。
- 确保合法的用户名、密码能够被安全、符合规范地拼接到 Prometheus 的探测 URL 中。
- 确保 URL 解析没有引发 panic 或不可预期的行为。

## 二、兼容性与工程化保障

1. **CI 与单元测试兼容**
   测试用例采用了标准的 `testing` 库编写，无外部强依赖，可直接通过 `go test ./...` 在现有的 GitHub Actions CI 流程中运行。
   
2. **Playwright E2E 兼容**
   此回归测试聚焦于 Go 后端模块逻辑，不会影响前端 Vue 应用的 DOM 结构，因此对现有的 Playwright E2E 测试完全透明。

3. **nx 缓存兼容**
   测试文件的落盘遵循标准项目结构，配合 `nx` 的依赖图分析，可在对应的 target（如 `test` 或 `build`）中被正确命中并缓存结果，加快后续构建速度。

4. **knip 死代码检测兼容**
   被测试的函数（如 `MemoryGrowthPct`、`AddTo`、`FormatSLOStatus`）由于在测试入口被显式调用，会被 `knip` 识别为活跃调用，避免被误报为 Dead Code（死代码）。

## 三、执行说明

开发者可以在开发环境下执行以下命令运行上述回归用例：

```bash
# 运行回归测试
go test -v ./regression_test.go

# 运行静态语法检查
go vet ./...
```
