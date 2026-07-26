package ui

import (
	"bytes"
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/x/ansi"
)

// highlightSource returns ANSI-colored source for the terminal preview.
func highlightSource(filename, code string) string {
	if code == "" {
		return code
	}
	lexer := lexers.Match(filename)
	if lexer == nil {
		lexer = lexers.Get(langFromExt(path.Ext(filename)))
	}
	if lexer == nil {
		lexer = lexers.Analyse(code)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get("dracula")
	if style == nil {
		style = styles.Get("monokai")
	}
	if style == nil {
		style = styles.Fallback
	}

	formatter := formatters.Get("terminal16m")
	if formatter == nil {
		formatter = formatters.Get("terminal256")
	}
	if formatter == nil {
		formatter = formatters.Fallback
	}

	it, err := lexer.Tokenise(nil, code)
	if err != nil {
		return code
	}
	var buf bytes.Buffer
	if err := formatter.Format(&buf, style, it); err != nil {
		return code
	}
	return buf.String()
}

func langFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".js", ".mjs", ".cjs":
		return "javascript"
	case ".jsx":
		return "jsx"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "tsx"
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".toml":
		return "toml"
	case ".md", ".markdown":
		return "markdown"
	case ".sh", ".bash":
		return "bash"
	case ".css":
		return "css"
	case ".html", ".htm":
		return "html"
	case ".sql":
		return "sql"
	case ".xml":
		return "xml"
	case ".dockerfile":
		return "dockerfile"
	default:
		return ""
	}
}

func truncateANSILines(s string, maxLines, width int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > maxLines {
		lines = append(lines[:maxLines], "…")
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, "…")
	}
	return strings.Join(lines, "\n")
}

// lintSource runs built-in + optional external linters; always returns something useful.
func lintSource(rel string, data []byte) string {
	ext := strings.ToLower(path.Ext(rel))
	var parts []string

	if msg := builtinLint(rel, ext, data); msg != "" {
		parts = append(parts, msg)
	}
	if msg := externalLint(rel, ext, data); msg != "" {
		parts = append(parts, msg)
	}
	if len(parts) == 0 {
		return helpDescStyle.Render("no linter for " + ext + " · e = LSP editor")
	}
	return strings.Join(parts, "\n")
}

func builtinLint(rel, ext string, data []byte) string {
	switch ext {
	case ".json":
		if !json.Valid(data) {
			var v any
			err := json.Unmarshal(data, &v)
			if err != nil {
				return errorStyle.Render("json ✗ " + err.Error())
			}
		}
		return statusStyle.Render("json ✓ valid")
	case ".go":
		fset := token.NewFileSet()
		_, err := parser.ParseFile(fset, rel, data, parser.AllErrors)
		if err != nil {
			return errorStyle.Render("go ✗ " + err.Error())
		}
		return statusStyle.Render("go ✓ parse ok")
	case ".js", ".mjs", ".cjs", ".jsx":
		if _, err := exec.LookPath("node"); err == nil {
			return "" // externalLint handles node --check
		}
		return statusStyle.Render("js · install node for syntax check · e = LSP")
	case ".ts", ".tsx":
		return statusStyle.Render("ts · e = LSP (tsserver) · or install eslint")
	case ".py":
		if _, err := exec.LookPath("python3"); err == nil {
			return ""
		}
		return statusStyle.Render("py · install python3/ruff · e = LSP")
	case ".yaml", ".yml":
		if looksLikeBrokenYAML(string(data)) {
			return errorStyle.Render("yaml ✗ possible indent/tab mix or empty key")
		}
		return statusStyle.Render("yaml · basic check ok · install yamllint for more")
	default:
		return ""
	}
}

func looksLikeBrokenYAML(s string) bool {
	lines := strings.Split(s, "\n")
	hasTab := false
	hasSpaceIndent := false
	for _, line := range lines {
		if strings.HasPrefix(line, "\t") {
			hasTab = true
		}
		if strings.HasPrefix(line, "  ") {
			hasSpaceIndent = true
		}
	}
	return hasTab && hasSpaceIndent
}

func externalLint(rel, ext string, data []byte) string {
	tmp, err := os.CreateTemp("", "dockafe-lint-*"+ext)
	if err != nil {
		return ""
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return ""
	}
	_ = tmp.Close()

	var cmd *exec.Cmd
	label := ""
	switch ext {
	case ".go":
		// go vet needs a package; builtin parse is enough for preview
		return ""
	case ".py":
		if _, err := exec.LookPath("ruff"); err == nil {
			cmd = exec.Command("ruff", "check", "--no-cache", tmpPath)
			label = "ruff"
		} else if _, err := exec.LookPath("python3"); err == nil {
			cmd = exec.Command("python3", "-m", "py_compile", tmpPath)
			label = "py_compile"
		}
	case ".js", ".mjs", ".cjs", ".jsx":
		if _, err := exec.LookPath("node"); err == nil {
			cmd = exec.Command("node", "--check", tmpPath)
			label = "node"
		} else if _, err := exec.LookPath("eslint"); err == nil {
			cmd = exec.Command("eslint", tmpPath)
			label = "eslint"
		}
	case ".ts", ".tsx":
		if _, err := exec.LookPath("eslint"); err == nil {
			cmd = exec.Command("eslint", tmpPath)
			label = "eslint"
		} else if _, err := exec.LookPath("tsc"); err == nil {
			cmd = exec.Command("tsc", "--noEmit", "--pretty", "false", tmpPath)
			label = "tsc"
		}
	case ".yaml", ".yml":
		if _, err := exec.LookPath("yamllint"); err == nil {
			cmd = exec.Command("yamllint", tmpPath)
			label = "yamllint"
		}
	case ".json":
		return "" // builtin only
	default:
		return ""
	}
	if cmd == nil {
		return ""
	}
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	// Rewrite temp path to original name in output
	text = strings.ReplaceAll(text, tmpPath, rel)
	if err == nil {
		if text == "" {
			return statusStyle.Render(label + " ✓ clean")
		}
		return statusStyle.Render(label+" ✓") + "\n" + text
	}
	if text == "" {
		return errorStyle.Render(label + " ✗ " + err.Error())
	}
	return errorStyle.Render(label+" ✗") + "\n" + text
}

func formatLintPanel(lint string) string {
	if lint == "" {
		return ""
	}
	return "\n\n" + helpKeyStyle.Render("── lint ──") + "\n" + lint
}
