// Package cdp 通过调试端口直连 Chrome/Edge：
// 建立页面级 WebSocket 通道，封装 Runtime.evaluate / DOM.setFileInputFiles 等
// 常用调用，并在通道断开时自动重连。
package cdp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Target struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	URL                  string `json:"url"`
	Title                string `json:"title"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

type callResult struct {
	result json.RawMessage
	err    error
}

type pageConn struct {
	ws      *websocket.Conn
	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan callResult
	dead    chan struct{}
	once    sync.Once
}

func (pc *pageConn) failAll(err error) {
	pc.once.Do(func() { close(pc.dead) })
	pc.mu.Lock()
	for id, ch := range pc.pending {
		ch <- callResult{err: err}
		delete(pc.pending, id)
	}
	pc.mu.Unlock()
}

func (pc *pageConn) readLoop() {
	for {
		_, data, err := pc.ws.ReadMessage()
		if err != nil {
			pc.failAll(fmt.Errorf("CDP 连接断开: %w", err))
			return
		}
		var msg struct {
			ID     *int64          `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int64  `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(data, &msg) != nil || msg.ID == nil {
			continue // 页面事件，忽略
		}
		pc.mu.Lock()
		ch, ok := pc.pending[*msg.ID]
		delete(pc.pending, *msg.ID)
		pc.mu.Unlock()
		if !ok {
			continue
		}
		switch {
		case msg.Error != nil:
			ch <- callResult{err: fmt.Errorf("CDP 错误 %d: %s", msg.Error.Code, msg.Error.Message)}
		default:
			ch <- callResult{result: msg.Result}
		}
	}
}

// Client 是一个指向调试端口的客户端，内部维护到目标页面的连接。
type Client struct {
	Base  string // http://127.0.0.1:9222
	urlRe *regexp.Regexp
	Logf  func(format string, args ...any)

	mu   sync.Mutex
	conn *pageConn
}

func New(port int, urlMatch string) (*Client, error) {
	re, err := regexp.Compile(urlMatch)
	if err != nil {
		return nil, err
	}
	return &Client{
		Base:  fmt.Sprintf("http://127.0.0.1:%d", port),
		urlRe: re,
		Logf:  func(string, ...any) {},
	}, nil
}

func (c *Client) logf(format string, args ...any) {
	if c.Logf != nil {
		c.Logf(format, args...)
	}
}

func (c *Client) ListTargets() ([]Target, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(c.Base + "/json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var targets []Target
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return nil, err
	}
	return targets, nil
}

// MatchTarget 判断 URL 是否匹配选择器配置的页面。
func (c *Client) MatchTarget(url string) bool {
	return c.urlRe.MatchString(url)
}

// EnsureConn 保证存在一条可用的页面级 CDP 连接。
func (c *Client) EnsureConn(ctx context.Context) error {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn != nil {
		select {
		case <-conn.dead:
		default:
			return nil
		}
	}

	targets, err := c.ListTargets()
	if err != nil {
		return fmt.Errorf("无法访问调试端口 (%s): %w", c.Base, err)
	}
	var target *Target
	for i := range targets {
		t := targets[i]
		if t.Type == "page" && c.urlRe.MatchString(t.URL) {
			target = &t
			break
		}
	}
	if target == nil {
		return fmt.Errorf("未找到匹配 %q 的页面标签", c.urlRe.String())
	}
	if target.WebSocketDebuggerURL == "" {
		return fmt.Errorf("目标页面没有 webSocketDebuggerUrl")
	}

	dialer := &websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	ws, _, err := dialer.DialContext(ctx, target.WebSocketDebuggerURL, nil)
	if err != nil {
		return fmt.Errorf("CDP WebSocket 连接失败: %w", err)
	}
	pc := &pageConn{
		ws:      ws,
		pending: make(map[int64]chan callResult),
		dead:    make(chan struct{}),
	}
	go pc.readLoop()
	c.mu.Lock()
	if c.conn != nil {
		_ = c.conn.ws.Close()
	}
	c.conn = pc
	c.mu.Unlock()
	c.logf("CDP 已连接: %s", target.URL)
	return nil
}

// Invalidate 丢弃当前连接，下次调用时重连。
func (c *Client) Invalidate() {
	c.mu.Lock()
	conn := c.conn
	c.conn = nil
	c.mu.Unlock()
	if conn != nil {
		_ = conn.ws.Close()
	}
}

// Call 执行一次 CDP 调用；通道断开或超时会自动重连并重试（最多 3 次）。
func (c *Client) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(500 * time.Millisecond):
			}
		}
		callCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		err := c.EnsureConn(callCtx)
		cancel()
		if err != nil {
			lastErr = err
			continue
		}
		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()
		callCtx2, cancel2 := context.WithTimeout(ctx, 4*time.Second)
		res, err := conn.call(callCtx2, method, params)
		cancel2()
		if err == nil {
			return res, nil
		}
		lastErr = err
		c.logf("CDP %s 失败(%v)，重连重试", method, err)
		c.Invalidate()
	}
	return nil, lastErr
}

func (pc *pageConn) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	pc.mu.Lock()
	pc.nextID++
	id := pc.nextID
	ch := make(chan callResult, 1)
	pc.pending[id] = ch
	pc.mu.Unlock()

	defer func() {
		pc.mu.Lock()
		delete(pc.pending, id)
		pc.mu.Unlock()
	}()

	payload, _ := json.Marshal(map[string]any{"id": id, "method": method, "params": params})
	_ = pc.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := pc.ws.WriteMessage(websocket.TextMessage, payload); err != nil {
		return nil, fmt.Errorf("发送 %s: %w", method, err)
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-pc.dead:
		return nil, fmt.Errorf("连接已断开")
	case r := <-ch:
		return r.result, r.err
	}
}

type evaluateResult struct {
	Result struct {
		Value json.RawMessage `json:"value"`
	} `json:"result"`
	ExceptionDetails *struct {
		Text      string `json:"text"`
		Exception struct {
			Description string `json:"description"`
		} `json:"exception"`
	} `json:"exceptionDetails"`
}

// Evaluate 执行 JS 表达式并按值返回结果。
func (c *Client) Evaluate(ctx context.Context, expression string) (json.RawMessage, error) {
	params := map[string]any{"expression": expression, "returnByValue": true}
	raw, err := c.Call(ctx, "Runtime.evaluate", params)
	if err != nil {
		return nil, err
	}
	var r evaluateResult
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	if r.ExceptionDetails != nil {
		return nil, fmt.Errorf("页面 JS 异常: %s %s", r.ExceptionDetails.Text, r.ExceptionDetails.Exception.Description)
	}
	return r.Result.Value, nil
}

// EvaluateObject 执行 JS 并返回远程对象 objectId（用于 DOM 节点）。
func (c *Client) EvaluateObject(ctx context.Context, expression string) (string, error) {
	params := map[string]any{"expression": expression, "returnByValue": false}
	raw, err := c.Call(ctx, "Runtime.evaluate", params)
	if err != nil {
		return "", err
	}
	var r struct {
		Result struct {
			ObjectID string `json:"objectId"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return "", err
	}
	if r.Result.ObjectID == "" {
		return "", fmt.Errorf("页面未返回对象句柄")
	}
	return r.Result.ObjectID, nil
}

