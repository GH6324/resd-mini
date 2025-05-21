package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/ncruces/zenity"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"resd-mini/core/shared"
	sysRuntime "runtime"
	"strings"
	"sync"
	"time"
)

type respData map[string]interface{}

type ResponseData struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type HttpServer struct {
	indexHTML []byte
	upGrader  websocket.Upgrader
	wsClients map[*websocket.Conn]bool
	broadcast chan []byte
	mutex     sync.RWMutex
}

func initHttpServer() *HttpServer {
	if httpServerOnce == nil {
		httpServerOnce = &HttpServer{
			upGrader: websocket.Upgrader{
				CheckOrigin: func(r *http.Request) bool {
					return true
				},
			},
			wsClients: make(map[*websocket.Conn]bool),
			broadcast: make(chan []byte, 1000),
		}
		file, err := appOnce.assets.ReadFile("web/dist/index.html")
		if err != nil {
			globalLogger.Error().Stack().Err(err)
		} else {
			httpServerOnce.indexHTML = file
		}
	}
	return httpServerOnce
}

func (h *HttpServer) run() {
	listener, err := net.Listen("tcp", globalConfig.Host+":"+globalConfig.Port)
	if err != nil {
		globalLogger.Err(err)
		log.Fatalf("Service cannot start: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/ws", h.wsHandler)
	mux.HandleFunc("/api/install", h.install)
	mux.HandleFunc("/api/set-system-password", h.setSystemPassword)
	mux.HandleFunc("/api/preview", h.preview)
	mux.HandleFunc("/api/proxy-open", h.openSystemProxy)
	mux.HandleFunc("/api/proxy-unset", h.unsetSystemProxy)
	mux.HandleFunc("/api/open-directory", h.openDirectoryDialog)
	mux.HandleFunc("/api/open-file", h.openFileDialog)
	mux.HandleFunc("/api/open-folder", h.openFolder)
	mux.HandleFunc("/api/is-proxy", h.isProxy)
	mux.HandleFunc("/api/app-info", h.appInfo)
	mux.HandleFunc("/api/set-config", h.setConfig)
	mux.HandleFunc("/api/get-config", h.getConfig)
	mux.HandleFunc("/api/set-type", h.setType)
	mux.HandleFunc("/api/clear", h.clear)
	mux.HandleFunc("/api/delete", h.delete)
	mux.HandleFunc("/api/download", h.download)
	mux.HandleFunc("/api/wx-file-decode", h.wxFileDecode)
	mux.HandleFunc("/api/batch-import", h.batchImport)
	mux.HandleFunc("/api/cert", h.downCert)

	// Static assets endpoint
	mux.HandleFunc("/", h.staticHandler)

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Host == globalConfig.Host+":"+globalConfig.Port || r.Host == "127.0.0.1:"+globalConfig.Port && strings.Contains(r.URL.Path, "/api") {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				mux.ServeHTTP(w, r)
			} else {
				proxyOnce.Proxy.ServeHTTP(w, r)
			}
		}),
	}
	go h.handleMessages()
	fmt.Println("Service started, listening http://" + globalConfig.Host + ":" + globalConfig.Port)
	if err1 := server.Serve(listener); err1 != nil {
		globalLogger.Err(err1)
		fmt.Printf("Service startup exception: %v", err1)
	}
}

func (h *HttpServer) staticHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" || r.URL.Path == "/index.html" {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(h.indexHTML)
		return
	}

	filePath := strings.TrimPrefix(r.URL.Path, "/")
	file, err := appOnce.assets.ReadFile("web/dist/" + filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Serve the file with correct content type
	http.ServeContent(w, r, filePath, time.Time{}, strings.NewReader(string(file)))
}

func (h *HttpServer) wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upGrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	h.mutex.Lock()
	h.wsClients[conn] = true
	h.mutex.Unlock()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("WebSocket read error:", err)
			break
		}
		h.broadcast <- message
	}
	h.mutex.Lock()
	delete(h.wsClients, conn)
	h.mutex.Unlock()
}

