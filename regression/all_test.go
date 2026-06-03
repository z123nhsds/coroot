package regression

import (
	"testing"
)

// TestAllRegressionTests 运行所有回归测试
func TestAllRegressionTests(t *testing.T) {
	suite := NewTestSuite()

	// 添加所有回归测试
	suite.AddTest(NewRCASLOTest())
	suite.AddTest(NewMCPLogMemoryTest())
	suite.AddTest(NewUtilsWatchersTest())

	// 运行所有测试
	suite.RunAll(t)
}
