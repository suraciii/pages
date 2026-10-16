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
	"strconv"
	"strings"
	"time"

	"github.com/suraciii/pages/internal/filesystem"
)

const (
	catalogStagingName = "catalog-staging"
	catalogLockName    = "catalog.lock"
	catalogMarker      = "<!-- pages:page-index -->"
	catalogRecordName  = "index.json"
	// catalogRecordMark is the serialized prefix of every Page Record: the
	// opening brace followed by the pages key. A foreign `index.json` that
	// does not start this way is preserved instead of being replaced.
	catalogRecordMark = "{\n  \"pages\": ["
	// publishedTimeLayout is the reader-facing Published Time form. It has
	// minute precision and no zone: a reader sees the same wall-clock time
	// that `ls -l` shows on the server, and a local Index never mixes zones.
	publishedTimeLayout = "2006-01-02 15:04"
)

type catalogScope struct {
	identity string
	pages    []catalogPage
}

type catalogPage struct {
	slug    string
	modTime time.Time
}

type catalogTarget struct {
	key     string
	path    string
	marker  string
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
	if err := ensureCatalogTargetsSafe(targets, fileSystem); err != nil {
		return err
	}
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
	rootPages := make([]catalogPage, 0)
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
		if ValidName(name) {
			if page, ok := scanCatalogPage(publicRoot, name, fileSystem); ok {
				rootPages = append(rootPages, page)
			}
		}
	}
	sortCatalogPages(rootPages)
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

func scanIdentityPages(publicRoot, identity string, fileSystem filesystem.FS) ([]catalogPage, error) {
	identityDir := filepath.Join(publicRoot, IdentityScope(identity))
	entries, err := fileSystem.ReadDir(identityDir)
	if err != nil {
		return nil, fmt.Errorf("scan identity %q: %w", identity, err)
	}
	pages := make([]catalogPage, 0)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || !entry.IsDir() || !ValidName(name) {
			continue
		}
		if page, ok := scanCatalogPage(identityDir, name, fileSystem); ok {
			pages = append(pages, page)
		}
	}
	sortCatalogPages(pages)
	return pages, nil
}

func scanCatalogPage(scopeDir, slug string, fileSystem filesystem.FS) (catalogPage, bool) {
	info, err := fileSystem.Stat(filepath.Join(scopeDir, slug, "index.html"))
	if err != nil || !info.Mode().IsRegular() {
		return catalogPage{}, false
	}
	return catalogPage{slug: slug, modTime: info.ModTime()}, true
}

func sortCatalogPages(pages []catalogPage) {
	sort.Slice(pages, func(left, right int) bool {
		if pages[left].modTime.Equal(pages[right].modTime) {
			return pages[left].slug < pages[right].slug
		}
		return pages[left].modTime.After(pages[right].modTime)
	})
}

func makeCatalogTargets(publicRoot string, scopes []catalogScope) []catalogTarget {
	targets := make([]catalogTarget, 0, len(scopes)*2)
	identities := scopes[1:]
	for _, scope := range scopes {
		if scope.identity == "" {
			targets = append(targets,
				catalogTarget{
					key:     "root",
					path:    filepath.Join(publicRoot, "index.html"),
					marker:  catalogMarker,
					content: renderRootIndex(scope.pages, identities),
				},
				catalogTarget{
					key:     "root-record",
					path:    filepath.Join(publicRoot, catalogRecordName),
					marker:  catalogRecordMark,
					content: renderScopeRecord(scope.pages),
				},
			)
			continue
		}
		targets = append(targets,
			catalogTarget{
				key:     "identity-" + scope.identity,
				path:    filepath.Join(publicRoot, IdentityScope(scope.identity), "index.html"),
				marker:  catalogMarker,
				content: renderIdentityIndex(scope.identity, scope.pages, identities),
			},
			catalogTarget{
				key:     "record-identity-" + scope.identity,
				path:    filepath.Join(publicRoot, IdentityScope(scope.identity), catalogRecordName),
				marker:  catalogRecordMark,
				content: renderScopeRecord(scope.pages),
			},
		)
	}
	return targets
}

