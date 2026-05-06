package detector

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"feishu-mem/internal/config"
	larkadapter "feishu-mem/internal/lark-adapter"
	"feishu-mem/internal/ws"
)

// LongConnDetector 支持长连接的检测器接口
type LongConnDetector interface {
	larkadapter.Detector
	StartLongConn(handler larkadapter.LongConnResultHandler) error
}

// BaseDetector 独立检测器基类
type BaseDetector struct {
	name              string
	version           string
	detector          larkadapter.Detector
	wsClient          *ws.Client
	config            config.DetectorConfig
	larkConfig        *larkadapter.Config
	stateMgr          *larkadapter.StateManager
	inBurstMode       bool
	lastCheck         time.Time
	lastChangeTime    time.Time
	stopChan          chan struct{}
	stopWg            sync.WaitGroup
	useLongConn       bool // 是否使用长连接模式
}

// NewBaseDetector 创建通用检测器基类
func NewBaseDetector(name string, version string, detector larkadapter.Detector) *BaseDetector {
	return &BaseDetector{
		name:     name,
		version:  version,
		detector: detector,
		stopChan: make(chan struct{}),
	}
}

// EnableLongConn 启用长连接模式
func (d *BaseDetector) EnableLongConn() {
	d.useLongConn = true
}

// Initialize 初始化检测器
func (d *BaseDetector) Initialize() error {
	// 加载环境变量
	_ = godotenv.Load()

	// 加载配置
	var configLoadedFrom string
	settings := config.DefaultSettings()
	if cfgPath := os.Getenv("CONFIG_PATH"); cfgPath != "" {
		if s, err := config.LoadSettings(cfgPath); err == nil {
			settings = s
			configLoadedFrom = cfgPath
		}
	} else if s, err := config.LoadSettings("config/openclaw.yaml"); err == nil {
		settings = s
		configLoadedFrom = "config/openclaw.yaml"
	} else if s, err := config.LoadSettings("openclaw.yaml"); err == nil {
		settings = s
		configLoadedFrom = "openclaw.yaml"
	}

	if configLoadedFrom != "" {
		log.Printf("[Config] Loaded from: %s", configLoadedFrom)
	}

	// 获取当前检测器配置
	d.config = d.getDetectorConfig(settings)
	d.larkConfig = larkadapter.LoadConfig()

	if d.config.Enabled {
		log.Printf("========================================")
		log.Printf("Starting %s (v%s)", d.name, d.version)
		log.Printf("========================================")
		// 打印配置
		log.Printf("[Config] Enabled: %v", d.config.Enabled)
		log.Printf("[Config] Interval: %v", d.config.Interval)
		log.Printf("[Config] Burst Interval: %v", d.config.BurstInterval)
		log.Printf("[Config] Burst Timeout: %v", d.config.BurstTimeout)
		log.Printf("[Config] Heartbeat Interval: %v", d.config.HeartbeatInterval)
	} else {
		log.Printf("[%s] Detector disabled (v%s)", d.name, d.version)
	}

	// 初始化状态管理器
	statePath := filepath.Join(larkadapter.StateDir(), d.name+"_state.json")
	d.stateMgr = larkadapter.NewStateManager(statePath)
	d.lastCheck = d.stateMgr.GetLastCheck(d.name)
	if d.config.Enabled {
		log.Printf("[State] Last check: %v", d.lastCheck)
	}

	// 初始化WebSocket客户端
	wsAddr := os.Getenv("WS_ADDR")
	if wsAddr == "" {
		wsPort := settings.Service.WSPort
		if wsPort == 0 {
			wsPort = 8765
		}
		wsAddr = fmt.Sprintf("localhost:%d", wsPort)
	}

	// 将配置转换为ws.DetectorConfig
	wsConfig := ws.DetectorConfig{
		Enabled:           d.config.Enabled,
		Interval:          d.config.Interval.String(),
		BurstInterval:     d.config.BurstInterval.String(),
		BurstTimeout:      d.config.BurstTimeout.String(),
		HeartbeatInterval: d.config.HeartbeatInterval.String(),
	}
	d.wsClient = ws.NewClient(wsAddr, d.name, d.version, wsConfig)

	if d.config.Enabled {
		log.Printf("[WebSocket] Server: %s", wsAddr)
	}

	return nil
}