func (h *HttpServer) downCert(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/x-x509-ca-data")
	w.Header().Set("Content-Disposition", "attachment;filename=res-downloader-public.crt")
	w.Header().Set("Content-Transfer-Encoding", "binary")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(appOnce.PublicCrt)))
	w.WriteHeader(http.StatusOK)
	io.Copy(w, io.NopCloser(bytes.NewReader(appOnce.PublicCrt)))
}

func (h *HttpServer) preview(w http.ResponseWriter, r *http.Request) {
	realURL := r.URL.Query().Get("url")
	if realURL == "" {
		http.Error(w, "Missing 'url' parameter", http.StatusBadRequest)
		return
	}
	realURL, _ = url.QueryUnescape(realURL)
	parsedURL, err := url.Parse(realURL)
	if err != nil {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	request, err := http.NewRequest("GET", parsedURL.String(), nil)
	if err != nil {
		http.Error(w, "Failed to fetch the resource", http.StatusInternalServerError)
		return
	}

	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		request.Header.Set("Range", rangeHeader)
	}

	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		http.Error(w, "Failed to fetch the resource", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)

	if contentRange := resp.Header.Get("Content-Range"); contentRange != "" {
		w.Header().Set("Content-Range", contentRange)
	}

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		http.Error(w, "Failed to serve the resource", http.StatusInternalServerError)
	}
	return
}

func (h *HttpServer) handleMessages() {
	for {
		msg := <-h.broadcast
		h.mutex.RLock()
		for client := range h.wsClients {
			err := client.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				fmt.Printf("写入消息错误: %v", err)
				client.Close()
				h.mutex.Lock()
				delete(h.wsClients, client)
				h.mutex.Unlock()
			}
		}
		h.mutex.RUnlock()
	}
}

func (h *HttpServer) send(t string, data interface{}) {
	jsonData, err := json.Marshal(map[string]interface{}{
		"type": t,
		"data": data,
	})
	if err != nil {
		fmt.Println("Error converting map to JSON:", err)
		return
	}
	h.broadcast <- jsonData
}

func (h *HttpServer) writeJson(w http.ResponseWriter, data *ResponseData) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		globalLogger.Err(err)
	}
}

func (h *HttpServer) error(w http.ResponseWriter, args ...interface{}) {
	message := "ok"
	var data interface{}

	if len(args) > 0 {
		message = args[0].(string)
	}
	if len(args) > 1 {
		data = args[1]
	}
	h.writeJson(w, h.buildResp(0, message, data))
}

func (h *HttpServer) success(w http.ResponseWriter, args ...interface{}) {
	message := "ok"
	var data interface{}

	if len(args) > 0 {
		data = args[0]
	}

	if len(args) > 1 {
		message = args[1].(string)
	}
	h.writeJson(w, h.buildResp(1, message, data))
}

func (h *HttpServer) buildResp(code int, message string, data interface{}) *ResponseData {
	return &ResponseData{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

func (h *HttpServer) openDirectoryDialog(w http.ResponseWriter, r *http.Request) {
	folder, err := zenity.SelectFile(zenity.Filename(""), zenity.Directory())
	if err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, respData{
		"folder": folder,
	})
}

func (h *HttpServer) openFileDialog(w http.ResponseWriter, r *http.Request) {
	filePath, err := zenity.SelectFile(
		zenity.Filename(""),
		zenity.FileFilters{
			{"Video files", []string{"*.mp4"}, false},
		})
	if err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, respData{
		"file": filePath,
	})
}

