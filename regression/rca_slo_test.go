package regression

import (
	"testing"

	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/notifications"
	"github.com/coroot/coroot/prom"
	"github.com/stretchr/testify/assert"
)

// RCASLOTest 测试 RCA 摘要与 Prometheus SLO 告警联动
type RCASLOTest struct {
	name string
}

func NewRCASLOTest() RegressionTest {
	return &RCASLOTest{
		name: "RCA_SLO_Integration",
	}
}

func (t *RCASLOTest) Name() string {
	return t.name
}

func (t *RCASLOTest) Setup(testing *testing.T) error {
	return nil
}

func (t *RCASLOTest) Cleanup(testing *testing.T) error {
	return nil
}

func (t *RCASLOTest) Run(testing *testing.T) error {
	// 测试场景 1: 创建一个有 SLO 配置的应用
	app := model.NewApplication(model.NewApplicationId("test-cluster", "default", model.ApplicationKindDeployment, "test-service"))

	// 添加 SLO 配置
	app.AvailabilitySLIs = []*model.SLI{
		{
			Config: &model.SLIConfig{
				Objective: 99.9,
			},
		},
	}

	// 创建一个事件（模拟部署或错误）
	testing.Run("RCA_Generation_with_SLO", func(t *testing.T) {
		// 创建一个世界模型
		w := &model.World{
			Applications: []*model.Application{app},
		}

		// 创建一个项目
		project := &model.Project{}

		// 运行审计
		auditor.Audit(w, project, nil, nil)

		// 验证 SLO 检查是否被创建
		assert.NotEmpty(t, app.Reports)
		foundSLO := false
		for _, report := range app.Reports {
			if report.Name == model.AuditReportSLO {
				foundSLO = true
				assert.NotEmpty(t, report.Checks)
			}
		}
		assert.True(t, foundSLO, "SLO report not found")
	})

	// 测试场景 2: 验证通知模块对 RCA 摘要的使用
	testing.Run("Notification_RCA_Summary", func(t *testing.T) {
		// 创建一个事件
		incident := &model.ApplicationIncident{
			RCA: &model.RCA{
				Status:         "OK",
				ShortSummary:   "High latency detected due to database connection issues",
				ImmediateFixes: []string{"Increase database connection pool size", "Check database health"},
			},
		}

		// 模拟通知详情生成
		details := notifications.IncidentDetails(nil, incident)

		// 验证 RCA 摘要被正确提取
		assert.NotNil(t, details)
		if details != nil {
			assert.Equal(t, "High latency detected due to database connection issues", details.RCASummary)
			assert.Len(t, details.RCARemediations, 2)
		}
	})

	// 测试场景 3: 验证 Prometheus 查询函数的正确性
	testing.Run("Prometheus_Query_Integration", func(t *testing.T) {
		// 这里可以添加 Prometheus 相关的测试
		// 例如测试 SLI 指标的计算
	})

	return nil
}

func TestRCASLOIntegration(t *testing.T) {
	suite := NewTestSuite()
	suite.AddTest(NewRCASLOTest())
	suite.RunAll(t)
}
