// Package ops 是游戏服务的只读运维端点。
// 缺陷：未知路径返回 200、方法不对也照常处理、限流按全局计数而不是按玩家、
// 429 不带 Retry-After、窗口外请求不回收。
package ops

import (
	"encoding/json"
	"net/http"
)

// Server 是只读运维服务。
type Server struct {
	queue    int
	workers  int
	limit    int
	window   map[string]int
	requests int
	work     int
}

// New 建服务：queue 与 workers 是展示用的水位，limit 是每玩家每秒请求上限。
func New(queue, workers, limit int) *Server {
	return &Server{queue: queue, workers: workers, limit: limit, window: map[string]int{}}
}

// Handler 返回路由。缺陷：没注册的路径也回 200，方法不判，限流按全局。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		s.work += len(s.window)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		s.work += len(s.window)
		body, _ := json.Marshal(map[string]int{"queue": s.queue})
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		s.work += len(s.window)
		s.requests++
		if s.limit > 0 && s.requests > s.limit {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	return mux
}

// Work 是累计扫描的客户端数（规模观测）。
func (s *Server) Work() int { return s.work }
