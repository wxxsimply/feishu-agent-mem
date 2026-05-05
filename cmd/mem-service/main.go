package main

import (
	"context"
	"flag"
	"log"
	"os"
	gosignal "os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"feishu-mem/internal/config"
	"feishu-mem/internal/core"
	larkadapter "feishu-mem/internal/lark-adapter"
	"feishu-mem/internal/mcp/server"
	"feishu-mem/internal/signal"
	"feishu-mem/internal/storage/bitable"
	"feishu-mem/internal/storage/git"
)

// detectorState 单个检测器的状态
type detectorState struct {
	detector        larkadapter.Detector
	adapterType    signal.AdapterType
	config         config.DetectorConfig
	inBurstMode    bool
	lastChangeTime time.Time
	lastCheck      time.Time
	enabled        bool
}

func main() {
	var mode string
	flag.StringVar(&mode, "mode", "service", "运行模式: service (后台服务), mcp (MCP stdio模式)")
	flag.Parse()

	larkadapter.LoadEnv()

	settings := config.DefaultSettings()
	if cfgPath := os.Getenv("CONFIG_PATH"); cfgPath != "" {
		if s, err := config.LoadSettings(cfgPath); err == nil {
			settings = s
		}
	} else if s, err := config.LoadSettings("config/openclaw.yaml"); err == nil {
		settings = s
	} else if s, err := config.LoadSettings("openclaw.yaml"); err == nil {
		settings = s
	}

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
		if err := memoryGraph.LoadFromGit(gitStorage, settings.Project.Name); err != nil {
			log.Printf("[Memory] Warning: Failed to load from Git: %v", err)
		}
	}

	if mode == "mcp" {
		// MCP 模式（使用官方 SDK）
		log.Println("[MCP] Starting in MCP stdio mode (using official SDK)...")
		srv, err := server.NewMemoryMCPServer(memoryGraph, gitStorage)
		if err != nil {
			log.Fatalf("[MCP] Failed to create server: %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if err := srv.Run(ctx); err != nil {
			log.Fatalf("[MCP] Server error: %v", err)
		}
		return
	}

	// Service 模式
	log.Println("========================================")
	log.Println("Starting feishu-agent-mem service... (v2)")
	log.Println("========================================")
	log.Printf("[System] NumGoroutine: %d", runtime.NumGoroutine())
	log.Printf("[System] NumCPU: %d", runtime.NumCPU())

	larkCfg := larkadapter.LoadConfig()

	log.Printf("[Config] Project: %s", settings.Project.Name)
	log.Printf("[Config] ChatIDs: %v", larkCfg.ChatIDs)
	log.Printf("[Config] MCP Port: %d", settings.MCP.Port)

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

	// 初始化检测器状态
	docExtractor := larkadapter.NewDocExtractor(larkCfg)
	if len(settings.Detectors.LarkDoc.DocTokens) > 0 {
		docExtractor.SetDocTokens(settings.Detectors.LarkDoc.DocTokens)
	}
	detectorStates := map[signal.AdapterType]*detectorState{
		signal.AdapterIM: {
			detector: larkadapter.NewIMExtractor(larkCfg),
			adapterType: signal.AdapterIM,
			config: settings.Detectors.LarkIM,
			enabled: settings.Detectors.LarkIM.Enabled,
		},
		signal.AdapterVC: {
			detector: larkadapter.NewVCExtractor(larkCfg),
			adapterType: signal.AdapterVC,
			config: settings.Detectors.LarkVC,
			enabled: settings.Detectors.LarkVC.Enabled,
		},
	signal.AdapterDocs: {
			detector: docExtractor,
			adapterType: signal.AdapterDocs,
			config: settings.Detectors.LarkDoc,
			enabled: settings.Detectors.LarkDoc.Enabled,
		},
		signal.AdapterCalendar: {
			detector: larkadapter.NewCalendarExtractor(larkCfg),
			adapterType: signal.AdapterCalendar,
			config: settings.Detectors.LarkCalendar,
			enabled: settings.Detectors.LarkCalendar.Enabled,
		},
		signal.AdapterTask: {
			detector: larkadapter.NewTaskExtractor(larkCfg),
			adapterType: signal.AdapterTask,
			config: settings.Detectors.LarkTask,
			enabled: settings.Detectors.LarkTask.Enabled,
		},
		signal.AdapterWiki: {
			detector: larkadapter.NewWikiExtractor(larkCfg),
			adapterType: signal.AdapterWiki,
			config: settings.Detectors.LarkWiki,
			enabled: settings.Detectors.LarkWiki.Enabled,
		},
	}

	// 打印检测器配置
	log.Println("[Detector] Configuration:")
	for at, ds := range detectorStates {
		if ds.enabled {
			log.Printf("  %v: interval=%v, burst=%v, timeout=%v",
				at, ds.config.Interval, ds.config.BurstInterval, ds.config.BurstTimeout)
		} else {
			log.Printf("  %v: disabled", at)
		}
	}

	stateMgr := larkadapter.NewStateManager(
		filepath.Join(larkadapter.StateDir(), "detect_state.json"),
	)

	// 初始化防抖追踪器
	var docDebounceTracker *larkadapter.DocDebounceTracker
	if settings.Detectors.LarkDoc.EnableDebounce || settings.Detectors.LarkWiki.EnableDebounce {
		debounceWindow := settings.Detectors.LarkDoc.DebounceWindow
		if debounceWindow <= 0 {
			debounceWindow = 120 * time.Second
		}
		docDebounceTracker = larkadapter.NewDocDebounceTracker(larkadapter.StateDir(), debounceWindow)
		log.Printf("[Debounce] Initialized with window: %v", debounceWindow)

		// 注入到 extractor
		docExtractor.SetDebounceTracker(docDebounceTracker)
	}

	maxWorkers := max(runtime.NumCPU(), 2)
	workerPool := signal.NewWorkerPool(signalEngine, maxWorkers)

	// 将防抖追踪器注入到 worker pool 中（需要先修改 worker pool 支持）
	if docDebounceTracker != nil {
		workerPool.SetDebounceTracker(docDebounceTracker)
	}

	log.Println("[Service] Running in service mode (v2 with burst mode)")
	log.Printf("[Service] Decisions loaded: %d", memoryGraph.Count())
	log.Printf("[Service] Topics: %d", memoryGraph.TopicCount(settings.Project.Name))
	log.Printf("[Service] MCP port: %d", settings.MCP.Port)

	workerPool.Start()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go resultProcessor(ctx, workerPool, pipeline, workerPool.Results())

	// 初始化所有检测器的lastCheck
	log.Println("[Service] Initializing detector states...")
	for _, ds := range detectorStates {
		if ds.enabled {
			ds.lastCheck = stateMgr.GetLastCheck(ds.detector.Name())
		}
	}

	log.Println("[Service] Starting detector goroutines...")
	// 为每个启用的检测器启动独立的协程
	for _, ds := range detectorStates {
		if !ds.enabled {
			continue
		}
		ds := ds // 捕获变量
		go runDetectorLoop(ctx, ds, signalEngine, stateMgr, workerPool, docDebounceTracker)
	}

	sigChan := make(chan os.Signal, 1)
	gosignal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	statsTicker := time.NewTicker(30 * time.Second)
	defer statsTicker.Stop()

	// 仅打印统计信息
	go func() {
		for {
			select {
			case <-statsTicker.C:
				log.Println("-----------------------------------")
				log.Printf("[System] Running goroutines: %d", runtime.NumGoroutine())
				workerPool.LogStats()
				// 打印检测器状态
				log.Printf("[Detectors] Status:")
				for at, ds := range detectorStates {
					if ds.enabled {
						mode := "normal"
						if ds.inBurstMode {
							mode = "BURST"
						}
						log.Printf("  %v: mode=%s, lastChange=%v", at, mode, ds.lastChangeTime)
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	sig := <-sigChan
	log.Printf("[System] Received signal: %v, shutting down...", sig)

	workerPool.Stop()

	log.Println("[System] feishu-agent-mem service stopped successfully")
}

// runDetectorLoop 单个检测器的循环
func runDetectorLoop(
	ctx context.Context,
	ds *detectorState,
	signalEngine *signal.SignalActivationEngine,
	stateMgr *larkadapter.StateManager,
	workerPool *signal.WorkerPool,
	debounceTracker *larkadapter.DocDebounceTracker,
) {
	detectorName := ds.detector.Name()
	log.Printf("[Detector] Starting loop for %s", detectorName)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[Detector] Loop stopped for %s", detectorName)
			return
		default:
		}

		// 执行一次检测
		hasChanges := runSingleDetection(ds, signalEngine, stateMgr, workerPool, debounceTracker)

		// 计算下次检测间隔
		var nextInterval time.Duration
		if ds.inBurstMode {
			// 检查是否需要退出突发模式
			if time.Since(ds.lastChangeTime) > ds.config.BurstTimeout {
				log.Printf("[Detector] %s: No changes for %v, exiting burst mode",
					detectorName, ds.config.BurstTimeout)
				ds.inBurstMode = false
				nextInterval = ds.config.Interval
			} else {
				// 保持突发模式
				nextInterval = ds.config.BurstInterval
			}
		} else {
			// 正常模式
			nextInterval = ds.config.Interval
		}

		// 如果这次检测到变化，进入或保持突发模式
		if hasChanges {
			log.Printf("[Detector] %s: Changes detected, entering burst mode", detectorName)
			ds.inBurstMode = true
			ds.lastChangeTime = time.Now()
			nextInterval = ds.config.BurstInterval
		}

		// 等待下次检测
		if nextInterval > 0 {
			log.Printf("[Detector] %s: Next check in %v (mode=%s)",
				detectorName, nextInterval, boolToModeStr(ds.inBurstMode))
			select {
			case <-time.After(nextInterval):
			case <-ctx.Done():
				return
			}
		}
	}
}

// runSingleDetection 单次检测
func runSingleDetection(
	ds *detectorState,
	_ *signal.SignalActivationEngine,
	stateMgr *larkadapter.StateManager,
	workerPool *signal.WorkerPool,
	debounceTracker *larkadapter.DocDebounceTracker,
) bool {
	detectorName := ds.detector.Name()
	lastCheck := ds.lastCheck

	log.Printf("[Detector] %s: Checking for changes (lastCheck=%v, mode=%s)",
		detectorName, lastCheck, boolToModeStr(ds.inBurstMode))

	// 执行检测
	result, err := ds.detector.Detect(lastCheck)
	if err != nil {
		log.Printf("[Detector] %s: Failed: %v", detectorName, err)
		return false
	}

	detectTime := time.Now()
	ds.lastCheck = detectTime
	_ = stateMgr.UpdateLastCheck(detectorName, detectTime)

	// 处理检测结果
	if !result.HasChanges {
		log.Printf("[Detector] %s: No changes", detectorName)
		return false
	}

	log.Printf("[Detector] %s: Detected %d changes", detectorName, len(result.Changes))
	hasSubmittedChanges := false
	for i, change := range result.Changes {
		log.Printf("[Detector] Change %d: %s [%s]", i+1, change.Type, change.Summary)

		// 如果启用了防抖，先检查文档是否可以处理
		shouldSubmit := true
		if debounceTracker != nil && (ds.adapterType == signal.AdapterDocs || ds.adapterType == signal.AdapterWiki) {
			// 获取文档 token
			docToken := change.EntityID
			if actualToken, ok := change.Meta["actual_doc_token"]; ok && actualToken != "" {
				docToken = actualToken
			}

			// 先通知追踪器有变更（更新 lastChange 时间）
			contentHash, _ := change.Meta["content_hash"]
			debounceTracker.OnDocumentChanged(docToken, contentHash)

			// 检查是否可以处理
			canProcess, reason := debounceTracker.CanProcessNow(docToken)
			if !canProcess {
				log.Printf("[Debounce] Skipping change %d: %s", i+1, reason)
				shouldSubmit = false
			}
		}

		if shouldSubmit {
			job := &signal.DetectionJob{
				AdapterType: ds.adapterType,
				Change:      change,
				ReceivedAt:  time.Now(),
			}
			workerPool.SubmitJob(job)
			hasSubmittedChanges = true
		}
	}

	return hasSubmittedChanges
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

			// 处理附加变更（如反对意见）
			for _, pendingMut := range result.PendingMutations {
				log.Printf("[ResultProcessor] Applying pending mutation: %s (type=%s)",
					pendingMut.SDRID, pendingMut.Type)
				if err := pipeline.ApplyMutation(pendingMut); err != nil {
					log.Printf("[ResultProcessor] Failed to apply pending mutation: %v", err)
				} else {
					log.Printf("[ResultProcessor] Pending mutation applied successfully")
				}
			}

		case <-ctx.Done():
			log.Println("[ResultProcessor] Stopped")
			return
		}
	}
}

func boolToModeStr(burst bool) string {
	if burst {
		return "BURST"
	}
	return "normal"
}
