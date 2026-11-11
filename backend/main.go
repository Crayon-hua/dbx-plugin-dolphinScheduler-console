package main

import (
	"bytes"
	"crypto/tls"
	_ "embed"
	"encoding/json"
	"fmt"
	sdk "github.com/t8y2/dbx/plugins/sdk/go/dbx-plugin-sdk"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const pluginID = "io.github.crayon-hua.dolphinscheduler-console"
const version = "0.1.0"

//go:embed tools.json
var toolsData []byte

type obj = map[string]any
type parameter struct {
	Name string `json:"name"`
	In   string `json:"in"`
}
type operation struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Method      string      `json:"method"`
	Path        string      `json:"path"`
	Access      string      `json:"access"`
	Group       string      `json:"group"`
	Params      []parameter `json:"params"`
	Schema      obj         `json:"inputSchema"`
	Annotations obj         `json:"annotations"`
}
type session struct {
	base        string
	token       string
	readOnly    bool
	client      *http.Client
	transport   *http.Transport
	logicalHost string
}
type plugin struct {
	mu       sync.RWMutex
	sessions map[string]*session
	ops      []operation
}

func newPlugin() *plugin {
	p := &plugin{sessions: map[string]*session{}}
	if err := json.Unmarshal(toolsData, &p.ops); err != nil {
		panic(err)
	}
	return p
}
func asObj(v any) obj {
	m, _ := v.(map[string]any)
	if m == nil {
		return obj{}
	}
	return m
}
func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func decode(raw []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	return d.Decode(out)
}
func configSession(lifecycle obj) (*session, error) {
	c := asObj(lifecycle["connection"])
	cfg := asObj(c["external_config"])
	secrets := asObj(c["connection_secrets"])
	rt := asObj(lifecycle["runtime"])
	host := str(c["host"])
	if host == "" {
		host = str(rt["host"])
	}
	if host == "" || strings.ContainsAny(host, "/@?#\\ \t\r\n") {
		return nil, fmt.Errorf("API 主机必须是域名或 IP，不含协议、路径或空格")
	}
	host = strings.Trim(host, "[]")
	port := str(c["port"])
	if port == "" {
		port = str(rt["port"])
	}
	n, e := strconv.Atoi(port)
	if e != nil || n < 1 || n > 65535 {
		return nil, fmt.Errorf("API 端口应为 1–65535")
	}
	scheme := str(cfg["scheme"])
	if scheme == "" {
		scheme = "http"
	}
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("协议仅支持 http 或 https")
	}
	path := str(cfg["api_path"])
	if path == "" {
		path = "/dolphinscheduler"
	}
	if !strings.HasPrefix(path, "/") || strings.ContainsAny(path, "?#\\\r\n") || strings.Contains(path, "..") || strings.Contains(path, "%") || strings.Contains(path, "//") {
		return nil, fmt.Errorf("API 根路径必须是以 / 开头的普通路径")
	}
	token := str(secrets["api_token"])
	if strings.TrimSpace(token) == "" || strings.ContainsAny(token, "\r\n") {
		return nil, fmt.Errorf("请在连接表单中填写有效的 API Token")
	}
	readOnly := true
	if value, exists := cfg["read_only"]; exists {
		switch v := value.(type) {
		case bool:
			readOnly = v
		case string:
			if v != "true" && v != "false" {
				return nil, fmt.Errorf("只读设置无效")
			}
			readOnly = v == "true"
		default:
			return nil, fmt.Errorf("只读设置无效")
		}
	}
	if c["read_only"] == true {
		readOnly = true
	}
	timeout := 30
	if v := str(cfg["timeout_seconds"]); v != "" {
		i, err := strconv.Atoi(v)
		if err != nil || i < 1 || i > 120 {
			return nil, fmt.Errorf("超时应为 1–120 秒")
		}
		timeout = i
	}
	if len(asObj(rt["proxy"])) > 0 {
		return nil, fmt.Errorf("此版本支持 DBX 静态 SSH 转发；不支持 SOCKS5 动态路由")
	}
	targetHost := str(rt["host"])
	if targetHost == "" {
		targetHost = host
	}
	targetPort := str(rt["port"])
	if targetPort == "" {
		targetPort = port
	}
	if strings.ContainsAny(targetHost, "/@?#\\ \t\r\n") {
		return nil, fmt.Errorf("无效的运行时主机")
	}
	if i, err := strconv.Atoi(targetPort); err != nil || i < 1 || i > 65535 {
		return nil, fmt.Errorf("无效的运行时端口")
	}
	tr := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: time.Duration(timeout) * time.Second, KeepAlive: 30 * time.Second}).DialContext, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: host}, MaxIdleConns: 8, IdleConnTimeout: 60 * time.Second, ResponseHeaderTimeout: time.Duration(timeout) * time.Second}
	client := &http.Client{Transport: tr, Timeout: time.Duration(timeout) * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	return &session{base: scheme + "://" + net.JoinHostPort(strings.Trim(targetHost, "[]"), targetPort) + strings.TrimRight(path, "/"), token: token, readOnly: readOnly, client: client, transport: tr, logicalHost: net.JoinHostPort(host, port)}, nil
}

