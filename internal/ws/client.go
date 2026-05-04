package ws

import (
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client WebSocket 客户端（供检测器使用）
type Client struct {
	serverAddr   string
	detectorName string
	version      string
	conn         *websocket.Conn
	mutex        sync.Mutex
	connected    bool
	config       DetectorConfig
	lastCheck    time.Time
	lastChange   time.Time
	inBurstMode  bool
	stopChan     chan struct{}
	stopWg       sync.WaitGroup
	handlers     map[string]func(data []byte)
}

// NewClient 创建 WebSocket 客户端
func NewClient(serverAddr string, detectorName string, version string, config DetectorConfig) *Client {
	return &Client{
		serverAddr:   serverAddr,
		detectorName: detectorName,
		version:      version,
		config:       config,
		stopChan:     make(chan struct{}),
		handlers:     make(map[string]func(data []byte)),
	}
}

// Connect 连接到 WebSocket 服务器
func (c *Client) Connect() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.connected {
		log.Printf("[WebSocket Client] Already connected")
		return nil
	}

	u := url.URL{Scheme: "ws", Host: c.serverAddr, Path: "/ws"}
	log.Printf("[WebSocket Client] Connecting to %s", u.String())

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("dial error: %w", err)
	}

	c.conn = conn
	c.connected = true

	// 发送注册消息
	registerMsg, err := NewMessage(MsgTypeRegister, RegisterMessage{
		DetectorName: c.detectorName,
		Version:      c.version,
		Config:       c.config,
	})
	if err != nil {
		return fmt.Errorf("create register message error: %w", err)
	}

	if err := c.sendRaw(registerMsg); err != nil {
		return fmt.Errorf("send register message error: %w", err)
	}

	log.Printf("[WebSocket Client] Connected and registered as %s", c.detectorName)

	// 启动消息处理循环
	c.stopWg.Add(1)
	go c.messageLoop()

	// 启动心跳发送循环
	c.stopWg.Add(1)
	go c.heartbeatLoop()

	return nil
}

// Disconnect 断开连接
func (c *Client) Disconnect() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.connected {
		return
	}

	log.Printf("[WebSocket Client] Disconnecting %s", c.detectorName)
	close(c.stopChan)
	c.stopWg.Wait()

	if c.conn != nil {
		c.conn.Close()
	}
	c.connected = false
}

// SendDetectResult 发送检测结果
func (c *Client) SendDetectResult(result DetectResult) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.connected {
		return fmt.Errorf("not connected")
	}

	detectMsg, err := NewMessage(MsgTypeDetectResult, DetectResultMessage{
		DetectorName: c.detectorName,
		Result:       result,
	})
	if err != nil {
		return err
	}

	return c.sendRaw(detectMsg)
}

// SetHandler 设置消息处理器
func (c *Client) SetHandler(msgType string, handler func(data []byte)) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.handlers[msgType] = handler
}

// messageLoop 接收和处理服务端消息
func (c *Client) messageLoop() {
	defer c.stopWg.Done()

	for {
		select {
		case <-c.stopChan:
			return
		default:
		}

		c.mutex.Lock()
		conn := c.conn
		c.mutex.Unlock()

		if conn == nil {
			time.Sleep(1 * time.Second)
			continue
		}

		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[WebSocket Client] Read error: %v", err)
			// 等待后尝试重连
			time.Sleep(5 * time.Second)
			go c.tryReconnect()
			return
		}

		msg, err := DecodeMessage(msgBytes)
		if err != nil {
			log.Printf("[WebSocket Client] Failed to decode message: %v", err)
			continue
		}

		c.mutex.Lock()
		handler, exists := c.handlers[msg.Type]
		c.mutex.Unlock()

		if exists {
			handler(msg.Data)
		} else {
			// 默认处理一些消息类型
			switch msg.Type {
			case MsgTypeHeartbeatAck:
				// 心跳响应，无需处理
			case MsgTypeControl:
				var controlMsg ControlMessage
				if err := DecodeData(msg.Data, &controlMsg); err == nil {
					log.Printf("[WebSocket Client] Received control: %s", controlMsg.Command)
				}
			case MsgTypeStateSync:
				var stateMsg StateSyncMessage
				if err := DecodeData(msg.Data, &stateMsg); err == nil {
					log.Printf("[WebSocket Client] Received state sync")
				}
			}
		}
	}
}

// heartbeatLoop 发送心跳
func (c *Client) heartbeatLoop() {
	defer c.stopWg.Done()

	interval, err := time.ParseDuration(c.config.HeartbeatInterval)
	if err != nil {
		interval = 15 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.sendHeartbeat()
		}
	}
}

// sendHeartbeat 发送单次心跳
func (c *Client) sendHeartbeat() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.connected {
		return
	}

	heartbeatMsg, err := NewMessage(MsgTypeHeartbeat, HeartbeatMessage{
		DetectorName: c.detectorName,
		Version:      c.version,
		Status: DetectorStatus{
			State:         "running",
			LastCheck:     c.lastCheck.Format(time.RFC3339),
			LastChange:    c.lastChange.Format(time.RFC3339),
			InBurstMode:   c.inBurstMode,
		},
	})
	if err != nil {
		log.Printf("[WebSocket Client] Failed to create heartbeat: %v", err)
		return
	}

	if err := c.sendRaw(heartbeatMsg); err != nil {
		log.Printf("[WebSocket Client] Failed to send heartbeat: %v", err)
	}
}

// tryReconnect 尝试重连
func (c *Client) tryReconnect() {
	log.Printf("[WebSocket Client] Attempting reconnect...")
	for i := 0; i < 10; i++ {
		select {
		case <-c.stopChan:
			return
		default:
		}

		time.Sleep(3 * time.Second)
		if err := c.Connect(); err == nil {
			log.Printf("[WebSocket Client] Reconnected successfully")
			return
		}
		log.Printf("[WebSocket Client] Reconnect attempt %d failed", i+1)
	}
	log.Printf("[WebSocket Client] Giving up reconnect attempts")
}

// sendRaw 发送原始消息（内部使用）
func (c *Client) sendRaw(msg *Message) error {
	bytes, err := msg.Encode()
	if err != nil {
		return err
	}
	return c.conn.WriteMessage(websocket.TextMessage, bytes)
}

// UpdateDetectorState 更新检测器状态（用于心跳）
func (c *Client) UpdateDetectorState(lastCheck time.Time, lastChange time.Time, inBurstMode bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.lastCheck = lastCheck
	c.lastChange = lastChange
	c.inBurstMode = inBurstMode
}
