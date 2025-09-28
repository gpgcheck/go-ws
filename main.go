package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/net/proxy"
)

type WSClient struct {
	ID           int
	URL          string
	Conn         *websocket.Conn
	Done         chan struct{}
	Messages     chan []byte
	Errors       chan error
	Reconnect    bool
	MaxRetries   int
	RetryDelay   time.Duration
	RetryCount   int
	PingInterval time.Duration
	LastPingTime time.Time
	LastActivity time.Time
	PingCount    int
	SmartPing    bool
	ProxyURL     string
	closed       bool
	mu           sync.Mutex
}

// 日志级别
type LogLevel int

const (
	ERROR LogLevel = iota
	DEBUG
)

var currentLogLevel LogLevel = ERROR

// 日志函数
func logError(format string, v ...interface{}) {
	log.Printf("[ERROR] "+format, v...)
}

func logDebug(format string, v ...interface{}) {
	if currentLogLevel == DEBUG {
		log.Printf("[DEBUG] "+format, v...)
	}
}

type WSManager struct {
	clients           []*WSClient
	numClients        int
	url               string
	reconnect         bool
	maxRetries        int
	retryDelay        time.Duration
	ignoreMsg         bool
	pingInterval      time.Duration
	statusInterval    time.Duration
	smartPing         bool
	enableCompression bool
	fastConnect       bool
	proxyURL          string
	mu                sync.RWMutex
	wg                sync.WaitGroup
}

func NewWSManager(url string, numClients int, reconnect bool, maxRetries int, retryDelay time.Duration, ignoreMsg bool, pingInterval time.Duration, statusInterval time.Duration, smartPing bool, enableCompression bool, fastConnect bool, proxyURL string) *WSManager {
	return &WSManager{
		clients:           make([]*WSClient, 0, numClients),
		numClients:        numClients,
		url:               url,
		reconnect:         reconnect,
		maxRetries:        maxRetries,
		retryDelay:        retryDelay,
		ignoreMsg:         ignoreMsg,
		pingInterval:      pingInterval,
		statusInterval:    statusInterval,
		smartPing:         smartPing,
		enableCompression: enableCompression,
		fastConnect:       fastConnect,
		proxyURL:          proxyURL,
	}
}

func (m *WSManager) createClient(id int) *WSClient {
	return &WSClient{
		ID:           id,
		URL:          m.url,
		Done:         make(chan struct{}),
		Messages:     make(chan []byte, 100),
		Errors:       make(chan error, 10),
		Reconnect:    m.reconnect,
		MaxRetries:   m.maxRetries,
		RetryDelay:   m.retryDelay,
		RetryCount:   0,
		PingInterval: m.pingInterval,
		LastPingTime: time.Now(),
		LastActivity: time.Now(),
		PingCount:    0,
		SmartPing:    m.smartPing,
		ProxyURL:     m.proxyURL,
	}
}

func (c *WSClient) connect(enableCompression bool) error {
	u, err := url.Parse(c.URL)
	if err != nil {
		return fmt.Errorf("invalid URL: %v", err)
	}

	// 配置WebSocket拨号器
	dialer := websocket.DefaultDialer
	dialer.EnableCompression = enableCompression

	// 配置代理
	if c.ProxyURL != "" {
		proxyURL, err := url.Parse(c.ProxyURL)
		if err != nil {
			return fmt.Errorf("invalid proxy URL: %v", err)
		}

		// 根据代理类型选择不同的配置方式
		switch proxyURL.Scheme {
		case "socks5", "socks4":
			// SOCKS代理
			dialer.NetDial = func(network, addr string) (net.Conn, error) {
				dialer, err := proxy.SOCKS5("tcp", proxyURL.Host, nil, proxy.Direct)
				if err != nil {
					return nil, err
				}
				return dialer.Dial(network, addr)
			}
			logDebug("Client %d using SOCKS proxy: %s", c.ID, c.ProxyURL)
		case "http", "https":
			// HTTP代理
			dialer.Proxy = http.ProxyURL(proxyURL)
			logDebug("Client %d using HTTP proxy: %s", c.ID, c.ProxyURL)
		default:
			return fmt.Errorf("unsupported proxy scheme: %s", proxyURL.Scheme)
		}
	}

	// 建立WebSocket连接
	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}

	c.Conn = conn
	c.RetryCount = 0 // 重置重试计数
	c.LastActivity = time.Now()
	logDebug("Client %d connected to %s (compression: %t, proxy: %s)", c.ID, c.URL, enableCompression, c.ProxyURL)
	return nil
}

