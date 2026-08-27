package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/gorilla/mux"

	"github.com/openshift-online/rh-trex-ai/pkg/auth"
	"github.com/openshift-online/rh-trex-ai/pkg/environments"
	"github.com/openshift-online/rh-trex-ai/pkg/server"
)

const applicationAPIPrefix = "/api/rh-trex-ai/v1"

type applicationRoute struct {
	method string
	path   string
}

func (r applicationRoute) String() string {
	return r.method + " " + r.path
}

type openAPIDocument struct {
	Paths map[string]openAPIPathItem `json:"paths"`
}

type openAPIPathItem struct {
	Ref     string            `json:"$ref"`
	Get     *openAPIOperation `json:"get"`
	Post    *openAPIOperation `json:"post"`
	Put     *openAPIOperation `json:"put"`
	Patch   *openAPIOperation `json:"patch"`
	Delete  *openAPIOperation `json:"delete"`
	Options *openAPIOperation `json:"options"`
	Head    *openAPIOperation `json:"head"`
	Trace   *openAPIOperation `json:"trace"`
}

type openAPIOperation struct {
	OperationID string                 `json:"operationId"`
	Responses   map[string]interface{} `json:"responses"`
}

func TestRegisteredApplicationRoutesMatchResolvedOpenAPI(t *testing.T) {
	registered := registeredApplicationRoutes(t)
	documented := documentedApplicationRoutes(t)

	if missing := routeDifference(registered, documented); len(missing) > 0 {
		t.Errorf("registered application routes missing from resolved OpenAPI:\n  %s", strings.Join(missing, "\n  "))
	}
	if extra := routeDifference(documented, registered); len(extra) > 0 {
		t.Errorf("resolved OpenAPI operations without a registered application route:\n  %s", strings.Join(extra, "\n  "))
	}
}

func registeredApplicationRoutes(t *testing.T) map[applicationRoute]struct{} {
	t.Helper()

	router := mux.NewRouter()
	apiRouter := router.PathPrefix(applicationAPIPrefix).Subrouter()
	server.LoadDiscoveredRoutes(
		apiRouter,
		&environments.Services{},
		&auth.MiddlewareMock{},
		auth.NewAuthzMiddlewareMock(),
	)

	routes := make(map[applicationRoute]struct{})
	err := router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		methods, err := route.GetMethods()
		if err != nil {
			return nil
		}
		pathTemplate, err := route.GetPathTemplate()
		if err != nil {
			return fmt.Errorf("get registered path template: %w", err)
		}
		for _, method := range methods {
			addRoute(t, routes, method, pathTemplate, "registered router")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk registered application routes: %v", err)
	}
	return routes
}

func documentedApplicationRoutes(t *testing.T) map[applicationRoute]struct{} {
	t.Helper()

	operations := loadResolvedOpenAPIOperations(t, filepath.Join("..", "..", "openapi", "openapi.yaml"))
	routes := make(map[applicationRoute]struct{}, len(operations))
	for route := range operations {
		addRoute(t, routes, route.method, route.path, "resolved OpenAPI")
	}
	return routes
}

func loadResolvedOpenAPIOperations(t *testing.T, rootPath string) map[applicationRoute]openAPIOperation {
	t.Helper()

	rootPath, err := filepath.Abs(rootPath)
	if err != nil {
		t.Fatalf("resolve root OpenAPI path: %v", err)
	}
	documentRoot := filepath.Dir(rootPath)
	document := readOpenAPIDocument(t, rootPath)
	operations := make(map[applicationRoute]openAPIOperation)
	for routePath, pathItem := range document.Paths {
		resolved := resolveOpenAPIPathItem(t, documentRoot, rootPath, pathItem, make(map[string]struct{}))
		for method, operation := range map[string]*openAPIOperation{
			"GET": resolved.Get, "POST": resolved.Post, "PUT": resolved.Put, "PATCH": resolved.Patch,
			"DELETE": resolved.Delete, "OPTIONS": resolved.Options, "HEAD": resolved.Head, "TRACE": resolved.Trace,
		} {
			if operation == nil {
				continue
			}
			if operation.OperationID == "" {
				t.Fatalf("resolved OpenAPI operation %s %s has no operationId", method, routePath)
			}
			route := applicationRoute{method: method, path: normalizeRoutePath(routePath)}
			if _, exists := operations[route]; exists {
				t.Fatalf("duplicate resolved OpenAPI route after normalization: %s", route)
			}
			operations[route] = *operation
		}
	}
	return operations
}

func resolveOpenAPIPathItem(t *testing.T, documentRoot, currentPath string, item openAPIPathItem, visited map[string]struct{}) openAPIPathItem {
	t.Helper()
	if item.Ref == "" {
		return item
	}

	parts := strings.SplitN(item.Ref, "#", 2)
	if len(parts) != 2 || !strings.HasPrefix(parts[1], "/paths/") {
		t.Fatalf("unsupported OpenAPI path item reference %q", item.Ref)
	}
	targetPath := filepath.Clean(filepath.Join(filepath.Dir(currentPath), parts[0]))
	relative, err := filepath.Rel(documentRoot, targetPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(parts[0]) {
		t.Fatalf("OpenAPI path item reference escapes document root: %q", item.Ref)
	}
	pointer := strings.TrimPrefix(parts[1], "/paths/")
	pathKey := strings.ReplaceAll(strings.ReplaceAll(pointer, "~1", "/"), "~0", "~")
	visitKey := targetPath + "#/paths/" + pointer
	if _, exists := visited[visitKey]; exists {
		t.Fatalf("cyclic OpenAPI path item reference: %s", visitKey)
	}
	visited[visitKey] = struct{}{}

	document := readOpenAPIDocument(t, targetPath)
	target, exists := document.Paths[pathKey]
	if !exists {
		t.Fatalf("OpenAPI path item reference %q does not exist", item.Ref)
	}
	return resolveOpenAPIPathItem(t, documentRoot, targetPath, target, visited)
}

func readOpenAPIDocument(t *testing.T, documentPath string) openAPIDocument {
	t.Helper()
	contents, err := os.ReadFile(documentPath)
	if err != nil {
		t.Fatalf("read OpenAPI document %s: %v", documentPath, err)
	}
	var document openAPIDocument
	if err := yaml.Unmarshal(contents, &document); err != nil {
		t.Fatalf("parse OpenAPI document %s: %v", documentPath, err)
	}
	return document
}

func addRoute(t *testing.T, routes map[applicationRoute]struct{}, method, routePath, source string) {
	t.Helper()

	route := applicationRoute{
		method: strings.ToUpper(method),
		path:   normalizeRoutePath(routePath),
	}
	if _, exists := routes[route]; exists {
		t.Fatalf("duplicate %s route after normalization: %s", source, route)
	}
	routes[route] = struct{}{}
}

func normalizeRoutePath(routePath string) string {
	segments := strings.Split(strings.TrimSuffix(routePath, "/"), "/")
	for index, segment := range segments {
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			segments[index] = "{}"
		}
	}
	return strings.Join(segments, "/")
}

func routeDifference(left, right map[applicationRoute]struct{}) []string {
	difference := make([]string, 0)
	for route := range left {
		if _, exists := right[route]; !exists {
			difference = append(difference, route.String())
		}
	}
	sort.Strings(difference)
	return difference
}