func renderRootIndex(pages []catalogPage, identities []catalogScope) []byte {
	var body bytes.Buffer
	writeDocumentStart(&body, "Pages", "Pages")
	if len(identities) > 0 {
		writeIdentityNavigation(&body, "", identities)
	}
	writePageList(&body, "Default Identity", pages, "./")
	writeDocumentEnd(&body)
	return body.Bytes()
}

func renderIdentityIndex(identity string, pages []catalogPage, identities []catalogScope) []byte {
	var body bytes.Buffer
	title := "Pages / @" + identity
	writeDocumentStart(&body, title, title)
	if len(identities) > 0 {
		writeIdentityNavigation(&body, identity, identities)
	}
	body.WriteString("<nav aria-label=\"Breadcrumb\"><a href=\"../\">Pages</a> / @")
	body.WriteString(html.EscapeString(identity))
	body.WriteString("</nav>\n")
	writePageList(&body, "Pages", pages, "./")
	writeDocumentEnd(&body)
	return body.Bytes()
}

// renderScopeRecord renders the Page Record for one scope. The Record
// carries the same Pages in the same order as its Index. It names no scope:
// the file path already says which scope it belongs to.
func renderScopeRecord(pages []catalogPage) []byte {
	var body bytes.Buffer
	body.WriteString("{\n  \"pages\": [")
	for index, page := range pages {
		if index > 0 {
			body.WriteString(",")
		}
		body.WriteString("\n    {\"slug\": ")
		body.WriteString(strconv.Quote(page.slug))
		body.WriteString(", \"published\": ")
		body.WriteString(strconv.Quote(page.modTime.Format(time.RFC3339)))
		body.WriteString("}")
	}
	if len(pages) > 0 {
		body.WriteString("\n  ")
	}
	body.WriteString("]\n}\n")
	return body.Bytes()
}

// writeIdentityNavigation lists the scopes a reader can jump to. Each entry
// is a name and a link; the current scope is not labeled, because a reader
// is already on it.
func writeIdentityNavigation(body *bytes.Buffer, currentIdentity string, identities []catalogScope) {
	body.WriteString("<nav aria-label=\"Identity navigation\"><ul>\n")
	defaultHref := "./"
	if currentIdentity != "" {
		defaultHref = "../"
	}
	writeListItem(body, "Default Identity", defaultHref)
	for _, identity := range identities {
		name := "@" + identity.identity + "/"
		href := "./" + IdentityScope(identity.identity) + "/"
		if currentIdentity != "" {
			href = "../" + IdentityScope(identity.identity) + "/"
		}
		if identity.identity == currentIdentity {
			href = "./"
		}
		writeListItem(body, name, href)
	}
	body.WriteString("</ul></nav>\n")
}

func writeDocumentStart(body *bytes.Buffer, title, heading string) {
	body.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n")
	body.WriteString(catalogMarker + "\n")
	body.WriteString("<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	body.WriteString("<title>")
	body.WriteString(html.EscapeString(title))
	body.WriteString("</title>\n<style>body{font:16px system-ui,sans-serif;line-height:1.5;max-width:52rem;margin:3rem auto;padding:0 1rem;color:#202124}a{color:#1257a6;text-decoration:none}a:hover{text-decoration:underline}ul{list-style:none;padding:0;border-top:1px solid #d9d9d9}li{display:flex;gap:1rem;padding:.65rem 0;border-bottom:1px solid #d9d9d9}li a{flex:1}li span{color:#6b7280;font-size:.9rem}li time{color:#6b7280;font-size:.9rem;font-variant-numeric:tabular-nums}nav{color:#6b7280;margin-bottom:1.5rem}h1{font-size:2rem;font-weight:600}h2{font-size:1.1rem;margin-top:2rem}</style>\n")
	body.WriteString("</head>\n<body>\n<main>\n<h1>")
	body.WriteString(html.EscapeString(heading))
	body.WriteString("</h1>\n")
}

func ensureCatalogTargetsSafe(targets []catalogTarget, fileSystem filesystem.FS) error {
	for _, target := range targets {
		if _, err := fileSystem.Lstat(target.path); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return fmt.Errorf("inspect existing catalog %q: %w", target.path, err)
		}
		content, err := fileSystem.ReadFile(target.path)
		if err != nil || !bytes.Contains(content, []byte(target.marker)) {
			if err != nil {
				return fmt.Errorf("refusing to replace existing catalog %q: %w", target.path, err)
			}
			return fmt.Errorf("refusing to replace existing catalog %q: move the existing file first", target.path)
		}
	}
	return nil
}

