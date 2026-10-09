package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedSectionWrapsBody(t *testing.T) {
	section := ManagedSection("body line\n")
	if !strings.HasPrefix(section, agentsBeginMarker+"\n") {
		t.Fatalf("section does not start with the begin marker: %q", section)
	}
	if !strings.Contains(section, "\n"+agentsEndMarker+"\n") {
		t.Fatalf("section does not end with the end marker: %q", section)
	}
	if strings.Contains(section, "body line\n\n") {
		t.Fatalf("body was not right-trimmed: %q", section)
	}
}

func TestUpsertManagedSectionAppendsToUserContent(t *testing.T) {
	got, replaced := UpsertManagedSection("# Mine\n\nkeep me\n", "beads body")
	if replaced {
		t.Fatalf("reported a replacement where there was no section")
	}
	if !strings.HasPrefix(got, "# Mine\n\nkeep me\n") {
		t.Fatalf("user content was not preserved verbatim: %q", got)
	}
	if !ContainsManagedSection(got) {
		t.Fatalf("section was not added: %q", got)
	}
	if !strings.Contains(got, "beads body") {
		t.Fatalf("body missing from result: %q", got)
	}
}

func TestUpsertManagedSectionReplacesExistingSection(t *testing.T) {
	first, _ := UpsertManagedSection("# Mine\n\nkeep me\n", "old body")
	got, replaced := UpsertManagedSection(first, "new body")
	if !replaced {
		t.Fatalf("did not report replacing the existing section")
	}
	if strings.Contains(got, "old body") {
		t.Fatalf("old body survived: %q", got)
	}
	if !strings.Contains(got, "new body") {
		t.Fatalf("new body missing: %q", got)
	}
	if !strings.Contains(got, "# Mine\n\nkeep me\n") {
		t.Fatalf("user content was lost: %q", got)
	}
	if n := strings.Count(got, agentsBeginMarker); n != 1 {
		t.Fatalf("expected exactly one begin marker, got %d in %q", n, got)
	}
}

func TestUpsertManagedSectionEmptyFileGetsSectionOnly(t *testing.T) {
	got, replaced := UpsertManagedSection("   \n", "body")
	if replaced {
		t.Fatalf("reported a replacement on an empty file")
	}
	if got != ManagedSection("body") {
		t.Fatalf("empty file did not become exactly the section: %q", got)
	}
}

func TestUpsertManagedSectionUnbalancedMarkersAppend(t *testing.T) {
	broken := "# Mine\n\n" + agentsBeginMarker + "\nno end marker here\n"
	got, replaced := UpsertManagedSection(broken, "body")
	if replaced {
		t.Fatalf("reported a replacement on unbalanced markers")
	}
	if !strings.HasPrefix(got, broken) {
		t.Fatalf("unbalanced file was rewritten in place: %q", got)
	}
	if !strings.Contains(got, agentsEndMarker) {
		t.Fatalf("no section was appended: %q", got)
	}
}

func TestRemoveManagedSectionReportsLeftoverUserContent(t *testing.T) {
	withUser, _ := UpsertManagedSection("# Mine\n\nkeep me\n", "body")
	remaining, hasUser := RemoveManagedSection(withUser)
	if !hasUser {
		t.Fatalf("user content should have been reported as remaining in %q", remaining)
	}
	if !strings.Contains(remaining, "keep me") {
		t.Fatalf("user content was dropped: %q", remaining)
	}
	if strings.Contains(remaining, agentsBeginMarker) || strings.Contains(remaining, agentsEndMarker) {
		t.Fatalf("markers survived removal: %q", remaining)
	}

	onlySection := ManagedSection("body")
	remaining, hasUser = RemoveManagedSection(onlySection)
	if hasUser {
		t.Fatalf("a beads-only file reported user content: %q", remaining)
	}
	if strings.TrimSpace(remaining) != "" {
		t.Fatalf("a beads-only file left content behind: %q", remaining)
	}
}

func TestInstallManagedSectionFilePreservesExistingInstructions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "copilot-instructions.md")
	if err := os.WriteFile(path, []byte("# Project conventions\n\nAlways run make check.\n"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	action, err := InstallManagedSectionFile(path, "beads body")
	if err != nil {
		t.Fatalf("InstallManagedSectionFile: %v", err)
	}
	if action != SectionAdded {
		t.Fatalf("expected SectionAdded over an existing user file, got %q", action)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "Always run make check.") {
		t.Fatalf("user instructions were lost: %q", got)
	}
	if !ContainsManagedSection(got) || !strings.Contains(got, "beads body") {
		t.Fatalf("beads section missing after install: %q", got)
	}

	// A second run replaces the section in place and is still idempotent.
	action, err = InstallManagedSectionFile(path, "beads body")
	if err != nil {
		t.Fatalf("second InstallManagedSectionFile: %v", err)
	}
	if action != SectionUpdated {
		t.Fatalf("expected SectionUpdated on re-run, got %q", action)
	}
	again, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back again: %v", err)
	}
	if string(again) != got {
		t.Fatalf("re-install was not idempotent:\nfirst:\n%s\nsecond:\n%s", got, again)
	}
}

func TestInstallManagedSectionFileRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.md")
	link := filepath.Join(dir, "copilot-instructions.md")
	if err := os.WriteFile(target, []byte("user\n"), 0o644); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if _, err := InstallManagedSectionFile(link, "beads body"); err == nil {
		t.Fatalf("expected a symlink write to be refused")
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(data) != "user\n" {
		t.Fatalf("symlink target was modified: %q", data)
	}
}