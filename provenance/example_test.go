package provenance

import (
	"testing"
)

func TestGenerateProvenance(t *testing.T) {
	// 创建一个临时文件作为测试二进制文件
	// 这里只是一个示例，实际使用时需要真实的二进制文件
	p, err := GenerateProvenance(
		"test-binary",
		"https://github.com/coroot/coroot",
		"abc123def",
		"main.go",
		"https://github.com/actions/runner",
		"12345",
	)
	if err != nil {
		t.Fatalf("failed to generate provenance: %v", err)
	}
	
	if p == nil {
		t.Fatal("provenance is nil")
	}
	
	if len(p.Subject) != 1 {
		t.Errorf("expected 1 subject, got %d", len(p.Subject))
	}
	
	t.Logf("Provenance generated successfully: %+v", p)
}
