# 跨模块回归测试落地文档

## 概述

本文档介绍了 Coroot 项目中新增的跨模块回归测试框架，以及如何使用和维护这些测试。这些回归测试覆盖了三个关键场景，确保代码变更不会破坏系统的核心功能。

## 目录结构

```
coroot/
├── regression/
│   ├── regression.go              # 测试框架核心
│   ├── all_test.go                # 所有测试的入口
│   ├── rca_slo_test.go            # 第 1 项测试：RCA 与 SLO 告警联动
│   ├── mcp_log_memory_test.go     # 第 2 项测试：MCP 与 MemoryGrowthPct
│   └── utils_watchers_test.go     # 第 3 项测试：utils 与 watchers
└── Makefile                       # 更新的构建文件
```

## 测试场景说明

### 1. RCA 摘要与 Prometheus SLO 告警联动测试

**文件**: `regression/rca_slo_test.go`

**目标**: 验证 go vet 语法整改后，RCA（根本原因分析）摘要能正确与 Prometheus SLO（服务级别目标）告警关联。

**测试内容**:
- SLO 配置和检查生成
- RCA 摘要在通知系统中的使用
- Prometheus 查询功能的集成

### 2. MCP (RBAC + Stats) 与 MemoryGrowthPct 部署追踪测试

**文件**: `regression/mcp_log_memory_test.go`

**目标**: 验证 MCP（模型上下文协议）中的 RBAC（基于角色的访问控制）和统计功能，在 MemoryGrowthPct（内存增长百分比）部署追踪过程中对日志聚类的影响。

**测试内容**:
- RBAC 权限检查
- MCP 调用统计
- MemoryGrowthPct 计算与审计
- 日志模式聚类功能

### 3. Utils 函数与 Watchers 普罗米修斯健康探测测试

**文件**: `regression/utils_watchers_test.go`

**目标**: 验证 utils 包中的函数修复不会破坏 watchers 模块的 Prometheus 健康探测功能。

**测试内容**:
- utils 包核心函数（JSON、字符串集、格式化）
- Prometheus 查询客户端
- 应用健康检查逻辑

## 运行测试

### 运行所有回归测试

```bash
make regression-test
```

### 运行单个测试场景

```bash
# 运行 RCA 与 SLO 告警联动测试
make regression-test-rca-slo

# 运行 MCP 与 MemoryGrowthPct 测试
make regression-test-mcp-log-memory

# 运行 utils 与 watchers 测试
make regression-test-utils-watchers
```

### 直接使用 Go 命令

```bash
# 运行所有测试
go test -v ./regression -run TestAllRegressionTests

# 运行特定测试
go test -v ./regression -run TestRCASLOIntegration
```

## 与现有 CI 集成

这些回归测试已经与现有的 CI 系统兼容：

1. **与 Go 测试兼容**: 遵循标准的 Go 测试格式，可以通过 `go test` 直接运行
2. **与现有 Makefile 集成**: 在 Makefile 中新增了专门的回归测试目标
3. **与 Playwright E2E 测试**: 回归测试专注于后端功能，不与前端 E2E 测试冲突
4. **与 nx 缓存**: 由于使用了标准 Go 工具链，可以正常使用缓存
5. **与 knip 死代码检测**: 测试使用了实际的生产代码，不会被标记为死代码

### 在 GitHub Actions 中使用

可以在 `.github/workflows/ci.yml` 中添加回归测试步骤：

```yaml
- name: Run regression tests
  run: make regression-test
```

## 测试框架扩展

### 添加新的回归测试

1. 创建新的测试文件，例如 `regression/new_feature_test.go`
2. 实现 `RegressionTest` 接口：
   ```go
   type NewFeatureTest struct {
       name string
   }
   
   func (t *NewFeatureTest) Name() string { return t.name }
   func (t *NewFeatureTest) Setup(testing *testing.T) error { /* ... */ }
   func (t *NewFeatureTest) Run(testing *testing.T) error { /* ... */ }
   func (t *NewFeatureTest) Cleanup(testing *testing.T) error { /* ... */ }
   ```
3. 在 `all_test.go` 中注册新测试
4. 在 `Makefile` 中添加运行命令（可选）

### 最佳实践

- 每个测试场景独立，不依赖其他测试
- 使用 `Setup` 和 `Cleanup` 管理测试资源
- 测试应该快速运行，避免长时间操作
- 使用模拟（mocking）而不是真实外部服务
- 保持测试代码清晰易读，有良好的注释

## 维护指南

### 代码变更后更新测试

当修改以下模块时，请确保相应的回归测试仍然通过：

- **RCA 或 SLO 相关**: 更新 `rca_slo_test.go`
- **MCP、RBAC、Stats、内存分析或日志**: 更新 `mcp_log_memory_test.go`
- **Utils 或 Watchers**: 更新 `utils_watchers_test.go`

### 失败调试

1. 检查测试输出中的错误信息
2. 使用 `-v` 标志获取详细日志
3. 单独运行失败的测试进行调试
4. 检查是否引入了破坏性变更

## 与 go vet 配合使用

在运行回归测试前，确保通过 go vet 检查：

```bash
make go-vet
make regression-test
```

## 总结

这套回归测试框架提供了：
- 跨模块功能的集成测试
- 与现有工具链的良好兼容
- 清晰的扩展和维护指南
- 快速验证代码变更的能力

通过定期运行这些测试，可以有效防止代码回归，确保系统的稳定性和可靠性。
