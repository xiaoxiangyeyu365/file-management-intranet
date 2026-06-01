package handler

import (
	"cloudbox/internal/model"
	"os"
	"testing"
	"time"
)

func TestSplitParent(t *testing.T) {
	tests := []struct {
		input      string
		parentPath string
		name       string
	}{
		{"", "", ""},
		{"/docs/report.pdf", "/docs", "report.pdf"},
		{"/report.pdf", "", "report.pdf"},
		{"report.pdf", "", "report.pdf"},
		{"/a/b/c", "/a/b", "c"},
	}
	for _, tt := range tests {
		p, n := splitParent(tt.input)
		if p != tt.parentPath || n != tt.name {
			t.Errorf("splitParent(%q) = (%q, %q), want (%q, %q)", tt.input, p, n, tt.parentPath, tt.name)
		}
	}
}

func TestCloudFileInfo(t *testing.T) {
	f := &model.File{ID: 1, Name: "test.txt", IsFolder: false, UpdatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Physical: &model.PhysicalFile{Size: 1024}}
	fi := &cloudFileInfo{file: f}

	if fi.Name() != "test.txt" {
		t.Errorf("Name() = %q", fi.Name())
	}
	if fi.Size() != 1024 {
		t.Errorf("Size() = %d", fi.Size())
	}
	if fi.IsDir() {
		t.Error("IsDir() should be false")
	}
	if fi.Mode() != 0644 {
		t.Errorf("Mode() = %o", fi.Mode())
	}

	dir := &model.File{ID: 2, Name: "docs", IsFolder: true}
	di := &cloudFileInfo{file: dir}
	if !di.IsDir() {
		t.Error("IsDir() should be true for folder")
	}
	if di.Mode()&os.ModeDir == 0 {
		t.Error("Mode() should have ModeDir bit")
	}
}
