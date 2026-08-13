package destination

import "testing"

func TestParseDefaultsToCurrentDirectory(t *testing.T) {
	value, err := Parse("", func() (string, error) { return "/workspace", nil })
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if value.IsRemote() || value.LocalPath() != "/workspace" {
		t.Fatalf("value = %+v, want local path /workspace", value)
	}
}

func TestParseLocalPaths(t *testing.T) {
	for _, path := range []string{"pages-public", "./pages-public", "/srv/pages/public", "dir/name://page", `C:\pages`, `\\server\pages`} {
		value, err := Parse(path, unexpectedCurrentDirectory(t))
		if err != nil {
			t.Fatalf("parse %q: %v", path, err)
		}
		if value.IsRemote() || value.LocalPath() != path {
			t.Fatalf("value for %q = %+v", path, value)
		}
	}
}

func TestParseRemoteURL(t *testing.T) {
	value, err := Parse("https://pages.example.com/docs", unexpectedCurrentDirectory(t))
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
		if _, err := Parse(raw, unexpectedCurrentDirectory(t)); err == nil {
			t.Fatalf("Parse(%q) succeeded", raw)
		}
	}
}

func unexpectedCurrentDirectory(t *testing.T) func() (string, error) {
	t.Helper()
	return func() (string, error) {
		t.Fatal("current directory was resolved for a non-empty Destination")
		return "", nil
	}
}
