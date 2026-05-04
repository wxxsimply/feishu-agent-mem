package ws

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// upgrader WebSocket 升级配置
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// 允许所有来源，实际使用时应根据需要限制
		return true
	},
}

// Server WebSocket 服务端
type Server struct {
	port          int
	clients       map[string]*ClientConn
	clientsMutex  sync.RWMutex
	state         *GlobalState
	stateMutex    sync.RWMutex
	stopChan      chan struct{}
	stopWg        sync.WaitGroup
}

// ClientConn 单个检测器客户端连接
type ClientConn struct {
	conn           *websocket.Conn
	detectorName   string
	mutex          sync.Mutex
	lastHeartbeat  time.Time
	state          string
}

// NewServer 创建 WebSocket 服务端
func NewServer(port int) *Server {
	return &Server{
		port:        port,
		clients:     make(map[string]*ClientConn),
		state:       &GlobalState{Detectors: make(map[string]DetectorGlobalState)},
		stopChan:    make(chan struct{}),
	}
}

// Start 启动 WebSocket 服务端
func (s *Server) Start() error {
	log.Printf("[WebSocket] Starting server on port %d", s.port)

	http.HandleFunc("/ws", s.handleWebSocket)

	go func() {
		addr := fmt.Sprintf(":%d", s.port)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Printf("[WebSocket] Server stopped: %v", err)
		}
	}()

	// 启动状态清理协程
	s.stopWg.Add(1)
	go s.cleanupLoop()

	return nil
}

// Stop 停止 WebSocket 服务端
func (s *Server) Stop() {
	log.Println("[WebSocket] Stopping server...")

	close(s.stopChan)

	// 关闭所有客户端连接
	s.clientsMutex.Lock()
	for name, client := range s.clients {
		log.Printf("[WebSocket] Closing connection to %s", name)
		client.mutex.Lock()
		client.conn.Close()
		client.mutex.Unlock()
		delete(s.clients, name)
	}
	s.clientsMutex.Unlock()

	s.stopWg.Wait()
	log.Println("[WebSocket] Server stopped")
}

// handleWebSocket 处理 WebSocket 连接
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WebSocket] Failed to upgrade: %v", err)
		return
	}

	log.Println("[WebSocket] New client connected")
	defer func() {
		conn.Close()
	}()

	// 初始状态：等待注册
	client := &ClientConn{
		conn:          conn,
		state:         "pending",
		lastHeartbeat: time.Now(),
	}

	// 处理消息
	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[WebSocket] Read error: %v", err)
			s.handleDisconnect(client)
			return
		}

		msg, err := DecodeMessage(msgBytes)
		if err != nil {
			log.Printf("[WebSocket] Failed to decode message: %v", err)
			continue
		}

		if client.state == "pending" {
			if msg.Type != MsgTypeRegister {
				log.Printf("[WebSocket] Client not registered yet, ignoring message: %s", msg.Type)
				continue
			}
			s.handleRegister(client, msg)
		} else {
			s.handleClientMessage(client, msg)
		}
	}
}

// handleRegister 处理检测器注册
func (s *Server) handleRegister(client *ClientConn, msg *Message) {
	var registerMsg RegisterMessage
	if err := DecodeData(msg.Data, &registerMsg); err != nil {
		log.Printf("[WebSocket] Failed to decode register message: %v", err)
		return
	}

	client.detectorName = registerMsg.DetectorName
	client.state = "registered"

	log.Printf("[WebSocket] Detector registered: %s (v%s)", registerMsg.DetectorName, registerMsg.Version)

	// 添加到客户端列表
	s.clientsMutex.Lock()
	s.clients[client.detectorName] = client
	s.clientsMutex.Unlock()

	// 初始化全局状态
	s.stateMutex.Lock()
	s.state.Detectors[client.detectorName] = DetectorGlobalState{
		State:         "running",
		LastHeartbeat: time.Now().Format(time.RFC3339),
		LastChange:    "",
		TotalChanges:  0,
		TotalErrors:   0,
		Config:        registerMsg.Config,
	}
	s.stateMutex.Unlock()

	// 发送注册确认（心跳响应）
	ackMsg, err := NewMessage(MsgTypeHeartbeatAck, HeartbeatAckMessage{
		ServerTime: time.Now().Unix(),
		Status:     "registered",
	})
	if err == nil {
		s.sendToClient(client, ackMsg)
	}

	// 发送初始状态同步
	s.sendStateSync(client)
}

// handleClientMessage 处理已注册客户端的消息
func (s *Server) handleClientMessage(client *ClientConn, msg *Message) {
	switch msg.Type {
	case MsgTypeHeartbeat:
		s.handleHeartbeat(client, msg)
	case MsgTypeDetectResult:
		s.handleDetectResult(client, msg)
	default:
		log.Printf("[WebSocket] Unknown message type: %s", msg.Type)
	}
}

