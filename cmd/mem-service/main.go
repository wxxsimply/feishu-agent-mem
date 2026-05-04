package main

import (
	"context"
	"log"
	"os"
	gosignal "os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"feishu-mem/internal/config"
	"feishu-mem/internal/core"
	larkadapter "feishu-mem/internal/lark-adapter"
	"feishu-mem/internal/mcp"
	"feishu-mem/internal/signal"
	"feishu-mem/internal/storage/bitable"
	"feishu-mem/internal/storage/git"
)

func main() {
	larkadapter.LoadEnv()

	log.Println("========================================")
	log.Println("Starting feishu-agent-mem service...")
	log.Println("========================================")
	log.Printf("[System] NumGoroutine: %d", runtime.NumGoroutine())
	log.Printf("[System] NumCPU: %d", runtime.NumCPU())

	settings := config.DefaultSettings()
	if cfgPath := os.Getenv("CONFIG_PATH"); cfgPath != "" {
		if s, err := config.LoadSettings(cfgPath); err == nil {
			settings = s
		}
	} else if s, err := config.LoadSettings("config/openclaw.yaml"); err == nil {
		settings = s
	}
	larkCfg := larkadapter.LoadConfig()

	log.Printf("[Config] Project: %s", settings.Project.Name)
	log.Printf("[Config] ChatIDs: %v", larkCfg.ChatIDs)
	log.Printf("[Config] MCP Port: %d", settings.MCP.Port)

	gitStorage, err := git.NewGitStorage(git.Config{
		WorkDir:  settings.Git.WorkDir,
		Remote:   settings.Git.Remote,
		AutoPush: settings.Git.AutoPush,
		Branch:   settings.Git.Branch,
	})
	if err != nil {
		log.Fatalf("[Git] Failed to initialize: %v", err)
	}

	memoryGraph := core.NewMemoryGraph()
	if settings.Memory.PreloadOnStart {
		log.Println("[Memory] Loading decisions from Git...")
		if err := memoryGraph.LoadFromGit(gitStorage, settings.Project.Name); err != nil {
			log.Printf("[Memory] Warning: Failed to load from Git: %v", err)
		} else {
			log.Printf("[Memory] Loaded %d decisions into memory", memoryGraph.Count())
		}
	}

	larkCLI := larkadapter.NewLarkCLI()
	bitableStore := bitable.NewBitableStore(bitable.Config{
		BaseToken: settings.Bitable.BaseToken,
		Tables: bitable.TablesConfig{
			Decision: settings.Bitable.Tables.Decision,
			Topic:    settings.Bitable.Tables.Topic,
		},
	}, larkCLI)

	pipeline := core.NewPipelineEngine(gitStorage, bitableStore, memoryGraph)
	signalEngine := signal.NewSignalActivationEngine(pipeline, memoryGraph)

	detectors := map[signal.AdapterType]larkadapter.Detector{
		signal.AdapterIM:       larkadapter.NewIMExtractor(larkCfg),
		signal.AdapterVC:       larkadapter.NewVCExtractor(larkCfg),
		signal.AdapterDocs:     larkadapter.NewDocExtractor(larkCfg),
		signal.AdapterCalendar: larkadapter.NewCalendarExtractor(larkCfg),
		signal.AdapterTask:     larkadapter.NewTaskExtractor(larkCfg),
		signal.AdapterWiki:     larkadapter.NewWikiExtractor(larkCfg),
	}

	stateMgr := larkadapter.NewStateManager(
		filepath.Join(larkadapter.StateDir(), "detect_state.json"),
	)

	maxWorkers := max(runtime.NumCPU(), 2)
	workerPool := signal.NewWorkerPool(signalEngine, maxWorkers)

	mcpServer := mcp.NewMCPServer(memoryGraph, gitStorage, bitableStore)

	if os.Getenv("MCP_SERVER_MODE") == "stdio" {
		log.Println("[MCP] Starting in stdio mode...")
		if err := mcpServer.Start(); err != nil {
			log.Fatalf("[MCP] Server error: %v", err)
		}
		return
	}

	log.Println("[Service] Running in service mode")
	log.Printf("[Service] Decisions loaded: %d", memoryGraph.Count())
	log.Printf("[Service] Topics: %d", memoryGraph.TopicCount(settings.Project.Name))
	log.Printf("[Service] MCP port: %d", settings.MCP.Port)

	workerPool.Start()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go resultProcessor(ctx, workerPool, pipeline, workerPool.Results())

	log.Println("[Service] Initial detection cycle...")
	runDetectionCycle(detectors, signalEngine, stateMgr, workerPool)

	sigChan := make(chan os.Signal, 1)
	gosignal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(settings.Polling.Interval)
	defer ticker.Stop()

	statsTicker := time.NewTicker(30 * time.Second)
	defer statsTicker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				log.Println("-----------------------------------")
				log.Printf("[System] Running goroutines: %d", runtime.NumGoroutine())
				runDetectionCycle(detectors, signalEngine, stateMgr, workerPool)
			case <-statsTicker.C:
				workerPool.LogStats()
			case <-ctx.Done():
				return
			}
		}
	}()

	sig := <-sigChan
	log.Printf("[System] Received signal: %v, shutting down...", sig)

	workerPool.Stop()

	if err := mcpServer.Stop(); err != nil {
		log.Printf("[MCP] Error stopping server: %v", err)
	}

	log.Println("[System] feishu-agent-mem service stopped successfully")
}