func (c *WSClient) reconnectWithRetry(m *WSManager) bool {
	if !c.Reconnect || c.RetryCount >= c.MaxRetries {
		return false
	}

	c.RetryCount++
	logDebug("Client %d attempting to reconnect (attempt %d/%d)...", c.ID, c.RetryCount, c.MaxRetries)

	// 等待重试延迟
	time.Sleep(c.RetryDelay)

	// 尝试重连
	if err := c.connect(m.enableCompression); err != nil {
		logError("Client %d reconnect failed: %v", c.ID, err)
		return c.reconnectWithRetry(m) // 递归重试
	}

	logDebug("Client %d reconnected successfully", c.ID)
	return true
}

func (c *WSClient) readMessages(m *WSManager) {
	defer close(c.Messages)
	defer close(c.Errors)

	for {
		select {
		case <-c.Done:
			return
		default:
			_, message, err := c.Conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					logError("Client %d connection lost: %v", c.ID, err)

					// 尝试重连
					if c.reconnectWithRetry(m) {
						continue // 重连成功，继续读取消息
					} else {
						c.Errors <- fmt.Errorf("connection lost and max retries exceeded: %v", err)
						return
					}
				}
				return
			}
			c.Messages <- message
		}
	}
}

func (c *WSClient) sendPing(m *WSManager) {
	ticker := time.NewTicker(c.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.Done:
			return
		case <-ticker.C:
			// 智能心跳：只在连接空闲时发送ping
			if c.SmartPing {
				// 如果最近有活动，跳过这次ping
				if time.Since(c.LastActivity) < c.PingInterval/2 {
					logDebug("Client %d skipping ping due to recent activity", c.ID)
					continue
				}
			}

			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				logError("Client %d ping failed: %v", c.ID, err)

				// 尝试重连
				if c.reconnectWithRetry(m) {
					continue // 重连成功，继续发送ping
				} else {
					c.Errors <- fmt.Errorf("ping failed and max retries exceeded: %v", err)
					return
				}
			} else {
				c.LastPingTime = time.Now()
				c.PingCount++
				logDebug("Client %d sent ping (count: %d)", c.ID, c.PingCount)
			}
		}
	}
}

func (c *WSClient) close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}
	c.closed = true

	close(c.Done)
	if c.Conn != nil {
		c.Conn.Close()
		logDebug("Client %d disconnected", c.ID)
	}
}

func (m *WSManager) startClient(client *WSClient) {
	defer m.wg.Done()

	// 连接WebSocket
	if err := client.connect(m.enableCompression); err != nil {
		logError("Client %d failed to connect: %v", client.ID, err)
		return
	}

	// 添加到管理器
	m.mu.Lock()
	m.clients = append(m.clients, client)
	m.mu.Unlock()

	// 启动消息读取和ping发送
	go client.readMessages(m)
	go client.sendPing(m)

	// 处理消息和错误
	for {
		select {
		case message := <-client.Messages:
			// 更新活动时间
			client.LastActivity = time.Now()

			if !m.ignoreMsg {
				logDebug("Client %d received: %s", client.ID, string(message))
			} else {
				// 忽略消息模式：静默处理，不打印任何日志
			}
		case err := <-client.Errors:
			if err != nil {
				logError("Client %d error: %v", client.ID, err)
			}
			client.close()
			return
		case <-client.Done:
			return
		}
	}
}

func (m *WSManager) startAllClients() {
	logDebug("Starting %d concurrent WebSocket connections to %s", m.numClients, m.url)

	for i := 0; i < m.numClients; i++ {
		client := m.createClient(i + 1)
		m.wg.Add(1)
		go m.startClient(client)

		// 根据连接数动态调整延迟（优化连接速度）
		if !m.fastConnect {
			delay := 10 * time.Millisecond // 减少基础延迟
			if m.numClients > 50 {
				delay = 20 * time.Millisecond
			}
			if m.numClients > 100 {
				delay = 50 * time.Millisecond
			}
			if m.numClients > 500 {
				delay = 100 * time.Millisecond
			}
			time.Sleep(delay)
		}
		// 快速连接模式：无延迟
	}
}

func (m *WSManager) stopAllClients() {
	logDebug("Stopping all clients...")

	m.mu.RLock()
	for _, client := range m.clients {
		client.close()
	}
	m.mu.RUnlock()

	m.wg.Wait()
	logDebug("All clients stopped")
}

