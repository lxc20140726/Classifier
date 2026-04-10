package service

import (
	"strconv"
	"strings"

	"github.com/liqiye/classifier/internal/repository"
)

const (
	workflowPathRefTypeScan   = "scan"
	workflowPathRefTypeOutput = "output"
	workflowPathRefTypeCustom = "custom"
)

type workflowNodePathOptions struct {
	DefaultType      string
	DefaultOutputKey string
	LegacyKeys       []string
}

func resolveWorkflowNodePath(config map[string]any, appConfig *repository.AppConfig, options workflowNodePathOptions) string {
	refType := strings.ToLower(strings.TrimSpace(stringConfig(config, "path_ref_type")))
	refKey := strings.TrimSpace(stringConfig(config, "path_ref_key"))
	suffix := normalizeWorkflowPath(strings.TrimSpace(stringConfig(config, "path_suffix")))
	legacyPath := firstLegacyNodePath(config, options.LegacyKeys...)

	if refType == "" {
		if legacyPath != "" {
			refType = workflowPathRefTypeCustom
			refKey = legacyPath
		} else if strings.TrimSpace(options.DefaultType) != "" {
			refType = strings.ToLower(strings.TrimSpace(options.DefaultType))
		}
	}

	var base string
	switch refType {
	case workflowPathRefTypeOutput:
		outputKey := refKey
		if outputKey == "" {
			outputKey = strings.TrimSpace(options.DefaultOutputKey)
		}
		base = resolveOutputDirByKey(appConfig, outputKey)
		if base == "" && looksLikePath(outputKey) {
			base = normalizeWorkflowPath(outputKey)
		}
	case workflowPathRefTypeScan:
		base = resolveScanDirByKey(appConfig, refKey)
		if base == "" && looksLikePath(refKey) {
			base = normalizeWorkflowPath(refKey)
		}
	default:
		base = normalizeWorkflowPath(refKey)
	}

	if base == "" {
		base = legacyPath
	}
	if base == "" {
		return normalizeWorkflowPath(suffix)
	}
	if suffix == "" {
		return normalizeWorkflowPath(base)
	}
	return joinWorkflowPath(base, suffix)
}

func firstLegacyNodePath(config map[string]any, keys ...string) string {
	for _, key := range keys {
		value := normalizeWorkflowPath(stringConfig(config, key))
		if value != "" {
			return value
		}
	}
	return ""
}

func resolveOutputDirByKey(appConfig *repository.AppConfig, key string) string {
	if appConfig == nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "video":
		return normalizeWorkflowPath(appConfig.OutputDirs.Video)
	case "manga":
		return normalizeWorkflowPath(appConfig.OutputDirs.Manga)
	case "photo":
		return normalizeWorkflowPath(appConfig.OutputDirs.Photo)
	case "other":
		return normalizeWorkflowPath(appConfig.OutputDirs.Other)
	case "mixed":
		return normalizeWorkflowPath(appConfig.OutputDirs.Mixed)
	default:
		return ""
	}
}

func resolveScanDirByKey(appConfig *repository.AppConfig, key string) string {
	if appConfig == nil || len(appConfig.ScanInputDirs) == 0 {
		return ""
	}
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return normalizeWorkflowPath(appConfig.ScanInputDirs[0])
	}
	index, err := strconv.Atoi(trimmed)
	if err == nil {
		if index >= 0 && index < len(appConfig.ScanInputDirs) {
			return normalizeWorkflowPath(appConfig.ScanInputDirs[index])
		}
		return ""
	}
	for _, item := range appConfig.ScanInputDirs {
		if normalizeWorkflowPath(item) == normalizeWorkflowPath(trimmed) {
			return normalizeWorkflowPath(item)
		}
	}
	return ""
}

func looksLikePath(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	return strings.Contains(trimmed, "/") || strings.Contains(trimmed, "\\") || strings.HasPrefix(trimmed, ".")
}