func resultProcessor(
	ctx context.Context,
	_ *signal.WorkerPool,
	pipeline *core.PipelineEngine,
	results <-chan *signal.DecisionResult,
) {
	log.Println("[ResultProcessor] Started")

	for {
		select {
		case result := <-results:
			if result == nil {
				continue
			}

			log.Printf("[ResultProcessor] Received result for: %s", result.Job.Change.Summary)

			if result.Err != nil {
				log.Printf("[ResultProcessor] Error: %v", result.Err)
				continue
			}

			if result.Mutation != nil {
				log.Printf("[ResultProcessor] Applying mutation: %s", result.Mutation.SDRID)

				if err := pipeline.ApplyMutation(result.Mutation); err != nil {
					log.Printf("[ResultProcessor] Failed to apply mutation: %v", err)
				} else {
					log.Printf("[ResultProcessor] Mutation applied successfully")
				}
			}

		case <-ctx.Done():
			log.Println("[ResultProcessor] Stopped")
			return
		}
	}
}

func runDetectionCycle(
	detectors map[signal.AdapterType]larkadapter.Detector,
	_ *signal.SignalActivationEngine,
	stateMgr *larkadapter.StateManager,
	workerPool *signal.WorkerPool,
) {
	log.Printf("[Detector] Starting cycle with %d detectors", len(detectors))

	var wg sync.WaitGroup
	detectChan := make(chan *detectResult, len(detectors))

	// 1. 并行执行所有检测器
	for adapter, detector := range detectors {
		wg.Add(1)
		go func(a signal.AdapterType, d larkadapter.Detector) {
			defer wg.Done()
			lastCheck := stateMgr.GetLastCheck(d.Name())
			log.Printf("[Detector] %s: last check = %v", d.Name(), lastCheck)

			result, err := larkadapter.ExtractDetect(d)
			detectChan <- &detectResult{
				adapter:    a,
				detector:   d,
				result:     result,
				err:        err,
				detectTime: lastCheck,
			}
		}(adapter, detector)
	}

	// 2. 等待所有检测器完成
	go func() {
		wg.Wait()
		close(detectChan)
	}()

	// 3. 处理检测结果
	for dr := range detectChan {
		if dr.err != nil {
			log.Printf("[Detector] %s: Failed: %v", dr.detector.Name(), dr.err)
			continue
		}

		// 先处理变化，再更新时间，避免丢失
		if dr.result.HasChanges {
			log.Printf("[Detector] %s: Detected %d changes", dr.detector.Name(), len(dr.result.Changes))

			for i, change := range dr.result.Changes {
				log.Printf("[Detector] Change %d: %s [%s]", i+1, change.Type, change.Summary)

				job := &signal.DetectionJob{
					AdapterType: dr.adapter,
					Change:      change,
					ReceivedAt:  time.Now(),
				}

				workerPool.SubmitJob(job)
			}
		} else {
			log.Printf("[Detector] %s: No changes detected", dr.detector.Name())
		}

		_ = stateMgr.UpdateLastCheck(dr.detector.Name(), time.Now())
	}

	log.Println("[Detector] Cycle completed")
}

// detectResult 用于传递检测结果
type detectResult struct {
	adapter    signal.AdapterType
	detector   larkadapter.Detector
	result     *larkadapter.DetectResult
	err        error
	detectTime time.Time
}
