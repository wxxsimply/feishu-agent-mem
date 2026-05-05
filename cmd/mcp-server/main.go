package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"feishu-mem/internal/config"
	"feishu-mem/internal/core"
	"feishu-mem/internal/mcp/server"
	"feishu-mem/internal/storage/git"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	slog.Info("Loading config...")

	settings := config.DefaultSettings()
	if cfgPath := os.Getenv("CONFIG_PATH"); cfgPath != "" {
		if s, err := config.LoadSettings(cfgPath); err == nil {
			settings = s
		}
	} else {
		if s, err := config.LoadSettings("config/openclaw.yaml"); err == nil {
			settings = s
		}
	}

	// 初始化 Git 存储
	gitStorage, err := git.NewGitStorage(git.Config{
		WorkDir: settings.Git.WorkDir,
		Branch:  settings.Git.Branch,
	})
	if err != nil {
		slog.Warn("Failed to initialize Git storage, continuing without persistence", "error", err)
	}

	memoryGraph := core.NewMemoryGraph()
	if gitStorage != nil {
		if err := memoryGraph.LoadFromGit(gitStorage, settings.Project.Name); err != nil {
			slog.Warn("Failed to load from Git", "error", err)
		}
	}

	slog.Info("Creating MCP server...")
	srv, err := server.NewMemoryMCPServer(memoryGraph, gitStorage)
	if err != nil {
		slog.Error("Failed to create MCP server", "error", err)
		os.Exit(1)
	}

	slog.Info("Feishu Memory MCP Server starting...")

	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		slog.Info("Received signal, shutting down...")
		cancel()
	}()

	if err := srv.Run(ctx); err != nil && err != context.Canceled {
		slog.Error("MCP server error", "error", err)
		os.Exit(1)
	}

	slog.Info("MCP server stopped")
}
