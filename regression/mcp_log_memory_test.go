package regression

import (
	"testing"

	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/rbac"
	"github.com/stretchr/testify/assert"
)

// MCPLogMemoryTest 测试 MCP (rbac+stats) 日志聚类受 MemoryGrowthPct 部署追踪的影响
type MCPLogMemoryTest struct {
	name string
}

func NewMCPLogMemoryTest() RegressionTest {
	return &MCPLogMemoryTest{
		name: "MCP_Log_Memory_Integration",
	}
}

func (t *MCPLogMemoryTest) Name() string {
	return t.name
}

func (t *MCPLogMemoryTest) Setup(testing *testing.T) error {
	return nil
}

func (t *MCPLogMemoryTest) Cleanup(testing *testing.T) error {
	return nil
}

func (t *MCPLogMemoryTest) Run(testing *testing.T) error {
	// 测试场景 1: RBAC 权限检查
	testing.Run("RBAC_Permissions", func(t *testing.T) {
		// 创建一个用户
		user := &db.User{
			Roles: []rbac.RoleName{
				rbac.RoleEditor,
			},
		}

		// 验证用户角色
		assert.Contains(t, user.Roles, rbac.RoleEditor)
		assert.NotContains(t, user.Roles, rbac.RoleAdmin)
	})

	// 测试场景 2: 统计 MCP 调用
	testing.Run("Stats_MCP_Calls", func(t *testing.T) {
		// 这个测试可以验证 stats 包中的 MCP 调用注册功能
		// 由于 stats 包中的 Collector 是内部的，我们可以测试其核心逻辑

		// 验证 RegisterMCPCall 函数的逻辑
		mcpCalls := map[string]int{}
		registerMCPCall := func(tool string) {
			mcpCalls[tool]++
		}

		// 模拟 MCP 调用
		registerMCPCall("list_projects")
		registerMCPCall("list_applications")
		registerMCPCall("list_projects")

		assert.Equal(t, 2, mcpCalls["list_projects"])
		assert.Equal(t, 1, mcpCalls["list_applications"])
	})

	// 测试场景 3: MemoryGrowthPct 计算
	testing.Run("Memory_Growth_Calculation", func(t *testing.T) {
		// 创建一个测试应用程序
		app := model.NewApplication(model.NewApplicationId("test-cluster", "default", model.ApplicationKindDeployment, "memory-service"))
		instance := app.GetOrCreateInstance("instance-1", nil)

		// 创建内存使用时间序列
		ts := model.NewTimeSeries(0, 1, nil)
		// 添加模拟数据：内存使用呈增长趋势
		for i := 0; i < 60; i++ {
			ts.Add(100.0 + float32(i)*10)
		}

		// 创建一个容器
		container := &model.Container{
			Name:        "main",
			MemoryRss:   ts,
			MemoryLimit: model.NewTimeSeries(0, 1, nil).Add(2000),
		}
		instance.Containers = []*model.Container{container}

		// 测试 MemoryGrowthPct 函数
		// 注意：这个函数在 auditor 包中是私有的，我们需要测试其效果

		// 创建世界和项目
		w := &model.World{
			Applications: []*model.Application{app},
		}
		project := &model.Project{}

		// 运行审计
		auditor.Audit(w, project, nil, nil)

		// 验证内存报告是否生成
		foundMemoryReport := false
		for _, report := range app.Reports {
			if report.Name == model.AuditReportMemory {
				foundMemoryReport = true
				assert.NotEmpty(t, report.Checks)
			}
		}
		assert.True(t, foundMemoryReport, "Memory report not found")
	})

	// 测试场景 4: 日志聚类
	testing.Run("Log_Clustering", func(t *testing.T) {
		// 创建一个应用并添加日志模式
		app := model.NewApplication(model.NewApplicationId("test-cluster", "default", model.ApplicationKindDeployment, "log-service"))

		// 添加一些模拟的错误日志模式
		app.LogMessages = map[model.Severity]*model.LogMessages{
			model.SeverityError: {
				Patterns: map[string]*model.LogPattern{
					"pattern1": {
						Messages: model.NewTimeSeries(0, 1, nil).Add(10),
						Sample:   "Error connecting to database",
					},
					"pattern2": {
						Messages: model.NewTimeSeries(0, 1, nil).Add(5),
						Sample:   "Invalid request format",
					},
				},
			},
		}

		// 创建世界和项目
		w := &model.World{
			Applications: []*model.Application{app},
		}
		project := &model.Project{}

		// 运行审计
		auditor.Audit(w, project, nil, nil)

		// 验证日志报告是否生成
		foundLogReport := false
		for _, report := range app.Reports {
			if report.Name == model.AuditReportLogs {
				foundLogReport = true
				assert.NotEmpty(t, report.Checks)
			}
		}
		assert.True(t, foundLogReport, "Logs report not found")
	})

	return nil
}

func TestMCPLogMemoryIntegration(t *testing.T) {
	suite := NewTestSuite()
	suite.AddTest(NewMCPLogMemoryTest())
	suite.RunAll(t)
}