var decimal = regexp.MustCompile(`^-?[0-9]+$`)

func validateValue(key string, v any, schema obj) error {
	typ := str(schema["type"])
	switch typ {
	case "string":
		x, ok := v.(string)
		if !ok {
			return fmt.Errorf("%s 必须使用字符串；64 位 Code 请用十进制字符串", key)
		}
		if schema["pattern"] != nil && !decimal.MatchString(x) {
			return fmt.Errorf("%s 必须是十进制整数字符串", key)
		}
	case "integer", "number":
		x, ok := v.(json.Number)
		if !ok {
			return fmt.Errorf("%s 必须是数字", key)
		}
		f, err := x.Float64()
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || math.Abs(f) > 9007199254740991 || (typ == "integer" && math.Trunc(f) != f) {
			return fmt.Errorf("%s 不是有效的安全整数", key)
		}
		if min, ok := schema["minimum"].(float64); ok && f < min {
			return fmt.Errorf("%s 小于允许值", key)
		}
		if max, ok := schema["maximum"].(float64); ok && f > max {
			return fmt.Errorf("%s 超过允许值", key)
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("%s 必须是布尔值", key)
		}
	}
	if constant, ok := schema["const"]; ok && v != constant {
		return fmt.Errorf("请明确确认修改操作（confirm=true）")
	}
	if enums, ok := schema["enum"].([]any); ok {
		found := false
		for _, allowed := range enums {
			if str(v) == str(allowed) {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("%s 不在允许的枚举值中", key)
		}
	}
	return nil
}
func validate(op operation, args obj) (obj, error) {
	props := asObj(op.Schema["properties"])
	result := obj{}
	for k, v := range args {
		if k == "connectionId" {
			continue
		}
		schema, exists := props[k]
		if !exists {
			return nil, fmt.Errorf("未知参数 %s", k)
		}
		if v == nil {
			continue
		}
		if err := validateValue(k, v, asObj(schema)); err != nil {
			return nil, err
		}
		result[k] = v
	}
	for k, s := range props {
		if _, ok := result[k]; !ok {
			if d, ok := asObj(s)["default"]; ok {
				result[k] = d
			}
		}
	}
	if required, ok := op.Schema["required"].([]any); ok {
		for _, key := range required {
			if _, ok := result[str(key)]; !ok {
				return nil, fmt.Errorf("缺少必填参数 %s", key)
			}
		}
	}
	return result, nil
}
func normalize(v any, token string) any {
	switch x := v.(type) {
	case json.Number:
		if !strings.ContainsAny(string(x), ".eE") {
			if n, err := strconv.ParseInt(string(x), 10, 64); err != nil || n > 9007199254740991 || n < -9007199254740991 {
				return string(x)
			}
		}
		return x
	case string:
		if token != "" {
			return strings.ReplaceAll(x, token, "[REDACTED]")
		}
		return x
	case map[string]any:
		for k, val := range x {
			x[k] = normalize(val, token)
		}
		return x
	case []any:
		for i, val := range x {
			x[i] = normalize(val, token)
		}
		return x
	}
	return v
}
func (s *session) call(op operation, args obj) (any, error) {
	if s.readOnly && op.Access != "read" {
		return nil, fmt.Errorf("此连接为只读，禁止修改；请在 DBX 连接设置中关闭只读后重新连接")
	}
	values, err := validate(op, args)
	if err != nil {
		return nil, err
	}
	path := op.Path
	q := url.Values{}
	for _, param := range op.Params {
		v, exists := values[param.Name]
		if !exists {
			continue
		}
		value := str(v)
		if param.In == "path" {
			if !decimal.MatchString(value) {
				return nil, fmt.Errorf("路径参数 %s 必须是整数", param.Name)
			}
			path = strings.ReplaceAll(path, "{"+param.Name+"}", value)
		} else {
			q.Set(param.Name, value)
		}
	}
	if strings.Contains(path, "{") {
		return nil, fmt.Errorf("缺少路径参数")
	}
	address := s.base + path
	var body io.Reader
	if op.Method == "POST" || op.Method == "PUT" {
		body = strings.NewReader(q.Encode())
	} else if len(q) > 0 {
		address += "?" + q.Encode()
	}
	req, err := http.NewRequest(op.Method, address, body)
	if err != nil {
		return nil, fmt.Errorf("构造 API 请求失败")
	}
	req.Host = s.logicalHost
	req.Header.Set("Token", s.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("无法连接 DolphinScheduler：请检查主机、端口、协议、证书和网络，或稍后重试")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("DolphinScheduler HTTP %d：请检查 Token、权限和 API 根路径", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024+1))
	if err != nil {
		return nil, fmt.Errorf("读取 API 响应失败")
	}
	if len(raw) > 4*1024*1024 {
		return nil, fmt.Errorf("响应超过 4 MiB，请缩小分页或日志行数")
	}
	var result any
	if err := decode(raw, &result); err != nil {
		return nil, fmt.Errorf("API 未返回有效 JSON，请检查 API 根路径是否指向登录页或代理页面")
	}
	envelope, ok := result.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("API 响应不是 DolphinScheduler Result 对象")
	}
	code, hasCode := envelope["code"]
	if !hasCode {
		return nil, fmt.Errorf("API 响应缺少业务状态码，请检查服务版本和 API 根路径")
	}
	if str(code) != "0" {
		msg := str(envelope["msg"])
		if msg == "" {
			msg = str(envelope["message"])
		}
		msg = str(normalize(msg, s.token))
		if len(msg) > 1024 {
			msg = msg[:1024]
		}
		return nil, fmt.Errorf("DolphinScheduler 业务错误 %s：%s", str(code), msg)
	}
	return normalize(result, s.token), nil
}
func (p *plugin) lookup(name string) (operation, bool) {
	for _, op := range p.ops {
		if op.Name == name {
			return op, true
		}
	}
	return operation{}, false
}
func (p *plugin) getSession(id string) (*session, error) {
	p.mu.RLock()
	s := p.sessions[id]
	p.mu.RUnlock()
	if s == nil {
		return nil, fmt.Errorf("连接未打开，请从已保存的 DolphinScheduler 连接进入工作台")
	}
	return s, nil
}
func mcpResult(result any, err error) obj {
	var message string
	if err != nil {
		message = err.Error()
	} else {
		raw, _ := json.Marshal(result)
		message = string(raw)
	}
	return obj{"content": []obj{{"type": "text", "text": message}}, "isError": err != nil}
}
func (p *plugin) Handle(_ sdk.RequestContext, method string, raw json.RawMessage, _ *sdk.Emitter) (any, *sdk.PluginError) {
	v := obj{}
	if len(raw) > 0 && string(raw) != "null" {
		if err := decode(raw, &v); err != nil {
			return nil, sdk.NewError(-32602, "请求参数不是有效 JSON")
		}
	}
	fail := func(err error) (any, *sdk.PluginError) { return nil, sdk.NewError(-32000, err.Error()) }
	switch method {
	case "connection/test", "connection/connect":
		s, err := configSession(v)
		if err != nil {
			return fail(err)
		}
		op, _ := p.lookup("projects_list")
		_, err = s.call(op, obj{})
		if err != nil {
			s.transport.CloseIdleConnections()
			return fail(err)
		}
		if method == "connection/test" {
			s.transport.CloseIdleConnections()
			return obj{"success": true, "message": "连接成功：Token 有效，项目接口可访问。"}, nil
		}
		id := str(asObj(v["connection"])["id"])
		if id == "" {
			s.transport.CloseIdleConnections()
			return fail(fmt.Errorf("缺少连接 ID"))
		}
		p.mu.Lock()
		if old := p.sessions[id]; old != nil {
			old.transport.CloseIdleConnections()
		}
		p.sessions[id] = s
		p.mu.Unlock()
		return obj{"success": true}, nil
	case "connection/disconnect":
		id := str(asObj(v["connection"])["id"])
		if id == "" {
			id = str(v["connectionId"])
		}
		p.mu.Lock()
		if s := p.sessions[id]; s != nil {
			s.transport.CloseIdleConnections()
			delete(p.sessions, id)
		}
		p.mu.Unlock()
		return obj{"success": true}, nil
	case "mcp/tools", "ds/catalog":
		readOnly := false
		if id := str(v["connectionId"]); id != "" {
			if s, err := p.getSession(id); err == nil {
				readOnly = s.readOnly
			}
		}
		list := []obj{}
		for _, op := range p.ops {
			if method == "mcp/tools" && readOnly && op.Access != "read" {
				continue
			}
			list = append(list, obj{"name": op.Name, "description": op.Description, "inputSchema": op.Schema, "annotations": op.Annotations, "group": op.Group, "access": op.Access})
		}
		if method == "mcp/tools" {
			for _, entry := range list {
				delete(entry, "group")
				delete(entry, "access")
			}
			return obj{"tools": list}, nil
		}
		return obj{"tools": list, "readOnly": readOnly, "version": version}, nil
	case "ds/call", "mcp/call":
		op, ok := p.lookup(str(v["tool"]))
		if !ok {
			return fail(fmt.Errorf("未知工具"))
		}
		var s *session
		var err error
		if method == "mcp/call" {
			life := asObj(v["lifecycle"])
			if len(life) == 0 {
				return mcpResult(nil, fmt.Errorf("缺少宿主绑定的连接配置")), nil
			}
			s, err = configSession(life)
			if err == nil {
				defer s.transport.CloseIdleConnections()
			}
		} else {
			s, err = p.getSession(str(v["connectionId"]))
		}
		if err != nil {
			if method == "mcp/call" {
				return mcpResult(nil, err), nil
			}
			return fail(err)
		}
		result, err := s.call(op, asObj(v["arguments"]))
		if method == "mcp/call" {
			return mcpResult(result, err), nil
		}
		if err != nil {
			return fail(err)
		}
		return result, nil
	default:
		return nil, sdk.MethodNotFound(method)
	}
}
func main() {
	server := sdk.NewServer(sdk.Metadata{ID: pluginID, Version: version, Capabilities: []string{"connections", "mcp"}}, newPlugin())
	if err := server.Serve(); err != nil {
		log.Fatal("DBX plugin protocol stopped")
	}
}
