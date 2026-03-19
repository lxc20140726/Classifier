package service

import (
	"path/filepath"
	"strings"
)

var imageExts = map[string]bool{
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

var videoExts = map[string]bool{
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

var mangaExts = map[string]bool{
	".cbz": true,
	".cbr": true,
	".cb7": true,
	".cbt": true,
}

var mangaKeywords = []string{"漫画", "comic", "manga"}

func Classify(folderName string, fileNames []string) string {
	folderNameLower := strings.ToLower(folderName)
	for _, keyword := range mangaKeywords {
		if strings.Contains(folderNameLower, strings.ToLower(keyword)) {
			return "manga"
		}
	}

	imageCount := 0
	videoCount := 0

	for _, fileName := range fileNames {
		ext := strings.ToLower(filepath.Ext(fileName))

		if mangaExts[ext] {
			return "manga"
		}

		if imageExts[ext] {
			imageCount++
		}

		if videoExts[ext] {
			videoCount++
		}
	}

	totalMedia := imageCount + videoCount
	if totalMedia == 0 {
		return "other"
	}

	imageRatio := float64(imageCount) / float64(totalMedia)
	videoRatio := float64(videoCount) / float64(totalMedia)

	if imageRatio >= 0.85 {
		return "photo"
	}

	if videoRatio >= 0.85 {
		return "video"
	}

	if imageRatio >= 0.15 && videoRatio >= 0.15 {
		return "mixed"
	}

	return "other"
}