func (h *HttpServer) openFolder(w http.ResponseWriter, r *http.Request) {
	var data struct {
		FilePath string `json:"filePath"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err == nil && data.FilePath == "" {
		return
	}

	filePath := data.FilePath
	var cmd *exec.Cmd

	switch sysRuntime.GOOS {
	case "darwin":
		cmd = exec.Command("open", "-R", filePath)
	case "windows":
		cmd = exec.Command("explorer", "/select,", filePath)
	case "linux":
		cmd = exec.Command("nautilus", filePath)
		if err := cmd.Start(); err != nil {
			cmd = exec.Command("thunar", filePath)
			if err := cmd.Start(); err != nil {
				cmd = exec.Command("dolphin", filePath)
				if err := cmd.Start(); err != nil {
					cmd = exec.Command("pcmanfm", filePath)
					if err := cmd.Start(); err != nil {
						globalLogger.Err(err)
						h.error(w, err.Error())
						return
					}
				}
			}
		}
	default:
		h.error(w, "unsupported platform")
		return
	}

	err = cmd.Start()
	if err != nil {
		globalLogger.Err(err)
		h.error(w, err.Error())
		return
	}
	h.success(w)
}

func (h *HttpServer) install(w http.ResponseWriter, r *http.Request) {
	if appOnce.isInstall() {
		h.success(w, respData{
			"isPass": systemOnce.Password == "",
		})
		return
	}

	out, err := appOnce.installCert()
	if err != nil {
		h.error(w, err.Error()+"\n"+out, respData{
			"isPass": systemOnce.Password == "",
		})
		return
	}

	h.success(w, respData{
		"isPass": systemOnce.Password == "",
	})
}

func (h *HttpServer) setSystemPassword(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Password string `json:"password"`
		IsCache  bool   `json:"isCache"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		h.error(w, err.Error())
		return
	}
	systemOnce.SetPassword(data.Password, data.IsCache)
	h.success(w)
}

func (h *HttpServer) openSystemProxy(w http.ResponseWriter, r *http.Request) {
	err := appOnce.OpenSystemProxy()
	if err != nil {
		h.error(w, err.Error(), respData{
			"value": appOnce.IsProxy,
		})
		return
	}
	h.success(w, respData{
		"value": appOnce.IsProxy,
	})
}

func (h *HttpServer) unsetSystemProxy(w http.ResponseWriter, r *http.Request) {
	err := appOnce.UnsetSystemProxy()
	if err != nil {
		h.error(w, err.Error(), respData{
			"value": appOnce.IsProxy,
		})
		return
	}
	h.success(w, respData{
		"value": appOnce.IsProxy,
	})
}

func (h *HttpServer) isProxy(w http.ResponseWriter, r *http.Request) {
	h.success(w, respData{
		"value": appOnce.IsProxy,
	})
}

func (h *HttpServer) appInfo(w http.ResponseWriter, r *http.Request) {
	h.success(w, appOnce)
}

func (h *HttpServer) getConfig(w http.ResponseWriter, r *http.Request) {
	h.success(w, globalConfig)
}

func (h *HttpServer) setConfig(w http.ResponseWriter, r *http.Request) {
	var data Config
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	globalConfig.setConfig(data)
	h.success(w)
}

func (h *HttpServer) setType(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Type string `json:"type"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err == nil {
		if data.Type != "" {
			resourceOnce.setResType(strings.Split(data.Type, ","))
		} else {
			resourceOnce.setResType([]string{})
		}
	}

	h.success(w)
}

func (h *HttpServer) clear(w http.ResponseWriter, r *http.Request) {
	resourceOnce.clear()
	h.success(w)
}

func (h *HttpServer) delete(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Sign string `json:"sign"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err == nil && data.Sign != "" {
		resourceOnce.delete(data.Sign)
	}
	h.success(w)
}

func (h *HttpServer) download(w http.ResponseWriter, r *http.Request) {
	var data struct {
		MediaInfo
		DecodeStr string `json:"decodeStr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	resourceOnce.download(data.MediaInfo, data.DecodeStr)
	h.success(w)
}

func (h *HttpServer) wxFileDecode(w http.ResponseWriter, r *http.Request) {
	var data struct {
		MediaInfo
		Filename  string `json:"filename"`
		DecodeStr string `json:"decodeStr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	savePath, err := resourceOnce.wxFileDecode(data.MediaInfo, data.Filename, data.DecodeStr)
	if err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, respData{
		"save_path": savePath,
	})
}

func (h *HttpServer) batchImport(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	fileName := filepath.Join(globalConfig.SaveDirectory, "res-downloader-"+shared.GetCurrentDateTimeFormatted()+".txt")
	err := os.WriteFile(fileName, []byte(data.Content), 0644)
	if err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, respData{
		"file_name": fileName,
	})
}
