package main

import (
	"encoding/json"
	"fmt"
	sdk "github.com/t8y2/dbx/plugins/sdk/go/dbx-plugin-sdk"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func lifeFor(address string, readOnly bool) obj {
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(address, "http://"))
	return obj{"connection": obj{"id": "test-connection", "host": host, "port": port, "external_config": obj{"scheme": "http", "api_path": "/ds", "read_only": readOnly}, "connection_secrets": obj{"api_token": "test-token-never-log"}}, "runtime": obj{"host": host, "port": port}}
}
func handle(p *plugin, method string, v obj) (any, *sdk.PluginError) {
	raw, _ := json.Marshal(v)
	return p.Handle(sdk.RequestContext{}, method, raw, nil)
}
func argsFor(op operation) obj {
	out := obj{}
	for _, key := range op.Schema["required"].([]any) {
		k := str(key)
		s := asObj(asObj(op.Schema["properties"])[k])
		switch str(s["type"]) {
		case "boolean":
			out[k] = true
		case "integer":
			out[k] = json.Number("7")
		default:
			out[k] = "sample"
			if s["pattern"] != nil {
				out[k] = "9223372036854775806"
			}
			if enums, ok := s["enum"].([]any); ok {
				out[k] = enums[0]
			}
		}
	}
	return out
}

func TestEveryReviewedToolBuildsCorrectRequest(t *testing.T) {
	p := newPlugin()
	for _, op := range p.ops {
		t.Run(op.Name, func(t *testing.T) {
			args := argsFor(op)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != op.Method {
					t.Errorf("method %s", r.Method)
				}
				if r.Header.Get("Token") != "test-token-never-log" {
					t.Error("missing token")
				}
				if strings.Contains(r.URL.RawQuery, "test-token") {
					t.Error("token in URL")
				}
				expected := "/ds" + op.Path
				for _, param := range op.Params {
					if param.In == "path" {
						expected = strings.ReplaceAll(expected, "{"+param.Name+"}", str(args[param.Name]))
					}
				}
				if r.URL.Path != expected {
					t.Errorf("path %s expected %s", r.URL.Path, expected)
				}
				if err := r.ParseForm(); err != nil {
					t.Fatal(err)
				}
				values, _ := validate(op, args)
				for _, param := range op.Params {
					if param.In == "query" {
						if value, ok := values[param.Name]; ok && r.Form.Get(param.Name) != str(value) {
							t.Errorf("parameter %s: %s", param.Name, r.Form.Get(param.Name))
						}
					}
				}
				if r.Form.Has("confirm") {
					t.Error("confirmation leaked into API")
				}
				if (op.Method == "POST" || op.Method == "PUT") && !strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
					t.Error("not form encoded")
				}
				fmt.Fprint(w, `{"code":0,"data":{"code":9223372036854775806}}`)
			}))
			defer server.Close()
			s, err := configSession(lifeFor(server.URL, false))
			if err != nil {
				t.Fatal(err)
			}
			defer s.transport.CloseIdleConnections()
			result, err := s.call(op, args)
			if err != nil {
				t.Fatal(err)
			}
			if asObj(asObj(result)["data"])["code"] != "9223372036854775806" {
				t.Error("int64 precision lost")
			}
		})
	}
}

func TestReadOnlyAndConfirmationNeverSendWrite(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); fmt.Fprint(w, `{"code":0,"data":[]}`) }))
	defer server.Close()
	p := newPlugin()
	for _, op := range p.ops {
		if op.Access == "read" {
			continue
		}
		s, _ := configSession(lifeFor(server.URL, true))
		_, err := s.call(op, argsFor(op))
		s.transport.CloseIdleConnections()
		if err == nil {
			t.Errorf("read-only allowed %s", op.Name)
		}
		s, _ = configSession(lifeFor(server.URL, false))
		args := argsFor(op)
		delete(args, "confirm")
		_, err = s.call(op, args)
		s.transport.CloseIdleConnections()
		if err == nil {
			t.Errorf("missing confirmation allowed %s", op.Name)
		}
	}
	if requests.Load() != 0 {
		t.Fatal("guarded write reached network")
	}
}

