package service

import (
	"cloudbox/internal/config"
	"testing"
)

func TestParseAIResponse_SummaryAndTags(t *testing.T) {
	input := "摘要：这是一份关于数据分析的文档，包含了Python和机器学习的相关内容。\n标签：数据分析,Python,机器学习,可视化,报告"
	summary, tags := parseAIResponse(input)
	if summary != "这是一份关于数据分析的文档，包含了Python和机器学习的相关内容。" {
		t.Errorf("summary = %q", summary)
	}
	if len(tags) != 5 || tags[0] != "数据分析" {
		t.Errorf("tags = %v", tags)
	}
}

func TestParseAIResponse_ParseFailure(t *testing.T) {
	input := "This is just some random text without the expected format"
	summary, tags := parseAIResponse(input)
	if summary != input {
		t.Errorf("expected raw input as summary on parse failure, got %q", summary)
	}
	if len(tags) != 0 {
		t.Errorf("expected no tags on parse failure, got %v", tags)
	}
}

func TestParseAIResponse_EmptyTags(t *testing.T) {
	input := "摘要：测试摘要\n标签："
	summary, tags := parseAIResponse(input)
	if summary != "测试摘要" {
		t.Errorf("summary = %q", summary)
	}
	if len(tags) != 0 {
		t.Errorf("expected no tags when tags section is empty, got %v", tags)
	}
}

func TestShouldAutoProcess(t *testing.T) {
	cfg := config.AIConfig{AutoDocument: true, AutoImage: false, AutoVideo: false}

	tests := []struct {
		mime     string
		expected bool
	}{
		{"text/plain", true},
		{"application/pdf", true},
		{"image/jpeg", false},
		{"video/mp4", false},
		{"application/octet-stream", false},
	}
	for _, tt := range tests {
		got := shouldAutoProcess(tt.mime, cfg)
		if got != tt.expected {
			t.Errorf("shouldAutoProcess(%q) = %v, want %v", tt.mime, got, tt.expected)
		}
	}
}

func TestIsDocumentType(t *testing.T) {
	tests := []struct {
		mime     string
		expected bool
	}{
		{"text/plain", true},
		{"text/markdown", true},
		{"application/pdf", true},
		{"image/jpeg", false},
		{"video/mp4", false},
	}
	for _, tt := range tests {
		got := isDocumentType(tt.mime)
		if got != tt.expected {
			t.Errorf("isDocumentType(%q) = %v, want %v", tt.mime, got, tt.expected)
		}
	}
}
