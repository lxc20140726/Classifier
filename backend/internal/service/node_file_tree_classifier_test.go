package service

import (
	"context"
	"testing"
)

func TestFileTreeClassifier(t *testing.T) {
	t.Parallel()

	executor := newFileTreeClassifierExecutor()

	tests := []struct {
		name            string
		folderInput     any
		wantStatus      ExecutionStatus
		wantSignal      *ClassificationSignal
		wantPassThrough bool
		wantNilOutputs  bool
	}{
		{
			name: "cbz_file",
			folderInput: FolderTree{
				Files: []FileEntry{{Name: "archive.cbz", Ext: "cbz"}},
			},
			wantStatus: ExecutionSuccess,
			wantSignal: &ClassificationSignal{Category: "manga", Confidence: 0.95},
		},
		{
			name: "video_and_subtitle",
			folderInput: FolderTree{
				Files: []FileEntry{{Name: "movie.mkv", Ext: "mkv"}, {Name: "movie.srt", Ext: "srt"}},
			},
			wantStatus: ExecutionSuccess,
			wantSignal: &ClassificationSignal{Category: "video", Confidence: 0.90},
		},
		{
			name: "video_with_cover",
			folderInput: FolderTree{
				Files: []FileEntry{{Name: "ep1.mkv", Ext: "mkv"}, {Name: "ep2.mkv", Ext: "mkv"}, {Name: "ep3.mkv", Ext: "mkv"}, {Name: "cover.jpg", Ext: "jpg"}},
			},
			wantStatus: ExecutionSuccess,
			wantSignal: &ClassificationSignal{Category: "video", Confidence: 0.88},
		},
		{
			name: "flat_images",
			folderInput: FolderTree{
				Files:   []FileEntry{{Name: "1.jpg", Ext: "jpg"}, {Name: "2.jpg", Ext: "jpg"}, {Name: "3.jpg", Ext: "jpg"}, {Name: "4.jpg", Ext: "jpg"}, {Name: "5.jpg", Ext: "jpg"}},
				Subdirs: nil,
			},
			wantStatus: ExecutionSuccess,
			wantSignal: &ClassificationSignal{Category: "photo", Confidence: 0.85},
		},
		{
			name: "no_match",
			folderInput: FolderTree{
				Files: []FileEntry{{Name: "doc.pdf", Ext: "pdf"}, {Name: "data.xlsx", Ext: "xlsx"}},
			},
			wantStatus:      ExecutionSuccess,
			wantPassThrough: true,
		},
		{
			name:           "nil_input",
			folderInput:    nil,
			wantStatus:     ExecutionSuccess,
			wantNilOutputs: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			out, err := executor.Execute(context.Background(), NodeExecutionInput{
				Inputs: map[string]any{"folder": tc.folderInput},
			})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if out.Status != tc.wantStatus {
				t.Fatalf("status = %q, want %q", out.Status, tc.wantStatus)
			}
			if len(out.Outputs) != 2 {
				t.Fatalf("len(outputs) = %d, want 2", len(out.Outputs))
			}

			if tc.wantNilOutputs {
				if out.Outputs[0] != nil || out.Outputs[1] != nil {
					t.Fatalf("outputs = %#v, want [nil nil]", out.Outputs)
				}
				return
			}

			if tc.wantSignal != nil {
				signal, ok := out.Outputs[0].(ClassificationSignal)
				if !ok {
					t.Fatalf("port0 type = %T, want ClassificationSignal", out.Outputs[0])
				}
				if signal.Category != tc.wantSignal.Category {
					t.Fatalf("signal.Category = %q, want %q", signal.Category, tc.wantSignal.Category)
				}
				if signal.Confidence != tc.wantSignal.Confidence {
					t.Fatalf("signal.Confidence = %v, want %v", signal.Confidence, tc.wantSignal.Confidence)
				}
				if signal.Reason == "" {
					t.Fatalf("signal.Reason = empty, want non-empty")
				}
				if out.Outputs[1] != nil {
					t.Fatalf("port1 = %#v, want nil", out.Outputs[1])
				}
				return
			}

			if !tc.wantPassThrough {
				t.Fatalf("invalid test case: signal=nil and pass-through disabled")
			}

			if out.Outputs[0] != nil {
				t.Fatalf("port0 = %#v, want nil", out.Outputs[0])
			}
			folder, ok := out.Outputs[1].(FolderTree)
			if !ok {
				t.Fatalf("port1 type = %T, want FolderTree", out.Outputs[1])
			}
			if len(folder.Files) != 2 || folder.Files[0].Name != "doc.pdf" || folder.Files[1].Name != "data.xlsx" {
				t.Fatalf("pass-through folder files = %#v, want original file list", folder.Files)
			}
		})
	}
}