func TestErrorHandlingAndRedirects(t *testing.T) {
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Add(1) }))
	defer target.Close()
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"business", `{"code":100,"msg":"invalid test-token-never-log"}`, 200},
		{"unauthorized", `test-token-never-log`, 401},
		{"html", `<html>login test-token-never-log</html>`, 200},
		{"shape", `{"data":[]}`, 200},
		{"redirect", "", 302},
		{"oversized", strings.Repeat("x", 4*1024*1024+1), 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.status == 302 {
					w.Header().Set("Location", target.URL)
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			s, _ := configSession(lifeFor(server.URL, true))
			defer s.transport.CloseIdleConnections()
			op, _ := newPlugin().lookup("projects_list")
			_, err := s.call(op, obj{})
			if err == nil {
				t.Fatal("expected error")
			}
			if strings.Contains(err.Error(), "test-token-never-log") {
				t.Fatal("secret leaked")
			}
		})
	}
	if redirected.Load() != 0 {
		t.Error("followed redirect with token")
	}
}

func TestLifecycleAndMCP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"code":0,"data":{"totalList":[],"total":0}}`)
	}))
	defer server.Close()
	p := newPlugin()
	life := lifeFor(server.URL, true)
	for _, method := range []string{"connection/test", "connection/connect", "connection/connect"} {
		if _, e := handle(p, method, life); e != nil {
			t.Fatal(e)
		}
	}
	catalog, e := handle(p, "mcp/tools", obj{"connectionId": "test-connection"})
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range asObj(catalog)["tools"].([]obj) {
		if asObj(entry["annotations"])["readOnlyHint"] != true {
			t.Fatal("read-only connection advertised write")
		}
	}
	result, e := handle(p, "mcp/call", obj{"tool": "projects_list", "arguments": obj{}, "lifecycle": life})
	if e != nil || asObj(result)["isError"] != false {
		t.Fatalf("MCP call failed %v %v", result, e)
	}
	result, e = handle(p, "mcp/call", obj{"tool": "project_create", "arguments": obj{"projectName": "x", "confirm": true}, "lifecycle": life})
	if e != nil || asObj(result)["isError"] != true {
		t.Fatal("MCP write guard failed")
	}
	for i := 0; i < 2; i++ {
		if _, e := handle(p, "connection/disconnect", life); e != nil {
			t.Fatal(e)
		}
	}
	if _, err := p.getSession("test-connection"); err == nil {
		t.Fatal("session survived disconnect")
	}
}

func TestValidationAndConfiguration(t *testing.T) {
	p := newPlugin()
	op, _ := p.lookup("workflow_get")
	for _, args := range []obj{{"projectCode": "1", "code": "2/../secret"}, {"projectCode": json.Number("1"), "code": "2"}, {"projectCode": "1", "code": "2", "url": "https://other"}} {
		if _, err := validate(op, args); err == nil {
			t.Errorf("accepted invalid args %v", args)
		}
	}
	op, _ = p.lookup("projects_list")
	for _, args := range []obj{{"pageSize": json.Number("101")}, {"pageNo": json.Number("0")}} {
		if _, err := validate(op, args); err == nil {
			t.Error("pagination limit not enforced")
		}
	}
	life := lifeFor("http://127.0.0.1:12345", true)
	delete(asObj(life["connection"]), "connection_secrets")
	if _, err := configSession(life); err == nil {
		t.Fatal("missing token accepted")
	}
	life = lifeFor("http://127.0.0.1:12345", true)
	asObj(asObj(life["connection"])["external_config"])["api_path"] = "/../escape"
	if _, err := configSession(life); err == nil {
		t.Fatal("bad path accepted")
	}
}