func writeDocumentEnd(body *bytes.Buffer) {
	body.WriteString("</main>\n</body>\n</html>\n")
}

func writePageList(body *bytes.Buffer, heading string, pages []catalogPage, prefix string) {
	body.WriteString("<section>\n<h2>")
	body.WriteString(html.EscapeString(heading))
	body.WriteString("</h2>\n")
	if len(pages) == 0 {
		body.WriteString("<p>No Pages published yet.</p>\n</section>\n")
		return
	}
	body.WriteString("<ul>\n")
	for _, page := range pages {
		writePageListItem(body, page, prefix+page.slug+"/")
	}
	body.WriteString("</ul>\n</section>\n")
}

func writePageListItem(body *bytes.Buffer, page catalogPage, href string) {
	published := page.modTime.Format(publishedTimeLayout)
	body.WriteString("<li><a href=\"")
	body.WriteString(html.EscapeString(href))
	body.WriteString("\">")
	body.WriteString(html.EscapeString(page.slug + "/"))
	body.WriteString("</a><time datetime=\"")
	body.WriteString(html.EscapeString(page.modTime.Format(time.RFC3339)))
	body.WriteString("\">")
	body.WriteString(html.EscapeString(published))
	body.WriteString("</time></li>\n")
}

func writeListItem(body *bytes.Buffer, name, href string) {
	body.WriteString("<li><a href=\"")
	body.WriteString(html.EscapeString(href))
	body.WriteString("\">")
	body.WriteString(html.EscapeString(name))
	body.WriteString("</a></li>\n")
}

func writeCatalog(target catalogTarget, stagingDir string, fileSystem filesystem.FS) error {
	// A refresh that produces byte-identical output writes nothing: the
	// artifact keeps its inode, so a reader never sees a needless rewrite
	// and a concurrent reader cannot observe a missing file.
	if existing, err := fileSystem.ReadFile(target.path); err == nil && bytes.Equal(existing, target.content) {
		return nil
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect existing catalog %q: %w", target.path, err)
	}
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
	// The random hex never contains a hyphen, so the first hyphen ends it and
	// the remainder is the target key. An Index key is `identity-<name>` and a
	// Record key is `record-identity-<name>`. The Record form is tested first
	// because its prefix is longer, which keeps the two forms unambiguous for
	// every valid Identity name — including names like `record-bumble`.
	_, key, found := strings.Cut(rest, "-")
	if !found {
		return "", false
	}
	switch key {
	case "root":
		return filepath.Join(publicRoot, "index.html"), true
	case "root-record":
		return filepath.Join(publicRoot, catalogRecordName), true
	}
	if identity, found := strings.CutPrefix(key, "record-identity-"); found {
		if !ValidName(identity) {
			return "", false
		}
		return filepath.Join(publicRoot, IdentityScope(identity), catalogRecordName), true
	}
	if identity, found := strings.CutPrefix(key, "identity-"); found {
		if !ValidName(identity) {
			return "", false
		}
		return filepath.Join(publicRoot, IdentityScope(identity), "index.html"), true
	}
	return "", false
}
