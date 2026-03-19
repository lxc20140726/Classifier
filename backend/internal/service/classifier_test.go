package service

import "testing"

func TestClassify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		folderName string
		fileNames  []string
		want       string
	}{
		{
			name:       "manga keyword in folder name",
			folderName: "My Comic Collection",
			fileNames:  []string{"image.jpg", "video.mp4"},
			want:       "manga",
		},
		{
			name:       "manga extension in files",
			folderName: "regular folder",
			fileNames:  []string{"chapter01.cbz", "cover.jpg"},
			want:       "manga",
		},
		{
			name:       "pure photo folder",
			folderName: "vacation",
			fileNames:  []string{"a.jpg", "b.png", "c.heic", "d.webp"},
			want:       "photo",
		},
		{
			name:       "pure video folder",
			folderName: "movies",
			fileNames:  []string{"a.mp4", "b.mkv", "c.ts"},
			want:       "video",
		},
		{
			name:       "mixed folder",
			folderName: "events",
			fileNames:  []string{"a.jpg", "b.png", "c.mp4", "d.mkv"},
			want:       "mixed",
		},
		{
			name:       "no media files",
			folderName: "docs",
			fileNames:  []string{"readme.txt", "notes.md"},
			want:       "other",
		},
		{
			name:       "exact 85 percent photo threshold",
			folderName: "mostly photos",
			fileNames: []string{
				"1.jpg", "2.jpg", "3.jpg", "4.jpg", "5.jpg", "6.jpg", "7.jpg", "8.jpg", "9.jpg",
				"10.jpg", "11.jpg", "12.jpg", "13.jpg", "14.jpg", "15.jpg", "16.jpg", "17.jpg",
				"a.mp4", "b.mp4", "c.mp4",
			},
			want: "photo",
		},
		{
			name:       "exact 85 percent video threshold",
			folderName: "mostly videos",
			fileNames: []string{
				"1.mp4", "2.mp4", "3.mp4", "4.mp4", "5.mp4", "6.mp4", "7.mp4", "8.mp4", "9.mp4",
				"10.mp4", "11.mp4", "12.mp4", "13.mp4", "14.mp4", "15.mp4", "16.mp4", "17.mp4",
				"a.jpg", "b.jpg", "c.jpg",
			},
			want: "video",
		},
		{
			name:       "below threshold fallback",
			folderName: "unsupported media-looking files",
			fileNames:  []string{"photo.jfif", "movie.mpeg", "archive.rar"},
			want:       "other",
		},
		{
			name:       "case insensitive extensions",
			folderName: "caps",
			fileNames:  []string{"A.JPEG", "B.MP4"},
			want:       "mixed",
		},
		{
			name:       "manga wins before ratio logic",
			folderName: "ratio would be video",
			fileNames:  []string{"chapter.cbr", "a.mp4", "b.mp4", "c.mp4", "d.mp4"},
			want:       "manga",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := Classify(tc.folderName, tc.fileNames)
			if got != tc.want {
				t.Fatalf("Classify(%q, %v) = %q, want %q", tc.folderName, tc.fileNames, got, tc.want)
			}
		})
	}
}