// getDetectorConfig 从总配置中获取当前检测器的配置
func (d *BaseDetector) getDetectorConfig(settings *config.Settings) config.DetectorConfig {
	switch d.name {
	case "lark_im":
		return settings.Detectors.LarkIM
	case "lark_doc":
		return settings.Detectors.LarkDoc
	case "lark_wiki":
		return settings.Detectors.LarkWiki
	case "lark_calendar":
		return settings.Detectors.LarkCalendar
	case "lark_task":
		return settings.Detectors.LarkTask
	case "lark_vc":
		return settings.Detectors.LarkVC
	case "lark_contact":
		return settings.Detectors.LarkContact
	default:
		return settings.Detectors.LarkIM
	}
}

// Run 运行检测器主循环
func (d *BaseDetector) Run() {
	// 连接WebSocket
	if err := d.wsClient.Connect(); err != nil {
		log.Printf("[WebSocket] Connect failed: %v (will work in standalone mode)", err)
	}

	// 启动信号监听
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动检测循环
	d.stopWg.Add(1)
	if d.config.Enabled {
		if d.useLongConn {
			if lcDetector, ok := d.detector.(LongConnDetector); ok {
				log.Printf("[%s] Detector enabled, starting long connection mode", d.name)
				go d.longConnLoop(lcDetector)
			} else {
				log.Printf("[%s] Detector does not support long connection, falling back to polling", d.name)
				go d.detectionLoop()
			}
		} else {
			log.Printf("[%s] Detector enabled, starting detection loop", d.name)
			go d.detectionLoop()
		}
	} else {
		log.Printf("[%s] Detector disabled, only heartbeat will be sent", d.name)
		go d.heartbeatOnlyLoop()
	}

	// 等待信号
	<-sigChan
	log.Printf("[%s] Received shutdown signal", d.name)

	// 停止
	d.Stop()
}

// Stop 停止检测器
func (d *BaseDetector) Stop() {
	log.Printf("[%s] Shutting down...", d.name)
	close(d.stopChan)
	d.stopWg.Wait()
	d.wsClient.Disconnect()
	log.Printf("[%s] Stopped", d.name)
}

// detectionLoop 检测循环
func (d *BaseDetector) detectionLoop() {
	defer d.stopWg.Done()

	for {
		select {
		case <-d.stopChan:
			return
		default:
		}

		// 执行检测
		hasChanges := d.doSingleDetection()

		// 计算下次检测间隔
		var nextInterval time.Duration
		if d.inBurstMode {
			if time.Since(d.lastChangeTime) > d.config.BurstTimeout {
				log.Printf("[%s] No changes for %v, exiting burst mode", d.name, d.config.BurstTimeout)
				d.inBurstMode = false
				nextInterval = d.config.Interval
			} else {
				nextInterval = d.config.BurstInterval
			}
		} else {
			nextInterval = d.config.Interval
		}

		// 检测到变化，进入或保持突发模式
		if hasChanges {
			log.Printf("[%s] Changes detected, entering burst mode", d.name)
			d.inBurstMode = true
			d.lastChangeTime = time.Now()
			nextInterval = d.config.BurstInterval
		}

		// 更新WebSocket客户端状态
		d.wsClient.UpdateDetectorState(d.lastCheck, d.lastChangeTime, d.inBurstMode)

		// 等待下次检测
		if nextInterval > 0 {
			select {
			case <-time.After(nextInterval):
			case <-d.stopChan:
				return
			}
		}
	}
}