func (m *WSManager) getActiveConnections() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clients)
}

// loadEnvFile 加载.env文件
func loadEnvFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 解析 KEY=VALUE 格式
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			os.Setenv(key, value)
		}
	}

	return scanner.Err()
}

// getEnvConfig 从环境变量获取配置
func getEnvConfig() (int, string, LogLevel, bool, int, time.Duration, bool, time.Duration, time.Duration, bool, bool, bool, string) {
	// 首先尝试加载.env文件
	if err := loadEnvFile(".env"); err != nil {
		logDebug("未找到.env文件或加载失败: %v", err)
	}
	// 从环境变量获取线程数，默认为10
	numClients := 10
	if clientsStr := os.Getenv("WS_CLIENTS"); clientsStr != "" {
		if clients, err := strconv.Atoi(clientsStr); err == nil && clients > 0 {
			numClients = clients
		}
	}

	// 从环境变量获取WebSocket地址，默认为example.com
	wsURL := os.Getenv("WS_URL")
	if wsURL == "" {
		wsURL = "wss://example.com/v1/ws/public"
	}

	// 从环境变量获取日志级别，默认为ERROR
	logLevel := ERROR
	if logLevelStr := os.Getenv("LOG_LEVEL"); logLevelStr != "" {
		switch strings.ToLower(logLevelStr) {
		case "debug":
			logLevel = DEBUG
		case "error":
			logLevel = ERROR
		}
	}

	// 从环境变量获取重连配置，默认为启用
	reconnect := true
	if reconnectStr := os.Getenv("WS_RECONNECT"); reconnectStr != "" {
		reconnect = strings.ToLower(reconnectStr) == "true"
	}

	// 从环境变量获取最大重试次数，默认为5
	maxRetries := 5
	if retriesStr := os.Getenv("WS_MAX_RETRIES"); retriesStr != "" {
		if retries, err := strconv.Atoi(retriesStr); err == nil && retries > 0 {
			maxRetries = retries
		}
	}

	// 从环境变量获取重试延迟，默认为5秒
	retryDelay := 5 * time.Second
	if delayStr := os.Getenv("WS_RETRY_DELAY"); delayStr != "" {
		if delay, err := strconv.Atoi(delayStr); err == nil && delay > 0 {
			retryDelay = time.Duration(delay) * time.Second
		}
	}

	// 从环境变量获取是否忽略消息，默认为false
	ignoreMsg := false
	if ignoreStr := os.Getenv("WS_IGNORE_MSG"); ignoreStr != "" {
		ignoreMsg = strings.ToLower(ignoreStr) == "true"
	}

	// 从环境变量获取心跳间隔，默认为120秒（进一步降低带宽占用）
	pingInterval := 120 * time.Second
	if pingStr := os.Getenv("WS_PING_INTERVAL"); pingStr != "" {
		if ping, err := strconv.Atoi(pingStr); err == nil && ping > 0 {
			pingInterval = time.Duration(ping) * time.Second
		}
	}

	// 从环境变量获取状态报告间隔，默认为30秒（降低日志输出）
	statusInterval := 30 * time.Second
	if statusStr := os.Getenv("WS_STATUS_INTERVAL"); statusStr != "" {
		if status, err := strconv.Atoi(statusStr); err == nil && status > 0 {
			statusInterval = time.Duration(status) * time.Second
		}
	}

	// 从环境变量获取智能心跳，默认为启用
	smartPing := true
	if smartStr := os.Getenv("WS_SMART_PING"); smartStr != "" {
		smartPing = strings.ToLower(smartStr) == "true"
	}

	// 从环境变量获取压缩支持，默认为禁用（获得更高带宽）
	enableCompression := false
	if compStr := os.Getenv("WS_ENABLE_COMPRESSION"); compStr != "" {
		enableCompression = strings.ToLower(compStr) == "true"
	}

	// 从环境变量获取快速连接，默认为启用
	fastConnect := true
	if fastStr := os.Getenv("WS_FAST_CONNECT"); fastStr != "" {
		fastConnect = strings.ToLower(fastStr) == "true"
	}

	// 从环境变量获取代理URL，默认为空（不使用代理）
	proxyURL := os.Getenv("WS_PROXY_URL")

	return numClients, wsURL, logLevel, reconnect, maxRetries, retryDelay, ignoreMsg, pingInterval, statusInterval, smartPing, enableCompression, fastConnect, proxyURL
}

