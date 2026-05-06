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
	"feishu-mem/internal/push"
	"feishu-mem/internal/signal"
	"feishu-mem/internal/storage/bitable"
	"feishu-mem/internal/storage/git"
	"feishu-mem/internal/ws"
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

	// 初始化防抖追踪器
	var docDebounceTracker *larkadapter.DocDebounceTracker
	if settings.Detectors.LarkDoc.EnableDebounce {
		docDebounceTracker = larkadapter.NewDocDebounceTracker(
			larkadapter.StateDir(),
			settings.Detectors.LarkDoc.DebounceWindow,
		)
		log.Printf("[Service] Debounce tracker enabled, window: %v", settings.Detectors.LarkDoc.DebounceWindow)
	} else {
		log.Println("[Service] Debounce tracker disabled")
	}

	// 初始化检测器状态
	docExtractor := larkadapter.NewDocExtractor(larkCfg)
	if len(settings.Detectors.LarkDoc.DocTokens) > 0 {
		docExtractor.SetDocTokens(settings.Detectors.LarkDoc.DocTokens)
	}
	if docDebounceTracker != nil {
		docExtractor.SetDebounceTracker(docDebounceTracker)
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

	maxWorkers := max(runtime.NumCPU(), 2)
	workerPool := signal.NewWorkerPool(signalEngine, maxWorkers)
	if docDebounceTracker != nil {
		workerPool.SetDebounceTracker(docDebounceTracker)
	}

	log.Println("[Service] Running in service mode (v2 with burst mode)")
	log.Printf("[Service] Decisions loaded: %d", memoryGraph.Count())
	log.Printf("[Service] Topics: %d", memoryGraph.TopicCount(settings.Project.Name))
	log.Printf("[Service] MCP port: %d", settings.MCP.Port)

	// 启动推送调度器
	chatIDs := larkCfg.ChatIDs
	var pushScheduler *push.PushScheduler
	if len(chatIDs) > 0 {
		pushEngine := push.NewPushEngine(memoryGraph)
		pushScheduler = push.NewPushScheduler(pushEngine, chatIDs)
		log.Printf("[Service] PushScheduler initialized (chats: %v)", chatIDs)
	}

	workerPool.Start()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go resultProcessor(ctx, workerPool, pipeline, memoryGraph, pushScheduler, workerPool.Results())

	// 启动状态自动更新器
	statusUpdater := signal.NewStatusUpdater(memoryGraph, pipeline, 5*time.Minute)
	go statusUpdater.Start(ctx)
	log.Println("[Service] StatusUpdater started (interval: 5m)")

	// 启动脏数据定期持久化（每 5 分钟把 AccessStats 变更写回 Git）
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				dirtyNodes := memoryGraph.GetDirtyAndClean()
				for _, node := range dirtyNodes {
					if _, err := gitStorage.WriteDecision(node); err != nil {
						log.Printf("[Flush] Failed to persist %s: %v", node.SDRID, err)
					}
				}
				if len(dirtyNodes) > 0 {
					log.Printf("[Flush] Persisted %d dirty nodes to Git", len(dirtyNodes))
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	log.Println("[Service] Dirty flush goroutine started (interval: 5m)")

	// 启动推送调度器
	if pushScheduler != nil {
		go pushScheduler.Start(ctx)
		log.Printf("[Service] PushScheduler started (chats: %v)", chatIDs)
	}

	// 初始化所有检测器的lastCheck和lastDetected
	log.Println("[Service] Initializing detector states...")
	for _, ds := range detectorStates {
		if ds.enabled {
			ds.lastCheck = stateMgr.GetLastCheck(ds.detector.Name())
			// 关键！用 LastDetected 作为实际检测变化的时间点！
			// 只有检测到变化时才更新 LastDetected，避免跳过文档更新！
			ds.lastCheck = stateMgr.GetLastDetected(ds.detector.Name())
		}
	}

	// 启动 WebSocket 服务端（接收独立 detector 进程的检测结果）
	wsServer := ws.NewServer(settings.Service.WSPort)
	if err := wsServer.Start(); err != nil {
		log.Fatalf("[WebSocket] Failed to start server: %v", err)
	}
	log.Printf("[WebSocket] Server started on port %d", settings.Service.WSPort)
	defer wsServer.Stop()

	// 启动 WebSocket 结果处理器
	go wsResultProcessor(ctx, workerPool, wsServer.DetectResultChannel())

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

	// 关键！LastCheck 仍然每次都更新（用于记录活跃度）
	_ = stateMgr.UpdateLastCheck(detectorName, detectTime)

	// 处理检测结果
	if !result.HasChanges {
		log.Printf("[Detector] %s: No changes", detectorName)
		// 没有变化，ds.lastCheck 仍然是原来的值（LastDetected）
		return false
	}

	log.Printf("[Detector] %s: Detected %d changes", detectorName, len(result.Changes))

	// 找到最新的变化时间戳，作为新的 lastDetected
	newestTs := int64(0)
	for _, change := range result.Changes {
		if change.Timestamp > newestTs {
			newestTs = change.Timestamp
		}
	}
	if newestTs == 0 {
		newestTs = detectTime.Unix()
	}
	newLastDetected := time.Unix(newestTs, 0)

	// 确保 last_detected 不会被设置成未来时间
	now := time.Now()
	if newLastDetected.After(now) {
		log.Printf("[Detector] %s: newLastDetected (%v) is in future, using now instead",
			detectorName, newLastDetected)
		newLastDetected = now
	}

	// 更新 lastDetected 为最新的变化时间！下次用这个作为起点继续检测！
	ds.lastCheck = newLastDetected
	_ = stateMgr.UpdateLastDetected(detectorName, newLastDetected)

	// 现在提交任务
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
	memoryGraph *core.MemoryGraph,
	pushScheduler *push.PushScheduler,
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
					// 推送更新的决策卡片
					if pushScheduler != nil {
						if node, ok := memoryGraph.GetDecision(result.Mutation.SDRID); ok {
							log.Printf("[ResultProcessor] Notifying push of decision update: %s", node.SDRID)
							pushScheduler.NotifyDecisionUpdate(node)
						}
					}
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
					// 推送更新的决策卡片
					if pushScheduler != nil {
						if node, ok := memoryGraph.GetDecision(pendingMut.SDRID); ok {
							log.Printf("[ResultProcessor] Notifying push of decision update: %s", node.SDRID)
							pushScheduler.NotifyDecisionUpdate(node)
						}
					}
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

// wsResultProcessor 处理来自 WebSocket 的检测结果
func wsResultProcessor(
	ctx context.Context,
	workerPool *signal.WorkerPool,
	resultChan <-chan ws.DetectResultMessage,
) {
	log.Println("[WebSocket] Result processor started")

	for {
		select {
		case msg := <-resultChan:
			log.Printf("[WebSocket] Processing result from %s: %d changes",
				msg.DetectorName, len(msg.Result.Changes))

			// 转换 adapter 类型
			adapterType := adapterTypeFromName(msg.DetectorName)

			// 处理每个变化
			for i, changeItem := range msg.Result.Changes {
				log.Printf("[WebSocket] Change %d: %s [%s]", i+1, changeItem.Type, changeItem.Summary)

				// 转换为 larkadapter.Change
				change := larkadapter.Change{
					Type:       changeItem.Type,
					EntityType: changeItem.EntityType,
					EntityID:   changeItem.EntityID,
					Summary:    changeItem.Summary,
					Timestamp:  changeItem.Timestamp,
					RawContent: changeItem.Content,
				}

				// 提交任务
				job := &signal.DetectionJob{
					AdapterType: adapterType,
					Change:      change,
					ReceivedAt:  time.Now(),
				}
				workerPool.SubmitJob(job)
			}

		case <-ctx.Done():
			log.Println("[WebSocket] Result processor stopped")
			return
		}
	}
}

// adapterTypeFromName 从检测器名称获取 adapter 类型
func adapterTypeFromName(name string) signal.AdapterType {
	switch name {
	case "lark_im":
		return signal.AdapterIM
	case "lark_doc":
		return signal.AdapterDocs
	case "lark_wiki":
		return signal.AdapterWiki
	case "lark_calendar":
		return signal.AdapterCalendar
	case "lark_task":
		return signal.AdapterTask
	case "lark_vc":
		return signal.AdapterVC
	default:
		return signal.AdapterIM
	}
}
