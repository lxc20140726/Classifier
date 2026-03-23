package handler

import (
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/liqiye/classifier/internal/fs"
)

type FSHandler struct {
	fsAdapter fs.FSAdapter
}

func NewFSHandler(fsAdapter fs.FSAdapter) *FSHandler {
	return &FSHandler{fsAdapter: fsAdapter}
}

type dirEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// ListDirs returns immediate subdirectories of `path`.
// If path is empty or "/" it returns top-level filesystem roots (on Unix, just "/").
func (h *FSHandler) ListDirs(c *gin.Context) {
	rawPath := c.Query("path")
	if rawPath == "" {
		rawPath = "/"
	}

	// Normalise and clean to prevent path-traversal tricks.
	cleanPath := filepath.Clean(rawPath)
	if !filepath.IsAbs(cleanPath) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path must be absolute"})
		return
	}

	ctx := c.Request.Context()

	info, err := h.fsAdapter.Stat(ctx, cleanPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path not accessible: " + err.Error()})
		return
	}
	if !info.IsDir {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is not a directory"})
		return
	}

	entries, err := h.fsAdapter.ReadDir(ctx, cleanPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list directory"})
		return
	}

	dirs := make([]dirEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir {
			continue
		}
		// Skip hidden directories.
		if strings.HasPrefix(entry.Name, ".") {
			continue
		}
		dirs = append(dirs, dirEntry{
			Name: entry.Name,
			Path: filepath.Join(cleanPath, entry.Name),
		})
	}

	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].Name < dirs[j].Name
	})

	parent := ""
	if cleanPath != "/" {
		parent = filepath.Dir(cleanPath)
	}

	c.JSON(http.StatusOK, gin.H{
		"path":    cleanPath,
		"parent":  parent,
		"entries": dirs,
	})
}
