package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jakkayy/kuberlauncher/api/config"
	"github.com/jakkayy/kuberlauncher/api/internal/db"
	"github.com/jakkayy/kuberlauncher/api/internal/handler"
	"github.com/jakkayy/kuberlauncher/api/internal/repository"
	"github.com/jakkayy/kuberlauncher/api/internal/service"
)

func main() {
	cfg := config.Load()

	database, err := db.Connect(cfg.DB.DSN())
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	projectRepo := repository.NewProjectRepository(database)
	projectSvc := service.NewProjectService(projectRepo)
	projectHandler := handler.NewProjectHandler(projectSvc)

	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")
	{
		projects := v1.Group("/projects")
		projects.POST("", projectHandler.Create)
		projects.GET("", projectHandler.List)
		projects.GET("/:id", projectHandler.GetByID)
		projects.DELETE("/:id", projectHandler.Delete)
		projects.GET("/:id/download", projectHandler.Download)
		projects.GET("/:id/preview", projectHandler.Preview)
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Port),
		Handler: r,
	}

	go func() {
		log.Printf("server listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server shutdown: %v", err)
	}
}