func main() {
	var (
		numClients = flag.Int("clients", 0, "Number of concurrent WebSocket connections (overrides WS_CLIENTS env var)")
		wsURL      = flag.String("url", "", "WebSocket URL to connect to (overrides WS_URL env var)")
		logLevel   = flag.String("log", "", "Log level: error or debug (overrides LOG_LEVEL env var)")
		reconnect  = flag.Bool("reconnect", true, "Enable auto-reconnect (overrides WS_RECONNECT env var)")
		maxRetries = flag.Int("retries", 0, "Max retry attempts (overrides WS_MAX_RETRIES env var)")
		retryDelay = flag.Int("delay", 0, "Retry delay in seconds (overrides WS_RETRY_DELAY env var)")
		ignoreMsg  = flag.Bool("ignore-msg", false, "Ignore received messages (overrides WS_IGNORE_MSG env var)")
		proxyURL   = flag.String("proxy", "", "Proxy URL (overrides WS_PROXY_URL env var)")
	)
	flag.Parse()

	// 从环境变量获取配置
	envClients, envURL, envLogLevel, envReconnect, envMaxRetries, envRetryDelay, envIgnoreMsg, envPingInterval, envStatusInterval, envSmartPing, envEnableCompression, envFastConnect, envProxyURL := getEnvConfig()

	// 如果命令行参数为空，则使用环境变量
	if *numClients == 0 {
		*numClients = envClients
	}
	if *wsURL == "" {
		*wsURL = envURL
	}
	if *logLevel == "" {
		currentLogLevel = envLogLevel
	} else {
		switch strings.ToLower(*logLevel) {
		case "debug":
			currentLogLevel = DEBUG
		case "error":
			currentLogLevel = ERROR
		default:
			currentLogLevel = ERROR
		}
	}
	if !*reconnect {
		*reconnect = envReconnect
	}
	if *maxRetries == 0 {
		*maxRetries = envMaxRetries
	}
	if *retryDelay == 0 {
		*retryDelay = int(envRetryDelay.Seconds())
	}
	if !*ignoreMsg {
		*ignoreMsg = envIgnoreMsg
	}
	if *proxyURL == "" {
		*proxyURL = envProxyURL
	}

	// 根据日志级别设置日志输出
	if currentLogLevel == DEBUG {
		log.SetOutput(os.Stderr)
	} else {
		// 抑制WebSocket库的噪音日志，但保留错误日志
		log.SetOutput(io.Discard)
	}

	// 显示当前配置
	fmt.Println("=== WebSocket 并发客户端 ===")
	fmt.Printf("配置信息:\n")
	fmt.Printf("  并发连接数: %d\n", *numClients)
	fmt.Printf("  WebSocket URL: %s\n", *wsURL)
	fmt.Printf("  日志级别: %s\n", func() string {
		if currentLogLevel == DEBUG {
			return "debug"
		}
		return "error"
	}())
	fmt.Printf("  自动重连: %t\n", *reconnect)
	fmt.Printf("  最大重试次数: %d\n", *maxRetries)
	fmt.Printf("  重试延迟: %d秒\n", *retryDelay)
	fmt.Printf("  忽略消息: %t\n", *ignoreMsg)
	fmt.Printf("  心跳间隔: %d秒\n", int(envPingInterval.Seconds()))
	fmt.Printf("  状态报告间隔: %d秒\n", int(envStatusInterval.Seconds()))
	fmt.Printf("  智能心跳: %t\n", envSmartPing)
	fmt.Printf("  启用压缩: %t\n", envEnableCompression)
	fmt.Printf("  快速连接: %t\n", envFastConnect)
	fmt.Printf("  代理服务器: %s\n", *proxyURL)

	fmt.Println()

	// 创建WebSocket管理器
	manager := NewWSManager(*wsURL, *numClients, *reconnect, *maxRetries, time.Duration(*retryDelay)*time.Second, *ignoreMsg, envPingInterval, envStatusInterval, envSmartPing, envEnableCompression, envFastConnect, *proxyURL)

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动所有客户端
	manager.startAllClients()

	// 定期报告状态
	statusTicker := time.NewTicker(manager.statusInterval)
	defer statusTicker.Stop()

	// 主循环
	for {
		select {
		case <-sigChan:
			logDebug("Received interrupt signal, shutting down...")
			manager.stopAllClients()
			return
		case <-statusTicker.C:
			active := manager.getActiveConnections()
			logDebug("Status: %d/%d active connections", active, *numClients)
		}
	}
}
