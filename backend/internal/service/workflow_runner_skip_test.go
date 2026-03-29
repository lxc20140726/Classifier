package service

import (
	"testing"

	"github.com/liqiye/classifier/internal/repository"
)

func TestShouldSkipNodeWithRequiredListInput(t *testing.T) {
	t.Parallel()

	node := repository.WorkflowGraphNode{
		ID:   "compress",
		Type: "compress-node",
		Inputs: map[string]repository.NodeInputSpec{
			"items": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "router", SourcePort: "manga"}},
		},
	}
	schema := NodeSchema{
		Inputs: []PortDef{
			{Name: "items", Type: PortTypeProcessingItemList, Required: true},
		},
	}

	tests := []struct {
		name   string
		inputs map[string]*TypedValue
		want   bool
	}{
		{
			name: "missing input value should skip",
			inputs: map[string]*TypedValue{
				"items": nil,
			},
			want: true,
		},
		{
			name: "empty list should skip",
			inputs: map[string]*TypedValue{
				"items": {Type: PortTypeProcessingItemList, Value: []ProcessingItem{}},
			},
			want: true,
		},
		{
			name: "non-empty list should execute",
			inputs: map[string]*TypedValue{
				"items": {Type: PortTypeProcessingItemList, Value: []ProcessingItem{{SourcePath: "/source/a"}}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := shouldSkipNode(node, tt.inputs, schema)
			if got != tt.want {
				t.Fatalf("shouldSkipNode() = %v, want %v", got, tt.want)
			}
		})
	}
}