// SetInputFiles 把本地文件塞进页面的 file input（等价 DOM.setFileInputFiles）。
func (c *Client) SetInputFiles(ctx context.Context, files []string) error {
	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for {
		if _, err := c.Call(ctx, "DOM.enable", nil); err != nil {
			return err
		}
		objectID, err := c.EvaluateObject(ctx, `document.querySelector('input[type="file"]')`)
		if err == nil {
			_, err = c.Call(ctx, "DOM.setFileInputFiles", map[string]any{"files": files, "objectId": objectID})
			return err
		}
		lastErr = err
		if time.Now().After(deadline) {
			return fmt.Errorf("等待 input[type=file] 超时: %w", lastErr)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
}

// SetFilesOnObject 对已取得的远程对象执行文件挂载。
func (c *Client) SetFilesOnObject(ctx context.Context, objectID string, files []string) error {
	if _, err := c.Call(ctx, "DOM.enable", nil); err != nil {
		return err
	}
	_, err := c.Call(ctx, "DOM.setFileInputFiles", map[string]any{"files": files, "objectId": objectID})
	return err
}

// BringToFront 把页面带到前台并启用焦点仿真。
func (c *Client) BringToFront(ctx context.Context) error {
	if _, err := c.Call(ctx, "Page.enable", nil); err != nil {
		return err
	}
	if _, err := c.Call(ctx, "Page.bringToFront", nil); err != nil {
		return err
	}
	_, err := c.Call(ctx, "Emulation.setFocusEmulationEnabled", map[string]any{"enabled": true})
	return err
}
