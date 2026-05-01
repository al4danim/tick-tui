package tui

import (
	"regexp"
	"strings"
	"unicode"
)

// atWordRe matches a trailing @prefix. Using [^@\s]* instead of \w* so CJK
// project names like @工作 are captured correctly (matches server-side parse_input).
var atWordRe = regexp.MustCompile(`@([^@\s]*)$`)

// ComputeGhostText finds the trailing @prefix in input and returns
// the suffix that would complete the first matching project name.
// Returns "" if no completion is available.
func ComputeGhostText(input string, projects []string) string {
	m := atWordRe.FindStringSubmatch(input)
	if m == nil {
		return ""
	}
	prefix := m[1] // text after @
	for _, p := range projects {
		if strings.HasPrefix(strings.ToLower(p), strings.ToLower(prefix)) && len(p) > len(prefix) {
			return p[len(prefix):]
		}
	}
	return ""
}

// AcceptGhostText appends the ghost suffix to current input value.
func AcceptGhostText(current, ghost string) string {
	return current + ghost
}

// computeProjectGhost returns the suffix that would complete the project field
// value to the first matching project name. Empty value matches the first project.
func computeProjectGhost(value string, projects []string) string {
	for _, p := range projects {
		if strings.HasPrefix(strings.ToLower(p), strings.ToLower(value)) && len(p) > len(value) {
			return p[len(value):]
		}
	}
	return ""
}

// renderTitleWithGhost renders the title input inline as:
//
//	<before><reverse-cursor-char><after><ghost>
//
// The cursor is shown as a reverse-video block over the character beneath it
// (or a reverse-video space when the cursor sits at end-of-text). Ghost text
// is rendered dim and only at end-of-text.
func renderTitleWithGhost(value string, pos int, ghost string, active bool) string {
	if !active {
		return value
	}
	// value is UTF-8 but pos is a rune index (from textinput.Position()).
	// Slicing by byte index would split multi-byte CJK characters → panic.
	runes := []rune(value)
	if pos > len(runes) {
		pos = len(runes)
	}
	if pos < len(runes) {
		// Cursor mid-text: reverse the character at cursor position; suppress ghost.
		before := string(runes[:pos])
		at := string(runes[pos])
		after := string(runes[pos+1:])
		return before + styleCursor.Render(at) + after
	}
	// Cursor at end: reverse a space, then optional dim ghost.
	return value + styleCursor.Render(" ") + styleDim.Render(ghost)
}

// renderProjectField renders the project field inline, with ghost-text autocomplete.
func renderProjectField(value string, projects []string, active bool) string {
	if active {
		ghost := computeProjectGhost(value, projects)
		return "@" + value + styleCursor.Render(" ") + styleDim.Render(ghost)
	}
	if value == "" {
		return ""
	}
	return "@" + value
}

// isWordChar matches \w characters for the @-completion regex.
func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// extractProjectFromTitle parses the last "@word" token out of title.
// Returns (cleanTitle, project). project is "" if no @token found.
func extractProjectFromTitle(s string) (string, string) {
	m := atWordRe.FindStringIndex(s)
	if m == nil {
		return strings.TrimSpace(s), ""
	}
	project := s[m[0]+1 : m[1]] // strip leading @
	clean := strings.TrimSpace(s[:m[0]])
	return clean, project
}

// buildPostText combines title and project into the text form expected by
// POST /features: "title @project" (or just "title" if project is empty).
func buildPostText(title, project string) string {
	t := strings.TrimSpace(title)
	p := strings.TrimSpace(project)
	if p == "" {
		// Still check if title itself contains @project inline
		return t
	}
	return t + " @" + p
}
