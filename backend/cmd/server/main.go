package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/liqiye/classifier/internal/config"
	"github.com/liqiye/classifier/internal/db"
	internalfs "github.com/liqiye/classifier/internal/fs"
	"github.com/liqiye/classifier/internal/handler"
	"github.com/liqiye/classifier/internal/repository"
	"github.com/liqiye/classifier/internal/service"
	"github.com/liqiye/classifier/internal/sse"
)

//go:embed web/dist
var webDist embed.FS

func main() {
	cfg := config.Load()

	dataDir := cfg.ConfigDir
	dbPath := filepath.Join(dataDir, "classifier.db")

	sqlDB, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer sqlDB.Close()

	folderRepo := repository.NewFolderRepository(sqlDB)
	jobRepo := repository.NewJobRepository(sqlDB)
	snapshotRepo := repository.NewSnapshotRepository(sqlDB)
	configRepo := repository.NewConfigRepository(sqlDB)
	auditRepo := repository.NewAuditRepository(sqlDB)

	fsAdapter := internalfs.NewOSAdapter()
	broker := sse.NewBroker()

	auditSvc := service.NewAuditService(auditRepo)
	snapshotSvc := service.NewSnapshotService(fsAdapter, snapshotRepo, folderRepo)
	scannerSvc := service.NewScannerService(fsAdapter, folderRepo)
	moveSvc := service.NewMoveService(fsAdapter, jobRepo, folderRepo, snapshotSvc, auditSvc, broker)

	folderHandler := handler.NewFolderHandler(folderRepo, scannerSvc, fsAdapter, cfg.SourceDir, cfg.DeleteStagingDir)
	moveHandler := handler.NewMoveHandler(moveSvc, jobRepo)
	jobHandler := handler.NewJobHandler(jobRepo)
	snapshotHandler := handler.NewSnapshotHandler(snapshotRepo, snapshotSvc)
	configHandler := handler.NewConfigHandler(configRepo)

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api")
	{
		api.GET("/events", sse.StreamHandler(broker))

		folders := api.Group("/folders")
		{
			folders.GET("", folderHandler.List)
			folders.GET("/:id", folderHandler.Get)
			folders.POST("/:id/restore", folderHandler.Restore)
			folders.PATCH("/:id/category", folderHandler.UpdateCategory)
			folders.PATCH("/:id/status", folderHandler.UpdateStatus)
			folders.DELETE("/:id", folderHandler.Delete)
			folders.POST("/scan", folderHandler.Scan)
		}

		jobs := api.Group("/jobs")
		{
			jobs.GET("", jobHandler.List)
			jobs.GET("/:id", jobHandler.Get)
			jobs.GET("/:id/progress", jobHandler.Progress)
			jobs.POST("/move", moveHandler.Start)
		}

		snapshots := api.Group("/snapshots")
		{
			snapshots.GET("", snapshotHandler.List)
			snapshots.POST("/:id/revert", snapshotHandler.Revert)
		}

		api.GET("/config", configHandler.Get)
		api.PUT("/config", configHandler.Put)
	}

	distFS, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		log.Fatalf("failed to create sub FS: %v", err)
	}

	r.NoRoute(func(c *gin.Context) {
		http.FileServer(http.FS(distFS)).ServeHTTP(c.Writer, c.Request)
	})

	log.Printf("Classifier starting on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