// handleHeartbeat 处理心跳
func (s *Server) handleHeartbeat(client *ClientConn, msg *Message) {
	var heartbeatMsg HeartbeatMessage
	if err := DecodeData(msg.Data, &heartbeatMsg); err != nil {
		log.Printf("[WebSocket] Failed to decode heartbeat: %v", err)
		return
	}

	client.lastHeartbeat = time.Now()
	log.Printf("[WebSocket] Heartbeat from %s (state=%s)", client.detectorName, heartbeatMsg.Status.State)

	// 更新全局状态
	s.stateMutex.Lock()
	if gs, exists := s.state.Detectors[client.detectorName]; exists {
		gs.State = heartbeatMsg.Status.State
		gs.LastHeartbeat = time.Now().Format(time.RFC3339)
		if heartbeatMsg.Status.LastChange != "" {
			gs.LastChange = heartbeatMsg.Status.LastChange
		}
		s.state.Detectors[client.detectorName] = gs
	}
	s.stateMutex.Unlock()

	// 发送心跳响应
	ackMsg, err := NewMessage(MsgTypeHeartbeatAck, HeartbeatAckMessage{
		ServerTime: time.Now().Unix(),
		Status:     "ok",
	})
	if err == nil {
		s.sendToClient(client, ackMsg)
	}
}

// handleDetectResult 处理检测结果
func (s *Server) handleDetectResult(client *ClientConn, msg *Message) {
	var detectMsg DetectResultMessage
	if err := DecodeData(msg.Data, &detectMsg); err != nil {
		log.Printf("[WebSocket] Failed to decode detect result: %v", err)
		return
	}

	log.Printf("[WebSocket] Received detect result from %s: hasChanges=%v, %d changes",
		detectMsg.DetectorName, detectMsg.Result.HasChanges, len(detectMsg.Result.Changes))

	// 更新全局状态
	s.stateMutex.Lock()
	if gs, exists := s.state.Detectors[client.detectorName]; exists {
		if detectMsg.Result.HasChanges {
			gs.TotalChanges += int64(len(detectMsg.Result.Changes))
			gs.LastChange = time.Now().Format(time.RFC3339)
		}
		gs.LastHeartbeat = time.Now().Format(time.RFC3339)
		s.state.Detectors[client.detectorName] = gs
	}
	s.stateMutex.Unlock()
}

// handleDisconnect 处理客户端断开
func (s *Server) handleDisconnect(client *ClientConn) {
	if client.detectorName == "" {
		return
	}

	log.Printf("[WebSocket] Detector disconnected: %s", client.detectorName)

	// 更新状态为 disconnected
	s.stateMutex.Lock()
	if gs, exists := s.state.Detectors[client.detectorName]; exists {
		gs.State = "disconnected"
		s.state.Detectors[client.detectorName] = gs
	}
	s.stateMutex.Unlock()

	// 从客户端列表中移除
	s.clientsMutex.Lock()
	delete(s.clients, client.detectorName)
	s.clientsMutex.Unlock()
}

// sendToClient 发送消息到客户端
func (s *Server) sendToClient(client *ClientConn, msg *Message) error {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	bytes, err := msg.Encode()
	if err != nil {
		return err
	}
	return client.conn.WriteMessage(websocket.TextMessage, bytes)
}

// sendStateSync 发送状态同步到客户端
func (s *Server) sendStateSync(client *ClientConn) {
	s.stateMutex.RLock()
	defer s.stateMutex.RUnlock()

	syncMsg, err := NewMessage(MsgTypeStateSync, StateSyncMessage{
		Timestamp:   time.Now().Unix(),
		GlobalState: *s.state,
	})
	if err != nil {
		log.Printf("[WebSocket] Failed to create state sync message: %v", err)
		return
	}
	s.sendToClient(client, syncMsg)
}

// cleanupLoop 清理长时间无心跳的客户端
func (s *Server) cleanupLoop() {
	defer s.stopWg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.checkClientTimeouts()
		}
	}
}

// checkClientTimeouts 检查客户端超时
func (s *Server) checkClientTimeouts() {
	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()

	timeoutDuration := 60 * time.Second
	now := time.Now()

	for name, client := range s.clients {
		if now.Sub(client.lastHeartbeat) > timeoutDuration {
			log.Printf("[WebSocket] Detector %s heartbeat timeout, closing connection", name)
			client.mutex.Lock()
			client.conn.Close()
			client.mutex.Unlock()
			delete(s.clients, name)

			// 更新状态
			s.stateMutex.Lock()
			if gs, exists := s.state.Detectors[name]; exists {
				gs.State = "disconnected"
				s.state.Detectors[name] = gs
			}
			s.stateMutex.Unlock()
		}
	}
}

// GetGlobalState 获取全局状态
func (s *Server) GetGlobalState() GlobalState {
	s.stateMutex.RLock()
	defer s.stateMutex.RUnlock()

	// 返回副本
	stateCopy := GlobalState{Detectors: make(map[string]DetectorGlobalState)}
	for k, v := range s.state.Detectors {
		stateCopy.Detectors[k] = v
	}
	return stateCopy
}

// SendControl 发送控制命令到指定检测器
func (s *Server) SendControl(detectorName string, command string, payload map[string]interface{}) error {
	s.clientsMutex.RLock()
	client, exists := s.clients[detectorName]
	s.clientsMutex.RUnlock()

	if !exists {
		return fmt.Errorf("detector %s not connected", detectorName)
	}

	controlMsg, err := NewMessage(MsgTypeControl, ControlMessage{
		Command: command,
		Payload: payload,
	})
	if err != nil {
		return err
	}

	return s.sendToClient(client, controlMsg)
}

// DetectResultChannel 用于传递检测结果给主程序（待扩展）
func (s *Server) DetectResultChannel() <-chan DetectResultMessage {
	// TODO: 实现消息通道
	return nil
}
