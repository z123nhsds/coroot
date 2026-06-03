package regression

import (
	"testing"
)

// RegressionTest 定义了一个回归测试接口
type RegressionTest interface {
	Name() string
	Setup(t *testing.T) error
	Run(t *testing.T) error
	Cleanup(t *testing.T) error
}

// TestSuite 管理多个回归测试
type TestSuite struct {
	tests []RegressionTest
}

func NewTestSuite() *TestSuite {
	return &TestSuite{}
}

func (s *TestSuite) AddTest(test RegressionTest) {
	s.tests = append(s.tests, test)
}

func (s *TestSuite) RunAll(t *testing.T) {
	for _, test := range s.tests {
		t.Run(test.Name(), func(t *testing.T) {
			if err := test.Setup(t); err != nil {
				t.Fatalf("setup failed: %v", err)
			}
			defer func() {
				if err := test.Cleanup(t); err != nil {
					t.Logf("cleanup warning: %v", err)
				}
			}()
			if err := test.Run(t); err != nil {
				t.Fatalf("test failed: %v", err)
			}
		})
	}
}
