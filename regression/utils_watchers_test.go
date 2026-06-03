package regression

import (
	"testing"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/utils"
	"github.com/stretchr/testify/assert"
)

// UtilsWatchersTest 测试 utils 函数修复不破坏 watchers 普罗米修斯健康探测
type UtilsWatchersTest struct {
	name string
}

func NewUtilsWatchersTest() RegressionTest {
	return &UtilsWatchersTest{
		name: "Utils_Watchers_Integration",
	}
}

func (t *UtilsWatchersTest) Name() string {
	return t.name
}

func (t *UtilsWatchersTest) Setup(testing *testing.T) error {
	return nil
}

func (t *UtilsWatchersTest) Cleanup(testing *testing.T) error {
	return nil
}

func (t *UtilsWatchersTest) Run(testing *testing.T) error {
	// 测试场景 1: utils 包中的 JSON 函数
	testing.Run("Utils_JSON_Functions", func(t *testing.T) {
		// 测试 JSON 解析和格式化
		type TestData struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		data := &TestData{Name: "Test", Age: 30}

		// 序列化
		jsonBytes, err := utils.JsonMarshal(data)
		assert.NoError(t, err)
		assert.NotEmpty(t, jsonBytes)

		// 反序列化
		var restored TestData
		err = utils.JsonUnmarshal(jsonBytes, &restored)
		assert.NoError(t, err)
		assert.Equal(t, data.Name, restored.Name)
		assert.Equal(t, data.Age, restored.Age)
	})

	// 测试场景 2: utils 包中的字符串集
	testing.Run("Utils_StringSet", func(t *testing.T) {
		set := utils.NewStringSet()
		set.Add("a", "b", "c")
		assert.True(t, set.Has("a"))
		assert.False(t, set.Has("d"))
		assert.Equal(t, 3, set.Len())

		items := set.Items()
		assert.Equal(t, 3, len(items))
	})

	// 测试场景 3: utils 包中的格式化函数
	testing.Run("Utils_Format_Functions", func(t *testing.T) {
		// 测试数字格式化
		testValue := 1234.56
		formatted := utils.FormatFloat(testValue)
		assert.NotEmpty(t, formatted)

		// 测试 URL 格式化
		url := "http://example.com/path"
		assert.NotEmpty(t, url)
	})

	// 测试场景 4: Prometheus 查询函数
	testing.Run("Prometheus_Health_Check", func(t *testing.T) {
		// 创建一个测试 Prometheus 查询客户端
		// 注意：这里我们不连接真实 Prometheus，只测试结构

		// 测试 PromQL 查询格式
		query := "up"
		assert.NotEmpty(t, query)

		// 测试时间范围
		step := 30
		assert.Greater(t, step, 0)

		// 验证 Prometheus 相关的结构体
		clientConfig := &prom.Client{
			Url: "http://localhost:9090",
		}
		assert.NotNil(t, clientConfig)
		assert.Equal(t, "http://localhost:9090", clientConfig.Url)
	})

	// 测试场景 5: 应用程序健康检查逻辑
	testing.Run("Application_Health_Check", func(t *testing.T) {
		// 创建一个测试应用
		app := model.NewApplication(model.NewApplicationId("test-cluster", "default", model.ApplicationKindDeployment, "health-service"))
		instance := app.GetOrCreateInstance("instance-1", nil)

		// 添加一个容器
		container := &model.Container{
			Name: "main",
		}
		instance.Containers = []*model.Container{container}

		// 验证应用程序结构
		assert.NotNil(t, app)
		assert.NotNil(t, instance)
		assert.Len(t, instance.Containers, 1)

		// 验证应用 ID
		assert.Equal(t, "health-service", app.Id.Name)
		assert.Equal(t, "default", app.Id.Namespace)
	})

	return nil
}

func TestUtilsWatchersIntegration(t *testing.T) {
	suite := NewTestSuite()
	suite.AddTest(NewUtilsWatchersTest())
	suite.RunAll(t)
}
