package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/glnreddy421/klew/v2/internal/engine"
	"github.com/glnreddy421/klew/v2/internal/kube"
)

// maxExtensionItems caps each list klew_list_extensions returns.
const maxExtensionItems = 200

func (s *Server) registerExtensionTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "klew_list_extensions",
		Description: "List the user's own collectors and rules (YAML with CEL) that this server loaded: the folders it read, each collector's resource, " +
			"each rule's resource, state and severity, documents that were skipped and why, and load problems. " +
			"Loaded rules run inside every klew_investigate and their findings join the built-in problems. " +
			"Reads local files only; makes no cluster calls. Extensions load when the server starts, so restart it after editing them " +
			"(in Claude Desktop, turn the Klew extension off and on).",
		Annotations: readOnly("List extensions", false),
	}, s.listExtensions)
}

type listExtensionsIn struct{}

type extensionFolder struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
}

type extensionCollector struct {
	Name          string `json:"name"`
	Group         string `json:"group,omitempty"`
	Version       string `json:"version,omitempty"`
	Resource      string `json:"resource"`
	ClusterScoped bool   `json:"cluster_scoped,omitempty"`
	Limit         int    `json:"limit"`
	File          string `json:"file,omitempty"`
}

type extensionRule struct {
	Name     string `json:"name"`
	Resource string `json:"resource"`
	State    string `json:"state"`
	Severity string `json:"severity,omitempty"`
	File     string `json:"file,omitempty"`
}

type skippedExtension struct {
	Kind   string `json:"kind,omitempty"`
	Name   string `json:"name,omitempty"`
	File   string `json:"file"`
	Reason string `json:"reason"`
}

type listExtensionsOut struct {
	Folders    []extensionFolder    `json:"folders"`
	Collectors []extensionCollector `json:"collectors"`
	Rules      []extensionRule      `json:"rules"`
	Skipped    []skippedExtension   `json:"skipped,omitempty"`
	Problems   []string             `json:"problems,omitempty"`
	Truncated  bool                 `json:"truncated,omitempty"`
	Note       string               `json:"note,omitempty"`
}

func (s *Server) listExtensions(_ context.Context, _ *mcp.CallToolRequest, _ listExtensionsIn) (*mcp.CallToolResult, listExtensionsOut, error) {
	reg := engine.ActiveRegistry()
	out := listExtensionsOut{
		Folders: make([]extensionFolder, 0, len(s.opts.ExtensionFolders)), Collectors: []extensionCollector{}, Rules: []extensionRule{},
	}
	for _, f := range s.opts.ExtensionFolders {
		fi, err := os.Stat(f)
		out.Folders = append(out.Folders, extensionFolder{Path: f, Exists: err == nil && fi.IsDir()})
	}

	// Where each loaded document came from, by kind and name.
	files := map[string]string{}
	for _, e := range reg.Entries {
		if e.Status == engine.ExtensionLoaded {
			files[e.Kind+"/"+e.Name] = filepath.Base(e.File)
			continue
		}
		out.Skipped = append(out.Skipped, skippedExtension{Kind: e.Kind, Name: e.Name, File: filepath.Base(e.File), Reason: clean(e.Reason)})
	}

	for _, c := range reg.Collectors {
		spec, ok := kube.ResourceCollectorSpecOf(c)
		if !ok {
			continue
		}
		out.Collectors = append(out.Collectors, extensionCollector{
			Name: spec.Name, Group: spec.Group, Version: spec.Version, Resource: spec.Resource,
			ClusterScoped: spec.ClusterScoped, Limit: spec.Limit, File: files["Collector/"+spec.Name],
		})
	}
	specs := reg.UserRuleSpecs()
	names := make([]string, 0, len(specs))
	for name := range specs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		spec := specs[name]
		out.Rules = append(out.Rules, extensionRule{
			Name: name, Resource: spec.Resource, State: spec.State, Severity: string(spec.Severity), File: files["Rule/"+name],
		})
	}
	for _, w := range reg.Warnings {
		out.Problems = append(out.Problems, clean(w))
	}

	if len(out.Collectors) > maxExtensionItems {
		out.Collectors, out.Truncated = out.Collectors[:maxExtensionItems], true
	}
	if len(out.Rules) > maxExtensionItems {
		out.Rules, out.Truncated = out.Rules[:maxExtensionItems], true
	}
	if len(out.Skipped) > maxExtensionItems {
		out.Skipped, out.Truncated = out.Skipped[:maxExtensionItems], true
	}
	if len(out.Problems) > maxWarnings {
		out.Problems, out.Truncated = out.Problems[:maxWarnings], true
	}
	if len(out.Collectors) == 0 && len(out.Rules) == 0 {
		out.Note = "No collectors or rules are loaded. Put Collector and Rule YAML files in one of the folders above " +
			"(see https://github.com/glnreddy421/klew/tree/main/extensions-examples), then restart the server " +
			"(in Claude Desktop, turn the Klew extension off and on)."
	}
	return nil, out, nil
}
