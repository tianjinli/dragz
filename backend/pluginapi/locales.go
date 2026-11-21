package pluginapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// extractPageLocale returns a minimal JSON object containing only pages.<id>
// from a full admin locale file. Missing keys yield an empty pages object.
func extractPageLocale(localePath, pluginID string) ([]byte, error) {
	raw, err := os.ReadFile(localePath)
	if err != nil {
		return nil, err
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("parse %s: %w", localePath, err)
	}
	pagesRaw, ok := root["pages"]
	if !ok {
		return []byte("{\n  \"pages\": {}\n}\n"), nil
	}
	var pages map[string]json.RawMessage
	if err := json.Unmarshal(pagesRaw, &pages); err != nil {
		return nil, fmt.Errorf("parse pages in %s: %w", localePath, err)
	}
	outPages := map[string]json.RawMessage{}
	if entry, ok := pages[pluginID]; ok {
		outPages[pluginID] = entry
	}
	out := map[string]any{"pages": outPages}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// mergePageLocale merges pages.<id> from fragment into the host locale file
// without rewriting the whole JSON tree (preserves sibling key order).
func mergePageLocale(hostLocalePath string, fragment []byte, pluginID string) error {
	var frag struct {
		Pages map[string]json.RawMessage `json:"pages"`
	}
	if err := json.Unmarshal(fragment, &frag); err != nil {
		return fmt.Errorf("parse locale fragment: %w", err)
	}
	entry, ok := frag.Pages[pluginID]
	if !ok {
		return nil
	}
	block, err := formatLocalePageBlock(pluginID, entry)
	if err != nil {
		return err
	}

	raw, err := os.ReadFile(hostLocalePath)
	if err != nil {
		return err
	}
	s := strings.ReplaceAll(string(raw), "\r\n", "\n")

	keyMarker := fmt.Sprintf(`"%s": {`, pluginID)
	if i := strings.Index(s, `"pages": {`); i >= 0 {
		// If the key already exists under pages, replace that object.
		pagesStart := i + len(`"pages": {`)
		if idx := strings.Index(s[pagesStart:], keyMarker); idx >= 0 {
			abs := pagesStart + idx
			end, err := findMatchingBrace(s, abs+len(keyMarker)-1)
			if err != nil {
				return fmt.Errorf("%s: %w", hostLocalePath, err)
			}
			// Include trailing comma if present.
			replaceEnd := end + 1
			if replaceEnd < len(s) && s[replaceEnd] == ',' {
				replaceEnd++
			}
			for replaceEnd < len(s) && (s[replaceEnd] == '\n' || s[replaceEnd] == '\r') {
				replaceEnd++
			}
			s = s[:abs] + block + ",\n" + s[replaceEnd:]
			return os.WriteFile(hostLocalePath, []byte(s), 0o644)
		}
		// Insert after "pages": {
		s = s[:pagesStart] + "\n    " + block + "," + s[pagesStart:]
		return os.WriteFile(hostLocalePath, []byte(s), 0o644)
	}
	return fmt.Errorf("%s: cannot find pages object", hostLocalePath)
}

func formatLocalePageBlock(pluginID string, entry json.RawMessage) (string, error) {
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, entry, "    ", "  "); err != nil {
		return "", err
	}
	body := pretty.String()
	// json.Indent on an object keeps the leading '{'; strip indent on first line.
	return fmt.Sprintf(`"%s": %s`, pluginID, body), nil
}

func findMatchingBrace(s string, openIdx int) (int, error) {
	if openIdx < 0 || openIdx >= len(s) || s[openIdx] != '{' {
		return -1, fmt.Errorf("expected '{' at %d", openIdx)
	}
	depth := 0
	inString := false
	escape := false
	for i := openIdx; i < len(s); i++ {
		c := s[i]
		if inString {
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i, nil
			}
		}
	}
	return -1, fmt.Errorf("unbalanced braces")
}

// componentToPageDir maps Meta.Menus Component "/example/index" → "example".
func componentToPageDir(component string) (string, error) {
	c := strings.Trim(component, "/")
	if c == "" || strings.Contains(c, "..") {
		return "", fmt.Errorf("invalid component %q", component)
	}
	parts := strings.Split(c, "/")
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid component %q", component)
	}
	if parts[len(parts)-1] == "index" && len(parts) > 1 {
		parts = parts[:len(parts)-1]
	}
	return strings.Join(parts, "/"), nil
}
