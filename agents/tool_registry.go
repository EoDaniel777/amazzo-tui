package agents

import (
	"context"
	"fmt"
	"strings"
	"amazzo/filesystem"
)

type ToolHandler func(ctx context.Context, params map[string]interface{}) (interface{}, error)

type ToolRegistry struct {
	tools    []Tool
	handlers map[string]ToolHandler
}

func NewToolRegistry(fs *filesystem.Manager) *ToolRegistry {
	tr := &ToolRegistry{
		tools:    make([]Tool, 0),
		handlers: make(map[string]ToolHandler),
	}

	tr.registerTool(
		"read_file",
		"Reads the content of a file at the given path",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{"type": "string", "description": "Relative path to the file"},
			},
			"required": []string{"path"},
		},
		func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			path, ok := params["path"].(string)
			if !ok {
				return nil, fmt.Errorf("missing or invalid path parameter")
			}
			content, err := fs.ReadFile(path)
			if err != nil {
				return nil, err
			}
			return string(content), nil
		},
	)

	tr.registerTool(
		"write_file",
		"Creates or updates a file with the given content",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path":    map[string]interface{}{"type": "string", "description": "Relative path to the file"},
				"content": map[string]interface{}{"type": "string", "description": "Content to write to the file"},
			},
			"required": []string{"path", "content"},
		},
		func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			path, ok1 := params["path"].(string)
			content, ok2 := params["content"].(string)
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("missing or invalid path/content parameters")
			}
			abs, err := fs.WriteFile(path, []byte(content), false)
			if err != nil {
				return nil, err
			}
			return fmt.Sprintf("File written to %s", abs), nil
		},
	)

	tr.registerTool(
		"edit_file",
		"Edits a file by replacing a search string with a replacement string",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path":     map[string]interface{}{"type": "string", "description": "Relative path to the file"},
				"search":   map[string]interface{}{"type": "string", "description": "String to search for"},
				"replace":   map[string]interface{}{"type": "string", "description": "String to replace it with"},
			},
			"required": []string{"path", "search", "replace"},
		},
		func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			path, ok1 := params["path"].(string)
			search, ok2 := params["search"].(string)
			replace, ok3 := params["replace"].(string)
			if !ok1 || !ok2 || !ok3 {
				return nil, fmt.Errorf("missing or invalid parameters")
			}
			res, err := fs.EditFile(path, search, replace)
			if err != nil {
				return nil, err
			}
			return res, nil
		},
	)

	tr.registerTool(
		"make_dir",
		"Creates a new directory",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{"type": "string", "description": "Relative path to the directory"},
			},
			"required": []string{"path"},
		},
		func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			path, ok := params["path"].(string)
			if !ok {
				return nil, fmt.Errorf("missing or invalid path parameter")
			}
			abs, err := fs.Mkdir(path)
			if err != nil {
				return nil, err
			}
			return fmt.Sprintf("Directory created at %s", abs), nil
		},
	)

	tr.registerTool(
		"list_dir",
		"Lists the contents of a directory",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{"type": "string", "description": "Relative path to the directory"},
			},
			"required": []string{"path"},
		},
		func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			path, ok := params["path"].(string)
			if !ok {
				return nil, fmt.Errorf("missing or invalid path parameter")
			}
			entries, _, err := fs.List(path)
			if err != nil {
				return nil, err
			}
			var list []string
			for _, e := range entries {
				prefix := "[FILE]"
				if e.IsDir() {
					prefix = "[DIR]"
				}
				list = append(list, fmt.Sprintf("%s %s", prefix, e.Name()))
			}
			return strings.Join(list, "\n"), nil
		},
	)

	return tr
}

func (tr *ToolRegistry) registerTool(name, desc string, params map[string]interface{}, handler ToolHandler) {
	tr.tools = append(tr.tools, Tool{
		Type: ToolTypeFunction,
		Function: Function{
			Name:        name,
			Description: desc,
			Parameters:  params,
		},
	})
	tr.handlers[name] = handler
}

func (tr *ToolRegistry) Execute(ctx context.Context, call ToolCall) (string, error) {
	handler, ok := tr.handlers[call.Function.Name]
	if !ok {
		return "", fmt.Errorf("tool %s not found", call.Function.Name)
	}
	res, err := handler(ctx, call.Function.Arguments)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", res), nil
}

func (tr *ToolRegistry) GetTools() []Tool {
	return tr.tools
}
