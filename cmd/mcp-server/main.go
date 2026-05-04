package main

import (
	"log"
	"os"
	gosignal "os/signal"
	"syscall"

	"feishu-mem/internal/config"
	"feishu-mem/internal/core"
	larkadapter "feishu-mem/internal/lark-adapter"
	"feishu-mem/internal/mcp"
	"feishu-mem/internal/storage/bitable"
	"feishu-mem/internal/storage/git"
)

func main() {
	// 1. 加载配置
	larkadapter.LoadEnv()

	settings := config.DefaultSettings()
	if cfgPath := os.Getenv("CONFIG_PATH"); cfgPath != "" {
		if s, err := config.LoadSettings(cfgPath); err == nil {
			settings = s
		}
	} else if s, err := config.LoadSettings("config/openclaw.yaml"); err == nil {
		settings = s
	}

	// 2. 初始化 Git 存储
	var gitStorage *git.GitStorage
	if settings.Git.WorkDir != "" {
		var err error
		gitStorage, err = git.NewGitStorage(git.Config{
			WorkDir:  settings.Git.WorkDir,
			Remote:   settings.Git.Remote,
			AutoPush: false,
			Branch:   settings.Git.Branch,
		})
		if err != nil {
			log.Printf("Warning: Git storage init failed: %v", err)
		}
	}

	// 3. 初始化内存图
	memoryGraph := core.NewMemoryGraph()
	if gitStorage != nil && settings.Memory.PreloadOnStart {
		if err := memoryGraph.LoadFromGit(gitStorage, settings.Project.Name); err != nil {
			log.Printf("Warning: failed to load from Git: %v", err)
		}
	}

	// 4. 初始化 Bitable 存储
	var bitableStore mcp.BitableStoreInterface
	if settings.Bitable.BaseToken != "" {
		bitableStore = bitable.NewBitableStore(bitable.Config{
			BaseToken: settings.Bitable.BaseToken,
			Tables: bitable.TablesConfig{
				Decision: settings.Bitable.Tables.Decision,
				Topic:    settings.Bitable.Tables.Topic,
				Phase:    settings.Bitable.Tables.Phase,
				Relation: settings.Bitable.Tables.Relation,
			},
		}, nil)
		log.Printf("Bitable store initialized with base_token: %s", settings.Bitable.BaseToken)
	} else {
		log.Printf("Warning: Bitable not configured (FEISHU_BASE_TOKEN not set), bitable features disabled")
	}

	// 5. 创建 MCP 服务器
	server := mcp.NewMCPServer(memoryGraph, gitStorage, bitableStore)

	log.Printf("Feishu Memory MCP Server starting...")
	log.Printf("  Project: %s", settings.Project.Name)
	log.Printf("  Decisions loaded: %d", memoryGraph.Count())

	// 6. 启动 MCP Server (stdio 模式)
	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("MCP server error: %v", err)
		}
	}()

	// 7. 等待退出信号
	sigChan := make(chan os.Signal, 1)
	gosignal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	log.Printf("Received signal: %v, shutting down...", sig)
	server.Stop()
	log.Println("MCP server stopped")
}
