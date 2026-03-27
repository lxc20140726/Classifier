package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strings"

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
	if err := configRepo.EnsureAppConfig(context.Background()); err != nil {
		log.Fatalf("ensure app config: %v", err)
	}
	auditRepo := repository.NewAuditRepository(sqlDB)
	workflowDefRepo := repository.NewWorkflowDefinitionRepository(sqlDB)
	workflowRunRepo := repository.NewWorkflowRunRepository(sqlDB)
	nodeRunRepo := repository.NewNodeRunRepository(sqlDB)
	nodeSnapshotRepo := repository.NewNodeSnapshotRepository(sqlDB)
	scheduledWorkflowRepo := repository.NewScheduledWorkflowRepository(sqlDB)

	fsAdapter := internalfs.NewOSAdapter()
	broker := sse.NewBroker()

	auditSvc := service.NewAuditService(auditRepo)
	snapshotSvc := service.NewSnapshotService(fsAdapter, snapshotRepo, folderRepo)
	scannerSvc := service.NewScannerService(fsAdapter, folderRepo, jobRepo, snapshotSvc, auditSvc, broker)
	scanJobStarterSvc := service.NewScanJobStarterService(jobRepo, scannerSvc)
	workflowRunnerSvc := service.NewWorkflowRunnerService(jobRepo, folderRepo, workflowDefRepo, workflowRunRepo, nodeRunRepo, nodeSnapshotRepo, fsAdapter, broker, auditSvc)
	scheduledWorkflowSvc := service.NewScheduledWorkflowService(scheduledWorkflowRepo, workflowRunnerSvc, scanJobStarterSvc)
	scheduledWorkflowScheduler := service.NewScheduledWorkflowScheduler(scheduledWorkflowRepo, scheduledWorkflowSvc)
	workflowRunnerSvc.RegisterExecutor(service.NewFolderTreeScannerExecutor(fsAdapter))
	workflowRunnerSvc.RegisterExecutor(service.NewNameKeywordClassifierExecutor())
	workflowRunnerSvc.RegisterExecutor(service.NewFileTreeClassifierExecutor())
	workflowRunnerSvc.RegisterExecutor(service.NewConfidenceCheckExecutor())
	workflowRunnerSvc.RegisterExecutor(service.NewManualClassifierExecutor())
	workflowRunnerSvc.RegisterExecutor(service.NewSubtreeAggregatorExecutor(folderRepo, auditSvc))
	if err := service.SeedDefaultWorkflow(context.Background(), workflowDefRepo); err != nil {
		log.Fatalf("seed default workflow: %v", err)
	}
	if err := service.SeedDefaultProcessingWorkflow(context.Background(), workflowDefRepo); err != nil {
		log.Fatalf("seed default processing workflow: %v", err)
	}

	if err := scheduledWorkflowSvc.BootstrapLegacyScanCron(context.Background(), configRepo); err != nil {
		log.Fatalf("bootstrap legacy scan cron: %v", err)
	}
	if err := scheduledWorkflowScheduler.Start(context.Background()); err != nil {
		log.Fatalf("start scheduled workflow scheduler: %v", err)
	}
	defer func() {
		if err := scheduledWorkflowScheduler.Stop(context.Background()); err != nil {
			log.Printf("stop scheduled workflow scheduler: %v", err)
		}
	}()

	folderHandler := handler.NewFolderHandler(folderRepo, configRepo, scheduledWorkflowRepo, scanJobStarterSvc, fsAdapter, cfg.SourceDir, cfg.DeleteStagingDir)
	jobHandler := handler.NewJobHandlerWithWorkflow(jobRepo, workflowRunnerSvc)
	snapshotHandler := handler.NewSnapshotHandler(snapshotRepo, snapshotSvc)
	configHandler := handler.NewConfigHandler(configRepo, nil)
	auditHandler := handler.NewAuditHandler(auditRepo)
	nodeTypeHandler := handler.NewNodeTypeHandler(workflowRunnerSvc)
	workflowDefHandler := handler.NewWorkflowDefHandler(workflowDefRepo, workflowRunnerSvc)
	workflowRunHandler := handler.NewWorkflowRunHandler(workflowRunnerSvc)
	scheduledWorkflowHandler := handler.NewScheduledWorkflowHandler(scheduledWorkflowRepo, scheduledWorkflowSvc, scheduledWorkflowScheduler)

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
			jobs.POST("", jobHandler.StartWorkflow)
			jobs.GET("/:id", jobHandler.Get)
			jobs.GET("/:id/progress", jobHandler.Progress)
			jobs.GET("/:id/workflow-runs", workflowRunHandler.ListByJob)
		}

		workflowRuns := api.Group("/workflow-runs")
		{
			workflowRuns.GET("/:id", workflowRunHandler.Get)
			workflowRuns.POST("/:id/resume", workflowRunHandler.Resume)
			workflowRuns.POST("/:id/provide-input", workflowRunHandler.ProvideInput)
			workflowRuns.POST("/:id/rollback", workflowRunHandler.Rollback)
		}

		workflowDefs := api.Group("/workflow-defs")
		{
			workflowDefs.GET("", workflowDefHandler.List)
			workflowDefs.POST("", workflowDefHandler.Create)
			workflowDefs.GET("/:id", workflowDefHandler.Get)
			workflowDefs.PUT("/:id", workflowDefHandler.Update)
			workflowDefs.DELETE("/:id", workflowDefHandler.Delete)
		}

		scheduledWorkflows := api.Group("/scheduled-workflows")
		{
			scheduledWorkflows.GET("", scheduledWorkflowHandler.List)
			scheduledWorkflows.POST("", scheduledWorkflowHandler.Create)
			scheduledWorkflows.GET("/:id", scheduledWorkflowHandler.Get)
			scheduledWorkflows.PUT("/:id", scheduledWorkflowHandler.Update)
			scheduledWorkflows.DELETE("/:id", scheduledWorkflowHandler.Delete)
			scheduledWorkflows.POST("/:id/run", scheduledWorkflowHandler.RunNow)
		}

		snapshots := api.Group("/snapshots")
		{
			snapshots.GET("", snapshotHandler.List)
			snapshots.POST("/:id/revert", snapshotHandler.Revert)
		}

		api.GET("/config", configHandler.Get)
		api.PUT("/config", configHandler.Put)
		api.GET("/node-types", nodeTypeHandler.List)
		api.GET("/audit-logs", auditHandler.List)
		api.GET("/fs/dirs", handler.NewFSHandler(fsAdapter).ListDirs)
	}

	distFS, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		log.Fatalf("failed to create sub FS: %v", err)
	}
	assetServer := http.FileServer(http.FS(distFS))

	r.NoRoute(func(c *gin.Context) {
		assetPath := strings.TrimPrefix(c.Request.URL.Path, "/")
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "api route not found"})
			return
		}
		if assetPath != "" {
			if _, err := fs.Stat(distFS, assetPath); err == nil {
				assetServer.ServeHTTP(c.Writer, c.Request)
				return
			}
			if filepath.Ext(assetPath) != "" {
				c.Status(http.StatusNotFound)
				return
			}
		}

		indexHTML, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	})

	log.Printf("Classifier starting on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
