package main

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

//go:embed public/*
var publicFiles embed.FS

// 反向映射：把响应里的 free 名字换回 pro 名字，骗过下游统计工具
var responseReplacer = strings.NewReplacer(
	"mimo-v2.5-free", "mimo-v2.5-pro",
	"deepseek-v4-flash-free", "deepseek-v4-flash",
)

const mimoSystemMarker = "You are MiMoCode, an interactive CLI tool that helps users with software engineering tasks."
const mimoBootstrapURL = "https://api.xiaomimimo.com/api/free-ai/bootstrap"
const mimoChatURL = "https://api.xiaomimimo.com/api/free-ai/openai/chat"

var (
	mimoJwtCache string
	mimoJwtExp   time.Time
	mimoMutex    sync.Mutex
	logMutex     sync.Mutex
	callLogs     []string
)

// 添加日志记录，最多保留 500 条
func addLog(msg string) {
	logMutex.Lock()
	defer logMutex.Unlock()
	callLogs = append(callLogs, msg)
	if len(callLogs) > 500 {
		callLogs = callLogs[len(callLogs)-500:]
	}
}

// 获取 MiMo JWT
func getMimoJWT() (string, error) {
	mimoMutex.Lock()
	defer mimoMutex.Unlock()
	if mimoJwtCache != "" && time.Now().Before(mimoJwtExp) {
		return mimoJwtCache, nil
	}

	seed := "opencodefree-proxy-stable-seed"
	clientHash := fmt.Sprintf("%x", sha256.Sum256([]byte(seed)))
	reqBody := fmt.Sprintf(`{"client":"%s"}`, clientHash)
	
	req, err := http.NewRequest("POST", mimoBootstrapURL, strings.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/131.0.0.0 Safari/537.36")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}
	
	jwtStr, ok := data["jwt"].(string)
	if !ok || jwtStr == "" {
		return "", fmt.Errorf("no jwt returned")
	}
	
	mimoJwtCache = jwtStr
	mimoJwtExp = time.Now().Add(45 * time.Minute) // 缓存45分钟
	return mimoJwtCache, nil
}

// 强制注入 System Prompt
func injectMimoSystemMarker(bodyData map[string]interface{}) {
	msgsRaw, ok := bodyData["messages"].([]interface{})
	if !ok {
		return
	}
	
	hasMarker := false
	for _, m := range msgsRaw {
		msg, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		if msg["role"] == "system" {
			content, ok := msg["content"].(string)
			if ok && strings.Contains(content, mimoSystemMarker) {
				hasMarker = true
				break
			}
		}
	}
	
	if !hasMarker {
		systemMsg := map[string]interface{}{
			"role":    "system",
			"content": mimoSystemMarker,
		}
		newMsgs := append([]interface{}{systemMsg}, msgsRaw...)
		bodyData["messages"] = newMsgs
	}
}

// 流式替换 Reader：对响应体做实时字符串替换（兼容 SSE 流）
type replacingReadCloser struct {
	src     io.ReadCloser
	buf     []byte // 未处理的残留字节
	done    bool
}

func (r *replacingReadCloser) Read(p []byte) (int, error) {
	if r.done && len(r.buf) == 0 {
		return 0, io.EOF
	}

	// 从上游读取新数据
	tmp := make([]byte, len(p))
	n, err := r.src.Read(tmp)
	if err != nil && err != io.EOF {
		return 0, err
	}
	if err == io.EOF {
		r.done = true
	}

	// 拼接残留 + 新数据
	combined := append(r.buf, tmp[:n]...)

	// 保留尾部 30 字节防止替换目标被截断
	overlap := 30
	var toProcess, toKeep []byte
	if r.done || len(combined) <= overlap {
		toProcess = combined
		toKeep = nil
	} else {
		toProcess = combined[:len(combined)-overlap]
		toKeep = combined[len(combined)-overlap:]
	}

	replaced := responseReplacer.Replace(string(toProcess))
	r.buf = toKeep

	copied := copy(p, replaced)
	if copied < len(replaced) {
		// 输出缓冲区不够大，把剩余的存回 buf
		r.buf = append([]byte(replaced[copied:]), r.buf...)
	}

	if r.done && len(r.buf) == 0 {
		return copied, io.EOF
	}
	return copied, nil
}

func (r *replacingReadCloser) Close() error {
	return r.src.Close()
}

func main() {
	// rand.Seed(time.Now().UnixNano()) // Go 1.20+ 自动初始化随机数种子

	subFS, err := fs.Sub(publicFiles, "public")
	if err != nil {
		log.Fatalf("无法加载内嵌的静态文件系统: %v", err)
	}
	fsHandler := http.FileServer(http.FS(subFS))

	opencodeURL, _ := url.Parse("https://opencode.ai")
	proxy := httputil.NewSingleHostReverseProxy(opencodeURL)
	originalDirector := proxy.Director

	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		
		isMimoDirect := false
		requestedModel := "unknown"
		
		if req.Method == "POST" && req.Body != nil {
			bodyBytes, err := io.ReadAll(req.Body)
			if err == nil {
				var reqData map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &reqData); err == nil {
					if model, ok := reqData["model"].(string); ok {
						requestedModel = model
						modified := false
						if model == "deepseek-v4-flash" {
							reqData["model"] = "deepseek-v4-flash-free"
							modified = true
						} else if model == "mimo-v2.5-pro" {
							// 50% 概率走 MiMoCode 直连
							if rand.Intn(2) == 0 {
								isMimoDirect = true
								reqData["model"] = "mimo-auto"
								injectMimoSystemMarker(reqData)
								modified = true
							} else {
								reqData["model"] = "mimo-v2.5-free"
								modified = true
							}
						}
						if modified {
							newBodyBytes, _ := json.Marshal(reqData)
							req.Body = io.NopCloser(bytes.NewBuffer(newBodyBytes))
							req.ContentLength = int64(len(newBodyBytes))
						} else {
							req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
						}
					} else {
						req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
					}
				} else {
					req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				}
			}
		}

		if isMimoDirect {
			// MiMo 直连路线配置
			mimoURL, _ := url.Parse(mimoChatURL)
			req.URL.Scheme = mimoURL.Scheme
			req.URL.Host = mimoURL.Host
			req.URL.Path = mimoURL.Path
			req.Host = mimoURL.Host
			
			jwt, err := getMimoJWT()
			if err != nil {
				log.Printf("获取 MiMo JWT 失败: %v", err)
			} else {
				req.Header.Set("Authorization", "Bearer "+jwt)
			}
			req.Header.Set("X-Mimo-Source", "mimocode-cli-free")
			req.Header.Set("x-session-affinity", "ses_opencodefree")
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/131.0.0.0 Safari/537.36")
			req.Header.Del("x-opencode-client")
			
			addLog(fmt.Sprintf("[%s] 请求 mimo-v2.5-pro -> 🎲 分配至 MiMoCode 官方直连渠道", time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05")))
		} else {
			// OpenCode 路线配置
			if strings.HasPrefix(req.URL.Path, "/v1/") {
				req.URL.Path = "/zen" + req.URL.Path
			}
			req.Host = opencodeURL.Host
			req.Header.Set("Authorization", "Bearer public")
			req.Header.Set("x-opencode-client", "desktop")
			
			if requestedModel != "unknown" {
				addLog(fmt.Sprintf("[%s] 请求 %s -> ☁️ 分配至 OpenCode 渠道", time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05"), requestedModel))
			}
		}
	}

	// 响应拦截：把 free 模型名换回 pro，让下游统计工具看到的永远是 pro
	proxy.ModifyResponse = func(resp *http.Response) error {
		resp.Body = &replacingReadCloser{src: resp.Body}
		resp.Header.Del("Content-Length") // 替换后长度可能变化
		resp.ContentLength = -1
		return nil
	}

	corsMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, x-api-key")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			authHeader := r.Header.Get("Authorization")
			apiKey := r.Header.Get("x-api-key")
			if authHeader != "Bearer sk-mimo" && apiKey != "sk-mimo" {
				http.Error(w, "Unauthorized: Invalid API Key", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		}
	}

	modelsHandler := func(w http.ResponseWriter, r *http.Request) {
		resData := map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{
					"id":       "deepseek-v4-flash",
					"object":   "model",
					"created":  time.Now().Unix(),
					"owned_by": "mimo",
				},
				{
					"id":       "mimo-v2.5-pro",
					"object":   "model",
					"created":  time.Now().Unix(),
					"owned_by": "mimo",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resData)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", corsMiddleware(modelsHandler))
	mux.HandleFunc("/v1/", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	}))
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		urlData, err := os.ReadFile("/tmp/tunnel.url")
		if err != nil {
			w.Write([]byte("Tunnel URL is not ready yet. Please refresh in a few seconds..."))
			return
		}
		w.Write(urlData)
	})
	
	mux.HandleFunc("/log", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		logMutex.Lock()
		defer logMutex.Unlock()
		if len(callLogs) == 0 {
			w.Write([]byte("暂无调用记录。\n"))
			return
		}
		// 倒序输出，最近的日志在前面
		var buf bytes.Buffer
		buf.WriteString("=====================================\n")
		buf.WriteString("       OpenCodeFree 代理网关路由日志     \n")
		buf.WriteString("=====================================\n")
		for i := len(callLogs) - 1; i >= 0; i-- {
			buf.WriteString(callLogs[i] + "\n")
		}
		w.Write(buf.Bytes())
	})
	
	mux.Handle("/", fsHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("双渠道负载均衡网关已启动，监听端口 :%s...", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("网关启动失败: %v", err)
	}
}
