// Package ops 是游戏服务的只读运维端点：健康检查、水位快照，
// 以及按玩家分桶的整秒限流窗口。
package ops

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Server 是只读运维服务。
type Server struct {
	queue    int
	workers  int
	limit    int
	mu       sync.Mutex
	window   map[string]bucket
	lastSec  int64
	requests int
	work     int
}

// bucket 是某客户端在某个整秒内的计数。
type bucket struct {
	sec   int64
	count int
}

// New 建服务：queue 与 workers 是展示用的水位，limit 是每玩家每秒请求上限。
func New(queue, workers, limit int) *Server {
	return &Server{queue: queue, workers: workers, limit: limit, window: map[string]bucket{}}
}

// Handler 返回路由：只认 /healthz 与 /stats，其余路径 404，非 GET 一律 405。
// 根路径 / 是按 X-Player 限流的游戏入口。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.serveHealthz)
	mux.HandleFunc("/stats", s.serveStats)
	mux.HandleFunc("/", s.serveRoot)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requests++
		s.mu.Unlock()
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// serveHealthz 返回 200 与 ok。
func (s *Server) serveHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// serveStats 返回当前水位快照，只读不改队列与 worker。
func (s *Server) serveStats(w http.ResponseWriter, r *http.Request) {
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
}

// serveRoot 是限流保护的游戏入口；其它未注册路径一律 404。
func (s *Server) serveRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Retry-After", "1")
	if !s.allow(r.Header.Get("X-Player")) {
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// allow 判定某客户端当前整秒内的请求是否超限：O(1) 查表，
// 只在秒边界翻篇时摊还回收上一秒的过期桶，不做全表扫描。
func (s *Server) allow(player string) bool {
	if s.limit <= 0 {
		return true
	}
	now := time.Now().Unix()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastSec != now {
		for player, b := range s.window {
			if b.sec != now {
				delete(s.window, player)
			}
		}
		s.lastSec = now
	}
	b := s.window[player]
	if b.sec != now {
		b = bucket{sec: now}
	}
	b.count++
	s.window[player] = b
	return b.count <= s.limit
}

// Work 是累计扫描的客户端数（规模观测）。
func (s *Server) Work() int { return s.work }