// doSingleDetection 执行单次检测
func (d *BaseDetector) doSingleDetection() bool {
	// 执行检测
	result, err := d.detector.Detect(d.lastCheck)
	if err != nil {
		log.Printf("[%s] Detection failed: %v", d.name, err)
		return false
	}

	detectTime := time.Now()
	d.lastCheck = detectTime
	_ = d.stateMgr.UpdateLastCheck(d.detector.Name(), detectTime)

	// 处理结果
	if !result.HasChanges {
		return false
	}

	log.Printf("[%s] Detected %d changes", d.name, len(result.Changes))
	for i, change := range result.Changes {
		log.Printf("[%s] Change %d: %s [%s]",
			d.name, i+1, change.Type, change.Summary)
	}

	// 如果连接了WebSocket，发送结果
	if d.wsClient != nil {
		// 转换为ws格式
		wsChanges := make([]ws.ChangeItem, len(result.Changes))
		for i, c := range result.Changes {
			wsChanges[i] = ws.ChangeItem{
				Type:       c.Type,
				EntityType: c.EntityType,
				EntityID:   c.EntityID,
				Summary:    c.Summary,
				Timestamp:  c.Timestamp,
			}
		}

		wsResult := ws.DetectResult{
			Source:     result.Source,
			HasChanges: result.HasChanges,
			DetectedAt: detectTime.Format(time.RFC3339),
			LastCheck:  d.lastCheck.Format(time.RFC3339),
			Changes:    wsChanges,
		}

		if err := d.wsClient.SendDetectResult(wsResult); err != nil {
			log.Printf("[%s] Failed to send result via WebSocket: %v", d.name, err)
		}
	}

	return true
}

func boolToModeStr(burst bool) string {
	if burst {
		return "BURST"
	}
	return "normal"
}

// heartbeatOnlyLoop 仅发送心跳的循环（检测器禁用时使用）
func (d *BaseDetector) heartbeatOnlyLoop() {
	defer d.stopWg.Done()

	heartbeatInterval := d.config.HeartbeatInterval
	if heartbeatInterval <= 0 {
		heartbeatInterval = 30 * time.Second
	}

	for {
		select {
		case <-d.stopChan:
			return
		case <-time.After(heartbeatInterval):
			d.wsClient.UpdateDetectorState(d.lastCheck, d.lastChangeTime, false)
		}
	}
}

// longConnLoop 长连接模式循环
func (d *BaseDetector) longConnLoop(detector LongConnDetector) {
	defer d.stopWg.Done()

	handler := func(result *larkadapter.DetectResult) error {
		detectTime := time.Now()
		d.lastCheck = detectTime
		d.lastChangeTime = detectTime
		d.inBurstMode = true
		_ = d.stateMgr.UpdateLastCheck(d.detector.Name(), detectTime)
		_ = d.stateMgr.UpdateLastDetected(d.detector.Name(), detectTime)

		log.Printf("[%s] Detected %d changes via long conn", d.name, len(result.Changes))
		for i, change := range result.Changes {
			log.Printf("[%s] Change %d: %s [%s]", d.name, i+1, change.Type, change.Summary)
		}

		// 发送结果到 WebSocket
		if d.wsClient != nil {
			wsChanges := make([]ws.ChangeItem, len(result.Changes))
			for i, c := range result.Changes {
				wsChanges[i] = ws.ChangeItem{
					Type:       c.Type,
					EntityType: c.EntityType,
					EntityID:   c.EntityID,
					Summary:    c.Summary,
					Timestamp:  c.Timestamp,
				}
			}

			wsResult := ws.DetectResult{
				Source:     result.Source,
				HasChanges: result.HasChanges,
				DetectedAt: detectTime.Format(time.RFC3339),
				LastCheck:  d.lastCheck.Format(time.RFC3339),
				Changes:    wsChanges,
			}

			if err := d.wsClient.SendDetectResult(wsResult); err != nil {
				log.Printf("[%s] Failed to send result via WebSocket: %v", d.name, err)
			}
		}

		_ = larkadapter.SaveDetectResult(result)
		return nil
	}

	// 启动心跳
	go func() {
		heartbeatInterval := d.config.HeartbeatInterval
		if heartbeatInterval <= 0 {
			heartbeatInterval = 30 * time.Second
		}
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-d.stopChan:
				return
			case <-ticker.C:
				d.wsClient.UpdateDetectorState(d.lastCheck, d.lastChangeTime, d.inBurstMode)
				// 检查是否需要退出突发模式
				if d.inBurstMode && time.Since(d.lastChangeTime) > d.config.BurstTimeout {
					log.Printf("[%s] No changes for %v, exiting burst mode", d.name, d.config.BurstTimeout)
					d.inBurstMode = false
				}
			}
		}
	}()

	// 启动长连接
	if err := detector.StartLongConn(handler); err != nil {
		log.Printf("[%s] Failed to start long connection: %v", d.name, err)
		return
	}

	// 等待停止信号
	<-d.stopChan
}
