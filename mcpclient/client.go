// Package mcpclient fala com servidores MCP externos (stdio e HTTP) e os
// expõe como ferramentas para o modelo.
//
// Os servidores vêm só da configuração do próprio Amazzo (ver ConfigPaths);
// configs de outras ferramentas não são lidas.
package mcpclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const protocolVersion = "2025-03-26"

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return fmt.Sprintf("mcp %d: %s", e.Code, e.Message) }

// transport envia uma chamada e devolve o resultado.
type transport interface {
	call(ctx context.Context, method string, params interface{}) (json.RawMessage, error)
	notify(method string, params interface{}) error
	close()
}

// ---------------------------------------------------------------- stdio

type stdioConn struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	wmu     sync.Mutex
	nextID  int64
	mu      sync.Mutex
	pending map[int64]chan rpcMessage
	done    chan struct{}
	logFile *os.File
}

func startStdio(command []string, env map[string]string, dir, logPath string) (*stdioConn, error) {
	if len(command) == 0 {
		return nil, errors.New("comando vazio")
	}
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	// Grupo próprio: matar o servidor mata também os filhos dele (JVM, node).
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	c := &stdioConn{cmd: cmd, stdin: stdin, pending: map[int64]chan rpcMessage{}, done: make(chan struct{})}
	if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600); err == nil {
		c.logFile = f
		cmd.Stderr = f
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	go c.readLoop(stdout)
	go func() { _ = cmd.Wait(); close(c.done) }()
	return c, nil
}

func (c *stdioConn) readLoop(r io.Reader) {
	br := bufio.NewReaderSize(r, 1<<20)
	for {
		line, err := br.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			var msg rpcMessage
			if json.Unmarshal(line, &msg) == nil {
				c.dispatch(msg)
			}
		}
		if err != nil {
			c.failAll(err)
			return
		}
	}
}

func (c *stdioConn) dispatch(msg rpcMessage) {
	// Pedido do servidor (ping, roots/list...): responde para ele não travar.
	if msg.Method != "" && len(msg.ID) > 0 {
		reply := map[string]interface{}{"jsonrpc": "2.0", "id": json.RawMessage(msg.ID)}
		if msg.Method == "ping" {
			reply["result"] = map[string]interface{}{}
		} else {
			reply["error"] = map[string]interface{}{"code": -32601, "message": "não suportado"}
		}
		c.write(reply)
		return
	}
	if msg.Method != "" {
		return // notificação
	}
	var id int64
	if json.Unmarshal(msg.ID, &id) != nil {
		return
	}
	c.mu.Lock()
	ch := c.pending[id]
	delete(c.pending, id)
	c.mu.Unlock()
	if ch != nil {
		ch <- msg
	}
}

func (c *stdioConn) failAll(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, ch := range c.pending {
		ch <- rpcMessage{Error: &rpcError{Code: -1, Message: "servidor encerrou: " + err.Error()}}
		delete(c.pending, id)
	}
}

func (c *stdioConn) write(v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_, err = c.stdin.Write(append(b, '\n'))
	return err
}

func (c *stdioConn) call(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	id := atomic.AddInt64(&c.nextID, 1)
	ch := make(chan rpcMessage, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	req := map[string]interface{}{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		req["params"] = params
	}
	if err := c.write(req); err != nil {
		return nil, err
	}
	select {
	case msg := <-ch:
		if msg.Error != nil {
			return nil, msg.Error
		}
		return msg.Result, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		// Avisa o servidor para parar de trabalhar nisso.
		_ = c.notify("notifications/cancelled", map[string]interface{}{"requestId": id, "reason": "cancelado"})
		return nil, ctx.Err()
	case <-c.done:
		return nil, errors.New("o servidor MCP encerrou")
	}
}

func (c *stdioConn) notify(method string, params interface{}) error {
	req := map[string]interface{}{"jsonrpc": "2.0", "method": method}
	if params != nil {
		req["params"] = params
	}
	return c.write(req)
}

func (c *stdioConn) close() {
	_ = c.stdin.Close()
	if c.cmd.Process != nil {
		pgid := c.cmd.Process.Pid
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
		select {
		case <-c.done:
		case <-time.After(2 * time.Second):
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		}
	}
	if c.logFile != nil {
		_ = c.logFile.Close()
	}
}

// ---------------------------------------------------------------- HTTP

type httpConn struct {
	url     string
	headers map[string]string
	client  *http.Client
	nextID  int64
	session atomic.Value // string
}

func newHTTP(url string, headers map[string]string) *httpConn {
	return &httpConn{url: url, headers: headers, client: &http.Client{}}
}

func (h *httpConn) post(ctx context.Context, body interface{}) (*http.Response, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}
	if s, _ := h.session.Load().(string); s != "" {
		req.Header.Set("Mcp-Session-Id", s)
	}
	return h.client.Do(req)
}

func (h *httpConn) call(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	id := atomic.AddInt64(&h.nextID, 1)
	req := map[string]interface{}{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		req["params"] = params
	}
	resp, err := h.post(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if s := resp.Header.Get("Mcp-Session-Id"); s != "" {
		h.session.Store(s)
	}
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	want := fmt.Sprint(id)
	handle := func(data []byte) (json.RawMessage, bool, error) {
		var msg rpcMessage
		if json.Unmarshal(data, &msg) != nil || msg.Method != "" {
			return nil, false, nil
		}
		if strings.TrimSpace(string(msg.ID)) != want {
			return nil, false, nil
		}
		if msg.Error != nil {
			return nil, true, msg.Error
		}
		return msg.Result, true, nil
	}

	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 1<<20), 16<<20)
		var data []byte
		flush := func() (json.RawMessage, bool, error) {
			if len(data) == 0 {
				return nil, false, nil
			}
			d := data
			data = nil
			return handle(d)
		}
		for sc.Scan() {
			line := sc.Text()
			if line == "" {
				if r, ok, err := flush(); ok {
					return r, err
				}
				continue
			}
			if strings.HasPrefix(line, "data:") {
				data = append(data, []byte(strings.TrimSpace(strings.TrimPrefix(line, "data:")))...)
			}
		}
		if r, ok, err := flush(); ok {
			return r, err
		}
		return nil, errors.New("fluxo SSE terminou sem resposta")
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if r, ok, err := handle(raw); ok {
		return r, err
	}
	return nil, errors.New("resposta HTTP sem resultado")
}

func (h *httpConn) notify(method string, params interface{}) error {
	req := map[string]interface{}{"jsonrpc": "2.0", "method": method}
	if params != nil {
		req["params"] = params
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.post(ctx, req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (h *httpConn) close() {}
