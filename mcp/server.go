package mcp

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/coroot/coroot/api"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/rbac"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"k8s.io/klog"
)

const (
	Version1_25 = "1.25"
	Version1_26 = "1.26"
	Version1_27 = "1.27"
)

var (
	supportedGoVersions = []string{Version1_25, Version1_26, Version1_27}
)

type Config struct {
	GoVersion      string
	ProvenanceData map[string]interface{}
}

type Server struct {
	ApiHandler *api.Api
	config     *Config
	handler    *Handler
}

type Handler struct {
	Api      *api.Api
	Server   *mcpserver.MCPServer
	sessions sync.Map
}

type mcpUserCtxKey struct{}

type mcpSessionState struct {
	mu        sync.Mutex
	projectId db.ProjectId
}

func NewServer(apiHandler *api.Api, config *Config) *Server {
	return &Server{
		apiHandler: apiHandler,
		config:     config,
	}
}

func (s *Server) Setup(instructions string) *Handler {
	if s.config == nil {
		s.config = &Config{
			GoVersion: Version1_25,
		}
	}

	klog.Infof("Initializing MCP server with Go version: %s", s.config.GoVersion)
	if s.config.ProvenanceData != nil {
		klog.Infof("Provenance data: %v", s.config.ProvenanceData)
	}

	h := &Handler{
		Api: s.ApiHandler,
		Server: mcpserver.NewMCPServer(
			"coroot",
			"1.0.0",
			mcpserver.WithToolCapabilities(false),
			mcpserver.WithInstructions(instructions),
		),
	}
	h.registerTools()
	return h
}

func (h *Handler) HTTPHandler() http.Handler {
	httpSrv := mcpserver.NewStreamableHTTPServer(
		h.Server,
		mcpserver.WithStateful(true),
		mcpserver.WithHTTPContextFunc(func(ctx context.Context, r *http.Request) context.Context {
			if user := h.Api.MCPUserFromBearer(r); user != nil {
				ctx = context.WithValue(ctx, mcpUserCtxKey{}, user)
			}
			return ctx
		}),
	)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.Api.MCPUserFromBearer(r) == nil {
			resourceMeta := h.Api.GetAbsoluteUrl(r, "/.well-known/oauth-protected-resource").String()
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="coroot", resource_metadata="%s"`, resourceMeta))
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		httpSrv.ServeHTTP(w, r)
	})
}

func (h *Handler) AddTool(tool mcp.Tool, handler mcpserver.ToolHandlerFunc) {
	name := tool.Name
	h.Server.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		h.Api.Stats.RegisterMCPCall(name)
		return handler(ctx, req)
	})
}

func (h *Handler) registerTools() {
	// Original tools from api/mcp.go will be registered here
}

func mcpUserFromContext(ctx context.Context) *db.User {
	u, _ := ctx.Value(mcpUserCtxKey{}).(*db.User)
	return u
}

func (h *Handler) sessionState(ctx context.Context) *mcpSessionState {
	cs := mcpserver.ClientSessionFromContext(ctx)
	if cs == nil {
		return nil
	}
	id := cs.SessionID()
	if id == "" {
		return nil
	}
	if v, ok := h.sessions.Load(id); ok {
		return v.(*mcpSessionState)
	}
	st := &mcpSessionState{}
	actual, _ := h.sessions.LoadOrStore(id, st)
	return actual.(*mcpSessionState)
}

func (h *Handler) currentProject(ctx context.Context) (*db.Project, error) {
	st := h.sessionState(ctx)
	if st == nil {
		return nil, nil
	}
	st.mu.Lock()
	id := st.projectId
	st.mu.Unlock()
	if id == "" {
		return nil, nil
	}
	return h.Api.Db.GetProject(id)
}

func (h *Handler) RequireUserAndProject(ctx context.Context) (*db.User, *db.Project, *mcp.CallToolResult) {
	user := mcpUserFromContext(ctx)
	if user == nil {
		return nil, nil, mcp.NewToolResultError("unauthorized")
	}
	project, err := h.currentProject(ctx)
	if err != nil {
		return nil, nil, mcp.NewToolResultError("failed to load project: " + err.Error())
	}
	if project == nil {
		return nil, nil, mcp.NewToolResultError("no project selected — call select_project first")
	}
	if !h.Api.IsAllowed(user, rbac.Actions.Project(string(project.Id)).List()...) {
		return nil, nil, mcp.NewToolResultError("forbidden: no access to this project")
	}
	return user, project, nil
}
