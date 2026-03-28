package service

import (
	"path/filepath"
	"strings"
)

func normalizeWorkflowPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}

	cleaned := filepath.Clean(trimmed)
	if filepath.VolumeName(cleaned) != "" {
		return cleaned
	}

	return filepath.ToSlash(cleaned)
}

func joinWorkflowPath(base, name string) string {
	return normalizeWorkflowPath(filepath.Join(strings.TrimSpace(base), strings.TrimSpace(name)))
}
