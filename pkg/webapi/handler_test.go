package webapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/liut/steadary/pkg/sandbox"
)

func requireLinux(t *testing.T) {
	if !sandbox.CurrentEnv().IsLinux {
		t.Skip("requires Linux")
	}
}

func requireBwrap(t *testing.T) {
	if !sandbox.CurrentEnv().HasBwrap {
		t.Skip("requires bwrap")
	}
}

// ==================== 路由契约测试 ====================

// newContractMux 注册全部路由，返回可直接 ServeHTTP 的 mux。
func newContractMux(t *testing.T) http.Handler {
	t.Helper()
	m := sandbox.NewManager(sandbox.ManagerConfig{
		SessionRoot: t.TempDir(),
		Driver:      sandbox.DriverNone,
		MaxSessions: 10,
	})
	mux := http.NewServeMux()
	NewHandler(m).Route(mux)
	return mux
}

// TestRouteMethodContract 锁定 REST 方法+路径契约：
// 未注册的方法必须返回 405，且 Allow 头精确列出已注册方法。
func TestRouteMethodContract(t *testing.T) {
	mux := newContractMux(t)

	routes := []struct {
		path  string
		want  string // 精确期望的 Allow 方法集合
		probe string // 未注册的方法
	}{
		{"/api/sessions", "POST", http.MethodGet},
		{"/api/sessions/user/sid", "DELETE", http.MethodGet},
		{"/api/sessions/user/sid/exec", "POST", http.MethodGet},
		{"/api/sessions/exec", "POST", http.MethodGet},
		// GET pattern 在 ServeMux 中隐含 HEAD
		{"/api/stats", "GET, HEAD", http.MethodPost},
		{"/api/ws/user/sid/shell", "GET, HEAD", http.MethodPost},
		{"/api/ws/shell", "GET, HEAD", http.MethodPost},
	}
	for _, rt := range routes {
		t.Run(rt.probe+" "+rt.path, func(t *testing.T) {
			r := httptest.NewRequest(rt.probe, rt.path, http.NoBody)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status=%d, want 405 (route must be registered with a method constraint)", w.Code)
			}
			if got := w.Header().Get("Allow"); got != rt.want {
				t.Errorf("Allow=%q, want %q", got, rt.want)
			}
		})
	}
}

// TestRouteHandlerContract 验证已注册路由命中各自 handler：
// REST handler 一律返回 JSON；WS 路由对非升级请求不得返回 404/405。
func TestRouteHandlerContract(t *testing.T) {
	mux := newContractMux(t)

	jsonRoutes := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/sessions", `{}`},
		{http.MethodDelete, "/api/sessions/user/sid", ""},
		{http.MethodPost, "/api/sessions/user/sid/exec", `{}`},
		{http.MethodPost, "/api/sessions/exec", `{}`},
		{http.MethodGet, "/api/stats", ""},
	}
	for _, rt := range jsonRoutes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			r := httptest.NewRequest(rt.method, rt.path, strings.NewReader(rt.body))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Errorf("Content-Type=%q, want JSON (route must reach its handler, not a mux 404/405)", ct)
			}
		})
	}

	for _, path := range []string{"/api/ws/user/sid/shell", "/api/ws/shell"} {
		t.Run("GET "+path, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, path, http.NoBody)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code == http.StatusNotFound || w.Code == http.StatusMethodNotAllowed {
				t.Errorf("status=%d — websocket route missing or method changed", w.Code)
			}
		})
	}
}

// ==================== Handler 基础测试 ====================

func TestNewHandler(t *testing.T) {
	m := sandbox.NewManager(sandbox.ManagerConfig{
		SessionRoot: "/tmp/steadary/sessions",
		Driver:      sandbox.DriverNone,
		MaxSessions: 10,
	})
	h := NewHandler(m)
	// NewHandler 返回 Handler 接口
	if h == nil {
		t.Error("NewHandler returned nil")
	}
}

// ==================== Helper 函数测试 ====================

func TestJsonOK(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"status": "ok"}

	jsonOK(w, &data)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got '%s'", resp["status"])
	}
}

func TestJsonError(t *testing.T) {
	w := httptest.NewRecorder()
	jsonError(w, "test error", http.StatusBadRequest)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp["error"] != "test error" {
		t.Errorf("expected error 'test error', got '%s'", resp["error"])
	}
}

// ==================== HTTP Handler 测试 ====================

func TestHandleStats(t *testing.T) {
	requireLinux(t)

	m := sandbox.NewManager(sandbox.ManagerConfig{
		SessionRoot: "/tmp/steadary/sessions",
		Driver:      sandbox.DriverNone,
		MaxSessions: 10,
	})
	h := NewHandler(m).(*handlerImpl)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/stats", nil)

	h.handleStats(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]int
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp["active_sessions"] != 0 {
		t.Errorf("expected 0 active sessions, got %d", resp["active_sessions"])
	}
	if len(resp) != 1 {
		t.Errorf("stats response has %d fields, want exactly {active_sessions}: %s", len(resp), w.Body.String())
	}
}

func TestHandleCreateSession(t *testing.T) {
	requireLinux(t)
	requireBwrap(t)

	m := sandbox.NewManager(sandbox.ManagerConfig{
		SessionRoot: "/tmp/steadary/sessions",
		Driver:      sandbox.DriverNone,
		MaxSessions: 10,
	})
	h := NewHandler(m).(*handlerImpl)

	body := `{"owner_id": "testuser", "session_id": "test123"}`
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/api/sessions", bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")

	h.handleCreateSession(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	for _, key := range []string{"owner_id", "session_id", "created_at"} {
		if _, ok := resp[key]; !ok {
			t.Errorf("create session response missing JSON field %q: %s", key, w.Body.String())
		}
	}
}

func TestHandleCreateSessionInvalidBody(t *testing.T) {
	requireLinux(t)

	m := sandbox.NewManager(sandbox.ManagerConfig{
		SessionRoot: "/tmp/steadary/sessions",
		Driver:      sandbox.DriverNone,
		MaxSessions: 10,
	})
	h := NewHandler(m).(*handlerImpl)

	// 缺少 session_id
	body := `{"owner_id": "testuser"}`
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/api/sessions", bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")

	h.handleCreateSession(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleCloseSession(t *testing.T) {
	requireLinux(t)
	requireBwrap(t)

	m := sandbox.NewManager(sandbox.ManagerConfig{
		SessionRoot: "/tmp/steadary/sessions",
		Driver:      sandbox.DriverNone,
		MaxSessions: 10,
	})
	h := NewHandler(m).(*handlerImpl)

	// 先创建
	body := `{"owner_id": "testuser", "session_id": "test-close"}`
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/api/sessions", bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	h.handleCreateSession(w, r)

	// 再关闭
	w = httptest.NewRecorder()
	r, _ = http.NewRequest("DELETE", "/api/sessions/testuser/test-close", nil)
	r.SetPathValue("uid", "testuser")
	r.SetPathValue("sid", "test-close")

	h.handleCloseSession(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleExecValidation(t *testing.T) {
	requireLinux(t)

	m := sandbox.NewManager(sandbox.ManagerConfig{
		SessionRoot: "/tmp/steadary/sessions",
		Driver:      sandbox.DriverNone,
		MaxSessions: 10,
	})
	h := NewHandler(m).(*handlerImpl)

	// 缺少 command
	body := `{"command": ""}`
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/api/sessions/user/sid/exec", bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	r.SetPathValue("uid", "user")
	r.SetPathValue("sid", "sid")

	h.handleExec(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}
