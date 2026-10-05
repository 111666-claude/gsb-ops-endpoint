// Package ops 是游戏服务的只读运维端点：健康检查、水位快照与按玩家的限流窗口。
package ops

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// clientWindow 是单个客户端在当前整秒桶内的计数。
type clientWindow struct {
	second int64
	count  int
}

// Server 是只读运维服务。
type Server struct {
	queue    int
	workers  int
	limit    int
	mu       sync.Mutex
	windows  map[string]*clientWindow
	requests int
	work     int
}

// New 建服务：queue 与 workers 是展示用的水位，limit 是每玩家每秒请求上限。
func New(queue, workers, limit int) *Server {
	return &Server{queue: queue, workers: workers, limit: limit, windows: map[string]*clientWindow{}}
}

// Handler 返回路由：/healthz 与 /stats 是只读运维端点，/ 是按玩家限流的流量端点；
// 其它路径一律 404，方法不是 GET 一律 405。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if !requireGet(w, r) {
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		if !requireGet(w, r) {
			return
		}
		s.mu.Lock()
		requests := s.requests
		s.mu.Unlock()
		body, _ := json.Marshal(map[string]int{
			"queue":    s.queue,
			"workers":  s.workers,
			"requests": requests,
		})
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if !requireGet(w, r) {
			return
		}
		w.Header().Set("Retry-After", "1")
		s.mu.Lock()
		s.requests++
		allowed := s.allowLocked(r.Header.Get("X-Player"), time.Now().Unix())
		s.mu.Unlock()
		if !allowed {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	return mux
}

// requireGet 拦截非 GET 方法：回 405 并带 Allow 头。
func requireGet(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return false
	}
	return true
}

// allowLocked 按玩家判定是否放行：整秒分桶、跨秒重置，单次判定 O(1)，不全扫客户端表。
func (s *Server) allowLocked(player string, second int64) bool {
	win := s.windows[player]
	if win == nil || win.second != second {
		win = &clientWindow{second: second}
		s.windows[player] = win
	}
	win.count++
	return s.limit <= 0 || win.count <= s.limit
}

// Work 是累计扫描的客户端数（规模观测）：限流判定为 O(1)，不做全表扫描，恒为 0。
func (s *Server) Work() int { return s.work }
