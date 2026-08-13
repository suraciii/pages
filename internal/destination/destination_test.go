package destination

import "testing"

func TestParseDefaultsToCurrentDirectory(t *testing.T) {
	value, err := ParseWithCurrentDirectory("", func() (string, error) { return "/workspace", nil })
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if value.IsRemote() || value.LocalPath() != "/workspace" {
		t.Fatalf("value = %+v, want local path /workspace", value)
	}
}

func TestParseLocalPaths(t *testing.T) {
	for _, path := range []string{"pages-public", "./pages-public", "/srv/pages/public", "dir/name://page", `C:\pages`, `\\server\pages`} {
		value, err := Parse(path)
		if err != nil {
			t.Fatalf("parse %q: %v", path, err)
		}
		if value.IsRemote() || value.LocalPath() != path {
			t.Fatalf("value for %q = %+v", path, value)
		}
	}
}

func TestParseRemoteURL(t *testing.T) {
	value, err := Parse("https://pages.example.com/docs")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !value.IsRemote() || value.String() != "https://pages.example.com/docs" {
		t.Fatalf("value = %+v", value)
	}
}

func TestParseRejectsInvalidURI(t *testing.T) {
	for _, raw := range []string{
		"ftp://pages.example.com",
		"https://",
		"https://pages.example.com?tenant=one",
		"https://pages.example.com/docs#publish",
	} {
		if _, err := Parse(raw); err == nil {
			t.Fatalf("Parse(%q) succeeded", raw)
		}
	}
}
