package pages

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/suraciii/pages/internal/filesystem"
)

const (
	catalogStagingName = "catalog-staging"
	catalogLockName    = "catalog.lock"
)

type catalogScope struct {
	identity string
	pages    []string
}

type catalogTarget struct {
	key     string
	path    string
	content []byte
}

// RefreshIndexes rebuilds the root and Named Identity Page Indexes from the
// current Public Root. The Index files are system artifacts, not Pages.
func RefreshIndexes(publicRoot string, fileSystem filesystem.FS) error {
	if publicRoot == "" {
		return errors.New("public root is required")
	}
	if fileSystem == nil {
		fileSystem = filesystem.OS
	}
	privateRoot := filepath.Join(publicRoot, ".pages")
	if err := fileSystem.MkdirAll(privateRoot, 0o700); err != nil {
		return fmt.Errorf("create pages state: %w", err)
	}
	if err := fileSystem.Chmod(privateRoot, 0o700); err != nil {
		return fmt.Errorf("make pages state private: %w", err)
	}
	stagingDir := filepath.Join(privateRoot, catalogStagingName)
	if err := fileSystem.MkdirAll(stagingDir, 0o700); err != nil {
		return fmt.Errorf("create catalog staging: %w", err)
	}
	if err := fileSystem.Chmod(stagingDir, 0o700); err != nil {
		return fmt.Errorf("make catalog staging private: %w", err)
	}
	unlock, err := fileSystem.Lock(filepath.Join(privateRoot, catalogLockName))
	if err != nil {
		return fmt.Errorf("lock catalog: %w", err)
	}
	defer unlock()
	if err := recoverCatalogSwaps(publicRoot, stagingDir, fileSystem); err != nil {
		return err
	}

	scopes, err := scanCatalogScopes(publicRoot, fileSystem)
	if err != nil {
		return err
	}
	targets := makeCatalogTargets(publicRoot, scopes)
	for _, target := range targets {
		if err := writeCatalog(target, stagingDir, fileSystem); err != nil {
			return err
		}
	}
	return nil
}

func scanCatalogScopes(publicRoot string, fileSystem filesystem.FS) ([]catalogScope, error) {
	entries, err := fileSystem.ReadDir(publicRoot)
	if err != nil {
		return nil, fmt.Errorf("scan public root: %w", err)
	}
	rootPages := make([]string, 0)
	identities := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || !entry.IsDir() {
			continue
		}
		if strings.HasPrefix(name, "@") {
			identity := strings.TrimPrefix(name, "@")
			if ValidName(identity) {
				identities = append(identities, identity)
			}
			continue
		}
		if ValidName(name) && isPageDir(filepath.Join(publicRoot, name), fileSystem) {
			rootPages = append(rootPages, name)
		}
	}
	sort.Strings(rootPages)
	sort.Strings(identities)

	scopes := make([]catalogScope, 0, len(identities)+1)
	scopes = append(scopes, catalogScope{pages: rootPages})
	for _, identity := range identities {
		pages, err := scanIdentityPages(publicRoot, identity, fileSystem)
		if err != nil {
			return nil, err
		}
		scopes = append(scopes, catalogScope{identity: identity, pages: pages})
	}
	return scopes, nil
}

func scanIdentityPages(publicRoot, identity string, fileSystem filesystem.FS) ([]string, error) {
	identityDir := filepath.Join(publicRoot, IdentityScope(identity))
	entries, err := fileSystem.ReadDir(identityDir)
	if err != nil {
		return nil, fmt.Errorf("scan identity %q: %w", identity, err)
	}
	pages := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || !entry.IsDir() || !ValidName(name) {
			continue
		}
		if isPageDir(filepath.Join(identityDir, name), fileSystem) {
			pages = append(pages, name)
		}
	}
	sort.Strings(pages)
	return pages, nil
}

func isPageDir(path string, fileSystem filesystem.FS) bool {
	info, err := fileSystem.Stat(filepath.Join(path, "index.html"))
	return err == nil && info.Mode().IsRegular()
}

func makeCatalogTargets(publicRoot string, scopes []catalogScope) []catalogTarget {
	targets := make([]catalogTarget, 0, len(scopes))
	for _, scope := range scopes {
		if scope.identity == "" {
			targets = append(targets, catalogTarget{
				key:     "root",
				path:    filepath.Join(publicRoot, "index.html"),
				content: renderRootIndex(scope.pages, scopes[1:]),
			})
			continue
		}
		targets = append(targets, catalogTarget{
			key:     "identity-" + scope.identity,
			path:    filepath.Join(publicRoot, IdentityScope(scope.identity), "index.html"),
			content: renderIdentityIndex(scope.identity, scope.pages),
		})
	}
	return targets
}

func renderRootIndex(pages []string, identities []catalogScope) []byte {
	var body bytes.Buffer
	writeDocumentStart(&body, "Pages", "Pages")
	writePageList(&body, "Pages", pages, "./")
	if len(identities) > 0 {
		body.WriteString("<section>\n<h2>Identities</h2>\n<ul>\n")
		for _, identity := range identities {
			name := "@" + identity.identity + "/"
			writeListItem(&body, name, "Identity", "./"+IdentityScope(identity.identity)+"/")
		}
		body.WriteString("</ul>\n</section>\n")
	}
	writeDocumentEnd(&body)
	return body.Bytes()
}

func renderIdentityIndex(identity string, pages []string) []byte {
	var body bytes.Buffer
	title := "Pages / @" + identity
	writeDocumentStart(&body, title, title)
	body.WriteString("<nav aria-label=\"Breadcrumb\"><a href=\"../\">Pages</a> / @")
	body.WriteString(html.EscapeString(identity))
	body.WriteString("</nav>\n")
	writePageList(&body, "Pages", pages, "./")
	writeDocumentEnd(&body)
	return body.Bytes()
}

