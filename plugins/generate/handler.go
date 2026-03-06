package generate

import (
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/openshift-online/rh-trex-ai/pkg/errors"
	"github.com/openshift-online/rh-trex-ai/pkg/handlers"
)

//go:embed generate-ui.html
var landingPageHTML []byte

var (
	kindPattern = regexp.MustCompile(`^[A-Z][A-Za-z0-9]+$`)
	fieldName   = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	validTypes  = map[string]bool{"string": true, "int": true, "int64": true, "bool": true, "float": true, "time": true}
	validMods   = map[string]bool{"required": true, "optional": true}
	hrefPrefix  = "/generate/"
)

type GenerateRequest struct {
	Kind       string   `json:"kind"`
	Fields     string   `json:"fields"`
	Plural     string   `json:"plural"`
	Generators []string `json:"generators"`
}

type GenerateResponse struct {
	ID        string         `json:"id"`
	Kind      string         `json:"kind"`
	ExpiresAt time.Time      `json:"expires_at"`
	FileCount int            `json:"file_count"`
	Files     []FileResponse `json:"files"`
}

type FileResponse struct {
	Path      string `json:"path"`
	Href      string `json:"href"`
	Generator string `json:"generator"`
}

type ERDRequest struct {
	ERD        string   `json:"erd"`
	Generators []string `json:"generators"`
	Project    string   `json:"project"`
	APIPrefix  string   `json:"api_prefix"`
}

type ERDResponse struct {
	ID         string         `json:"id"`
	Kinds      []string       `json:"kinds"`
	Generators []string       `json:"generators"`
	ExpiresAt  time.Time      `json:"expires_at"`
	FileCount  int            `json:"file_count"`
	Files      []FileResponse `json:"files"`
}

type GenerateHandler struct {
	cache    *GenerationCache
	renderer *Renderer
}

func NewGenerateHandler() *GenerateHandler {
	return &GenerateHandler{
		cache:    NewGenerationCache(),
		renderer: NewRenderer(),
	}
}

func (h *GenerateHandler) Generate(w http.ResponseWriter, r *http.Request) {
	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handlers.HandleError(r.Context(), w, errors.BadRequest("invalid request body: %v", err))
		return
	}

	if err := validateKind(req.Kind); err != nil {
		handlers.HandleError(r.Context(), w, errors.BadRequest("%v", err))
		return
	}
	if err := validateFieldsInput(req.Fields); err != nil {
		handlers.HandleError(r.Context(), w, errors.BadRequest("%v", err))
		return
	}

	files, err := h.renderer.Render(RenderRequest{
		Kind:       req.Kind,
		Fields:     req.Fields,
		Plural:     req.Plural,
		Generators: req.Generators,
	})
	if err != nil {
		handlers.HandleError(r.Context(), w, errors.GeneralError("render failed: %v", err))
		return
	}

	id := newID()
	h.cache.Set(id, req.Kind, files)

	resp := toResponse(id, req.Kind, files)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (h *GenerateHandler) GetResult(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	entry, ok := h.cache.Get(id)
	if !ok {
		handlers.HandleError(r.Context(), w, errors.NotFound("generation result %s not found or expired", id))
		return
	}

	resp := toResponse(id, entry.Kind, entry.Files)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *GenerateHandler) GetFile(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	path := mux.Vars(r)["path"]

	entry, ok := h.cache.Get(id)
	if !ok {
		handlers.HandleError(r.Context(), w, errors.NotFound("generation result %s not found or expired", id))
		return
	}

	for _, f := range entry.Files {
		if f.Path == path {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, f.Content)
			return
		}
	}

	handlers.HandleError(r.Context(), w, errors.NotFound("file %s not found in generation result %s", path, id))
}

func (h *GenerateHandler) LandingPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(landingPageHTML)
}

var allGenerators = []string{"backend", "sdk-go", "sdk-python", "sdk-ts", "cli", "console"}

