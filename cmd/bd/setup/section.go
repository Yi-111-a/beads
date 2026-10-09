package setup

import (
	"fmt"
	"os"
	"strings"
)

// ManagedSection wraps body in the beads integration markers so it can be
// inserted into - and later replaced inside - a file the user also owns.
func ManagedSection(body string) string {
	return agentsBeginMarker + "\n" + strings.TrimRight(body, "\n") + "\n" + agentsEndMarker + "\n"
}

// ContainsManagedSection reports whether content already carries a beads section.
func ContainsManagedSection(content string) bool {
	return containsBeadsMarker(content)
}

// UpsertManagedSection returns content with its beads section set to body.
// A file that does not have a section yet gets one appended, so unrelated
// user-authored content is preserved. The bool reports whether an existing
// section was replaced (as opposed to one being added).
func UpsertManagedSection(content, body string) (string, bool) {
	section := ManagedSection(body)
	if !containsBeadsMarker(content) {
		if strings.TrimSpace(content) == "" {
			return section, false
		}
		return strings.TrimRight(content, "\n") + "\n\n" + section, false
	}

	start := findBeginMarker(content)
	end := strings.Index(content, agentsEndMarker)
	if start == -1 || end == -1 || start > end {
		// Unbalanced markers: leave the file alone rather than corrupt it.
		return content + "\n\n" + section, false
	}

	after := end + len(agentsEndMarker)
	switch {
	case after+1 < len(content) && content[after] == '\r' && content[after+1] == '\n':
		after += 2
	case after < len(content) && (content[after] == '\r' || content[after] == '\n'):
		after++
	}
	return content[:start] + section + content[after:], true
}

// RemoveManagedSection drops the beads section from content. The bool reports
// whether anything other than whitespace is left, so callers can tell a
// beads-only file (safe to delete) from a shared one.
func RemoveManagedSection(content string) (string, bool) {
	remaining := removeBeadsSection(content)
	return remaining, strings.TrimSpace(remaining) != ""
}

// SectionAction describes what an InstallManagedSectionFile call did.
type SectionAction string

const (
	// SectionCreated means the file did not exist (or was empty) and now holds
	// only the beads section.
	SectionCreated SectionAction = "created"
	// SectionAdded means a beads section was appended to a user-authored file.
	SectionAdded SectionAction = "added"
	// SectionUpdated means an existing beads section was replaced in place.
	SectionUpdated SectionAction = "updated"
)

// InstallManagedSectionFile writes body as the beads section of path, leaving
// the rest of an existing file untouched.
func InstallManagedSectionFile(path, body string) (SectionAction, error) {
	content := ""
	if data, err := os.ReadFile(path); err == nil { // #nosec G304 -- setup destination is trusted
		content = string(data)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read %s: %w", path, err)
	}

	wasEmpty := strings.TrimSpace(content) == ""
	updated, replaced := UpsertManagedSection(content, body)
	if err := atomicWriteFile(path, []byte(updated)); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}

	switch {
	case replaced:
		return SectionUpdated, nil
	case wasEmpty:
		return SectionCreated, nil
	default:
		return SectionAdded, nil
	}
}
