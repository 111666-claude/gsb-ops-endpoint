package ops

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func call(handler http.Handler, method, path, player string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("X-Player", player)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestHealthzIsOK(t *testing.T) {
	if got := call(New(1, 1, 10).Handler(), "GET", "/healthz", "p1").Code; got != http.StatusOK {
		t.Fatalf("健康检查应该是 200，得到 %d", got)
	}
}

func TestStatsHasQueueField(t *testing.T) {
	body := call(New(3, 2, 10).Handler(), "GET", "/stats", "p1").Body.String()
	if !strings.Contains(body, `"queue"`) {
		t.Fatalf("stats 应该带 queue 字段，得到 %s", body)
	}
}

func TestWorkStartsAtZero(t *testing.T) {
	if New(1, 1, 10).Work() != 0 {
		t.Fatal("Work 初始应该是 0")
	}
}

// TestSampleAnchors 打印题面里引用的现状值。
func TestSampleAnchors(t *testing.T) {
	handler := New(3, 2, 1).Handler()
	unknown := call(handler, "GET", "/nope", "p1")
	t.Logf("unknown_status=%d", unknown.Code)
	badMethod := call(handler, "POST", "/healthz", "p1")
	t.Logf("method_status=%d", badMethod.Code)
	call(handler, "GET", "/", "p1")
	second := call(handler, "GET", "/", "p2")
	t.Logf("second_player_status=%d retry_after=%q", second.Code, second.Header().Get("Retry-After"))
}