func writeDocumentStart(body *bytes.Buffer, title, heading string) {
	body.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n")
	body.WriteString("<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	body.WriteString("<title>")
	body.WriteString(html.EscapeString(title))
	body.WriteString("</title>\n<style>body{font:16px system-ui,sans-serif;line-height:1.5;max-width:52rem;margin:3rem auto;padding:0 1rem;color:#202124}a{color:#1257a6;text-decoration:none}a:hover{text-decoration:underline}ul{list-style:none;padding:0;border-top:1px solid #d9d9d9}li{display:flex;gap:1rem;padding:.65rem 0;border-bottom:1px solid #d9d9d9}li a{flex:1}li span{color:#6b7280;font-size:.9rem}nav{color:#6b7280;margin-bottom:1.5rem}h1{font-size:2rem;font-weight:600}h2{font-size:1.1rem;margin-top:2rem}</style>\n")
	body.WriteString("</head>\n<body>\n<main>\n<h1>")
	body.WriteString(html.EscapeString(heading))
	body.WriteString("</h1>\n")
}

func writeDocumentEnd(body *bytes.Buffer) {
	body.WriteString("</main>\n</body>\n</html>\n")
}

func writePageList(body *bytes.Buffer, heading string, pages []string, prefix string) {
	body.WriteString("<section>\n<h2>")
	body.WriteString(html.EscapeString(heading))
	body.WriteString("</h2>\n")
	if len(pages) == 0 {
		body.WriteString("<p>No Pages published yet.</p>\n</section>\n")
		return
	}
	body.WriteString("<ul>\n")
	for _, page := range pages {
		writeListItem(body, page+"/", "Page", prefix+page+"/")
	}
	body.WriteString("</ul>\n</section>\n")
}

func writeListItem(body *bytes.Buffer, name, kind, href string) {
	body.WriteString("<li><a href=\"")
	body.WriteString(html.EscapeString(href))
	body.WriteString("\">")
	body.WriteString(html.EscapeString(name))
	body.WriteString("</a><span>")
	body.WriteString(html.EscapeString(kind))
	body.WriteString("</span></li>\n")
}

func writeCatalog(target catalogTarget, stagingDir string, fileSystem filesystem.FS) error {
	temporary, err := fileSystem.CreateTemp(stagingDir, "new-*.html")
	if err != nil {
		return fmt.Errorf("create catalog staging file: %w", err)
	}
	temporaryPath := temporary.Name()
	cleanupTemporary := true
	defer func() {
		if cleanupTemporary {
			_ = fileSystem.Remove(temporaryPath)
		}
	}()
	if _, err := io.Copy(temporary, bytes.NewReader(target.content)); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write catalog staging file: %w", err)
	}
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("make catalog readable: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close catalog staging file: %w", err)
	}

	backupPath := filepath.Join(stagingDir, "old-"+randomHex()+"-"+target.key)
	hadTarget := false
	if _, err := fileSystem.Lstat(target.path); err == nil {
		hadTarget = true
		if err := fileSystem.Rename(target.path, backupPath); err != nil {
			return fmt.Errorf("stage old catalog %q: %w", target.path, err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect catalog %q: %w", target.path, err)
	}

	if err := fileSystem.Rename(temporaryPath, target.path); err != nil {
		if hadTarget {
			if restoreErr := fileSystem.Rename(backupPath, target.path); restoreErr != nil {
				return fmt.Errorf("install catalog %q: %w; restore old catalog: %v", target.path, err, restoreErr)
			}
		}
		return fmt.Errorf("install catalog %q: %w", target.path, err)
	}
	cleanupTemporary = false
	if hadTarget {
		if err := fileSystem.Remove(backupPath); err != nil {
			return fmt.Errorf("remove old catalog %q: %w", target.path, err)
		}
	}
	return nil
}

func recoverCatalogSwaps(publicRoot, stagingDir string, fileSystem filesystem.FS) error {
	entries, err := fileSystem.ReadDir(stagingDir)
	if err != nil {
		return fmt.Errorf("scan catalog staging: %w", err)
	}
	for _, entry := range entries {
		path := filepath.Join(stagingDir, entry.Name())
		if !strings.HasPrefix(entry.Name(), "old-") {
			_ = fileSystem.RemoveAll(path)
			continue
		}
		target, valid := catalogRecoveryTarget(publicRoot, entry.Name())
		if !valid {
			_ = fileSystem.RemoveAll(path)
			continue
		}
		if _, err := fileSystem.Lstat(target); errors.Is(err, fs.ErrNotExist) {
			if err := fileSystem.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("create catalog recovery directory: %w", err)
			}
			if err := fileSystem.Rename(path, target); err != nil {
				return fmt.Errorf("restore interrupted catalog: %w", err)
			}
		} else if err == nil {
			_ = fileSystem.RemoveAll(path)
		} else {
			return fmt.Errorf("inspect catalog recovery target: %w", err)
		}
	}
	return nil
}

func catalogRecoveryTarget(publicRoot, name string) (string, bool) {
	rest := strings.TrimPrefix(name, "old-")
	_, key, found := strings.Cut(rest, "-")
	if !found {
		return "", false
	}
	if key == "root" {
		return filepath.Join(publicRoot, "index.html"), true
	}
	identity, found := strings.CutPrefix(key, "identity-")
	if !found || !ValidName(identity) {
		return "", false
	}
	return filepath.Join(publicRoot, IdentityScope(identity), "index.html"), true
}