func (h *GenerateHandler) GenerateERD(w http.ResponseWriter, r *http.Request) {
	var erdText string
	var generators []string
	var project, apiPrefix string

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "text/plain") {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			handlers.HandleError(r.Context(), w, errors.BadRequest("unable to read body: %v", err))
			return
		}
		erdText = string(body)
	} else {
		var req ERDRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			handlers.HandleError(r.Context(), w, errors.BadRequest("invalid request body: %v", err))
			return
		}
		erdText = req.ERD
		generators = req.Generators
		project = req.Project
		apiPrefix = req.APIPrefix
	}

	if strings.TrimSpace(erdText) == "" {
		handlers.HandleError(r.Context(), w, errors.BadRequest("erd field is required"))
		return
	}

	if project == "" {
		project = "my-project"
	}
	if apiPrefix == "" {
		apiPrefix = "/api/" + project + "/v1"
	}

	if len(generators) == 0 {
		generators = []string{"all"}
	}
	genSet := make(map[string]bool, len(generators))
	for _, g := range generators {
		genSet[g] = true
	}
	isAll := genSet["all"]

	entities, err := ParseERD(erdText)
	if err != nil {
		handlers.HandleError(r.Context(), w, errors.BadRequest("ERD parse error: %v", err))
		return
	}

	var allFiles []RenderedFile
	var kinds []string

	for _, entity := range entities {
		if err := validateKind(entity.Kind); err != nil {
			handlers.HandleError(r.Context(), w, errors.BadRequest("entity %s: %v", entity.Kind, err))
			return
		}
		kinds = append(kinds, entity.Kind)
	}

	if isAll || genSet["backend"] {
		for _, entity := range entities {
			files, err := h.renderer.Render(RenderRequest{
				Kind:       entity.Kind,
				Fields:     entity.Fields,
				Generators: []string{"entity"},
			})
			if err != nil {
				handlers.HandleError(r.Context(), w, errors.GeneralError("backend render failed for %s: %v", entity.Kind, err))
				return
			}
			allFiles = append(allFiles, files...)
		}
	}

	hasSDK := isAll || genSet["sdk-go"] || genSet["sdk-python"] || genSet["sdk-ts"]
	if hasSDK {
		sdkLangs := generators
		if isAll {
			sdkLangs = []string{"all"}
		}
		sdkFiles, err := renderSDK(entities, sdkLangs, project, apiPrefix)
		if err != nil {
			handlers.HandleError(r.Context(), w, errors.GeneralError("SDK render failed: %v", err))
			return
		}
		allFiles = append(allFiles, sdkFiles...)
	}

	if isAll || genSet["cli"] {
		cliFiles, err := renderCLI(entities, project, apiPrefix)
		if err != nil {
			handlers.HandleError(r.Context(), w, errors.GeneralError("CLI render failed: %v", err))
			return
		}
		allFiles = append(allFiles, cliFiles...)
	}

	if isAll || genSet["console"] {
		consoleFiles, err := renderConsole(entities, project, apiPrefix)
		if err != nil {
			handlers.HandleError(r.Context(), w, errors.GeneralError("Console render failed: %v", err))
			return
		}
		allFiles = append(allFiles, consoleFiles...)
	}

	activeGenerators := generators
	if isAll {
		activeGenerators = allGenerators
	}

	id := newID()
	h.cache.Set(id, strings.Join(kinds, ","), allFiles)

	var fileResponses []FileResponse
	for _, f := range allFiles {
		fileResponses = append(fileResponses, FileResponse{
			Path:      f.Path,
			Href:      hrefPrefix + id + "/" + f.Path,
			Generator: f.Generator,
		})
	}

	resp := ERDResponse{
		ID:         id,
		Kinds:      kinds,
		Generators: activeGenerators,
		ExpiresAt:  time.Now().Add(defaultTTL),
		FileCount:  len(allFiles),
		Files:      fileResponses,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func validateKind(kind string) error {
	if !kindPattern.MatchString(kind) {
		return fmt.Errorf("kind must match ^[A-Z][A-Za-z0-9]+$")
	}
	return nil
}

func validateFieldsInput(fields string) error {
	if fields == "" {
		return nil
	}
	for _, segment := range strings.Split(fields, ",") {
		parts := strings.Split(strings.TrimSpace(segment), ":")
		if len(parts) < 2 || len(parts) > 3 {
			return fmt.Errorf("invalid field %q: expected name:type or name:type:modifier", segment)
		}
		if !fieldName.MatchString(parts[0]) {
			return fmt.Errorf("field name %q must match ^[a-z][a-z0-9_]*$", parts[0])
		}
		if !validTypes[parts[1]] {
			return fmt.Errorf("field type %q not in allowlist (string, int, int64, bool, float, time)", parts[1])
		}
		if len(parts) == 3 && !validMods[parts[2]] {
			return fmt.Errorf("field modifier %q must be 'required' or 'optional'", parts[2])
		}
	}
	return nil
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return hex.EncodeToString(b)
}

func toResponse(id, kind string, files []RenderedFile) GenerateResponse {
	var fileResponses []FileResponse
	for _, f := range files {
		fileResponses = append(fileResponses, FileResponse{
			Path:      f.Path,
			Href:      hrefPrefix + id + "/" + f.Path,
			Generator: f.Generator,
		})
	}
	return GenerateResponse{
		ID:        id,
		Kind:      kind,
		ExpiresAt: time.Now().Add(defaultTTL),
		FileCount: len(files),
		Files:     fileResponses,
	}
}
