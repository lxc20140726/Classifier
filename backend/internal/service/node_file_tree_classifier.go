package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

const fileTreeClassifierExecutorType = "file-tree-classifier"

type treeClassifierRule struct {
	Condition  string  `json:"condition"`
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
}

type fileTreeClassifierConfig struct {
	Rules []treeClassifierRule `json:"rules"`
}

var defaultTreeRules = []treeClassifierRule{
	{Condition: "has_ext(.cbz|.cbr|.cb7|.cbt)", Category: "manga", Confidence: 0.95},
	{Condition: "has_video_and_subtitle", Category: "video", Confidence: 0.90},
	{Condition: "video_ratio_with_cover", Category: "video", Confidence: 0.88},
	{Condition: "flat_images_no_subdir", Category: "photo", Confidence: 0.85},
}

var videoExtsSet = map[string]bool{
	".mp4":  true,
	".mkv":  true,
	".avi":  true,
	".mov":  true,
	".wmv":  true,
	".flv":  true,
	".m4v":  true,
	".ts":   true,
	".rmvb": true,
	".rm":   true,
	".webm": true,
	".3gp":  true,
}

var subtitleExtsSet = map[string]bool{
	".srt": true,
	".ass": true,
	".sub": true,
	".ssa": true,
}

var imageExtsSet = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
	".bmp":  true,
	".tiff": true,
	".tif":  true,
	".heic": true,
	".heif": true,
	".avif": true,
	".raw":  true,
}

type fileTreeClassifierNodeExecutor struct{}

func newFileTreeClassifierExecutor() *fileTreeClassifierNodeExecutor {
	return &fileTreeClassifierNodeExecutor{}
}

func NewFileTreeClassifierExecutor() WorkflowNodeExecutor {
	return newFileTreeClassifierExecutor()
}

func (e *fileTreeClassifierNodeExecutor) Type() string {
	return fileTreeClassifierExecutorType
}

func (e *fileTreeClassifierNodeExecutor) Schema() NodeSchema {
	return NodeSchema{
		Type:        fileTreeClassifierExecutorType,
		Label:       "文件树分类器",
		Description: "根据 FolderTree 文件结构进行规则分类",
		InputPorts: []NodeSchemaPort{{
			Name:        "folder",
			Description: "FOLDER_TREE 输入目录树",
			Required:    true,
		}},
		OutputPorts: []NodeSchemaPort{{
			Name:        "signal",
			Description: "CLASSIFICATION_SIGNAL 命中时输出分类信号",
		}, {
			Name:        "folder",
			Description: "FOLDER_TREE 未命中时透传目录树",
		}},
	}
}

func (e *fileTreeClassifierNodeExecutor) Execute(_ context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	var folder *FolderTree

	rawFolder, ok := input.Inputs["folder"]
	if ok {
		switch value := rawFolder.(type) {
		case nil:
			folder = nil
		case FolderTree:
			copied := value
			folder = &copied
		case *FolderTree:
			folder = value
		}
	}

	if folder == nil {
		return NodeExecutionOutput{Outputs: []any{nil, nil}, Status: ExecutionSuccess}, nil
	}

	rules := parseTreeRules(input.Node.Config)
	for _, rule := range rules {
		if !evaluateRule(*folder, rule) {
			continue
		}

		return NodeExecutionOutput{Outputs: []any{ClassificationSignal{
			Category:   rule.Category,
			Confidence: rule.Confidence,
			Reason:     "file-tree:" + rule.Condition,
		}, nil}, Status: ExecutionSuccess}, nil
	}

	return NodeExecutionOutput{Outputs: []any{nil, *folder}, Status: ExecutionSuccess}, nil
}

func (e *fileTreeClassifierNodeExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, fmt.Errorf("%s: Resume not supported", e.Type())
}

func (e *fileTreeClassifierNodeExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}

func parseTreeRules(config map[string]any) []treeClassifierRule {
	rawRules, ok := config["rules"]
	if !ok || rawRules == nil {
		return append([]treeClassifierRule(nil), defaultTreeRules...)
	}

	rules, ok := decodeTreeRules(rawRules)
	if !ok || len(rules) == 0 {
		return append([]treeClassifierRule(nil), defaultTreeRules...)
	}

	return rules
}

func decodeTreeRules(raw any) ([]treeClassifierRule, bool) {
	switch value := raw.(type) {
	case []treeClassifierRule:
		return append([]treeClassifierRule(nil), value...), true
	case []any:
		out := make([]treeClassifierRule, 0, len(value))
		for _, item := range value {
			ruleMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			rule := treeClassifierRule{
				Condition: strings.TrimSpace(anyString(ruleMap["condition"])),
				Category:  strings.TrimSpace(anyString(ruleMap["category"])),
			}
			rawConfidence, ok := ruleMap["confidence"]
			if ok {
				switch conf := rawConfidence.(type) {
				case float64:
					rule.Confidence = conf
				case float32:
					rule.Confidence = float64(conf)
				case int:
					rule.Confidence = float64(conf)
				case int64:
					rule.Confidence = float64(conf)
				}
			}
			if rule.Condition == "" || rule.Category == "" {
				continue
			}
			out = append(out, rule)
		}
		return out, true
	default:
		return nil, false
	}
}

func evaluateRule(tree FolderTree, rule treeClassifierRule) bool {
	condition := strings.TrimSpace(rule.Condition)
	if strings.HasPrefix(condition, "has_ext(") && strings.HasSuffix(condition, ")") {
		inside := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(condition, "has_ext("), ")"))
		if inside == "" {
			return false
		}
		rawExts := strings.Split(inside, "|")
		exts := make([]string, 0, len(rawExts))
		for _, rawExt := range rawExts {
			normalized := normalizeExt(rawExt)
			if normalized == "" {
				continue
			}
			exts = append(exts, normalized)
		}
		if len(exts) == 0 {
			return false
		}
		return evalHasExt(tree, exts)
	}

	switch condition {
	case "has_video_and_subtitle":
		hasVideo := false
		hasSubtitle := false
		for _, f := range tree.Files {
			ext := fileExt(f)
			if videoExtsSet[ext] {
				hasVideo = true
			}
			if subtitleExtsSet[ext] {
				hasSubtitle = true
			}
			if hasVideo && hasSubtitle {
				return true
			}
		}
		return false
	case "flat_images_no_subdir":
		if len(tree.Files) == 0 || len(tree.Subdirs) > 0 {
			return false
		}
		for _, f := range tree.Files {
			if !imageExtsSet[fileExt(f)] {
				return false
			}
		}
		return true
	case "video_ratio_with_cover":
		if len(tree.Files) == 0 {
			return false
		}

		videoCount := 0
		imageCount := 0
		for _, f := range tree.Files {
			ext := fileExt(f)
			if videoExtsSet[ext] {
				videoCount++
			}
			if imageExtsSet[ext] {
				imageCount++
			}
		}

		nonImageCount := len(tree.Files) - imageCount
		if nonImageCount <= 0 {
			return false
		}

		return videoCount*100 >= nonImageCount*80 && imageCount >= 1 && imageCount <= 3
	default:
		return false
	}
}

func evalHasExt(tree FolderTree, exts []string) bool {
	extSet := map[string]bool{}
	for _, e := range exts {
		normalized := normalizeExt(e)
		if normalized == "" {
			continue
		}
		extSet[normalized] = true
		extSet[strings.TrimPrefix(normalized, ".")] = true
	}

	for _, f := range tree.Files {
		normalizedWithDot := fileExt(f)
		normalizedNoDot := strings.TrimPrefix(normalizedWithDot, ".")
		if extSet[normalizedWithDot] || extSet[normalizedNoDot] {
			return true
		}
	}

	return false
}

func normalizeExt(ext string) string {
	trimmed := strings.TrimSpace(strings.ToLower(ext))
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, ".") {
		return trimmed
	}
	return "." + trimmed
}

func fileExt(file FileEntry) string {
	ext := normalizeExt(file.Ext)
	if ext != "" {
		return ext
	}
	return normalizeExt(filepath.Ext(file.Name))
}
