package llamacpp

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	assets "github.com/inference-gateway/inference-gateway/internal/platform/assets"
	logger "github.com/inference-gateway/inference-gateway/internal/platform/logger"
)

const (
	// CacheModelsDir is where GGUFs land, under <owner>/<repo>/, in the
	// shared ~/.infer cache. Exported because the 503 guidance names it.
	CacheModelsDir = assets.ModelsDir + "/llamacpp"

	// ManifestSuffix names the per-quant file set stored next to the GGUFs so
	// later starts and the /v1/models listing need no network.
	ManifestSuffix = ".manifest.json"

	// hubUserAgent is what llama.cpp sends: the Hub only adds the ggufFile and
	// mmprojFile fields to a manifest for llama.cpp's User-Agent.
	hubUserAgent     = "llama-cpp"
	hubManifestRoute = "/v2/%s/manifests/%s"
	hubResolveRoute  = "/%s/resolve/main/%s"
	// linkedETagHeader carries an LFS file's sha256 on a resolve response,
	// which is how shards the manifest does not list get verified.
	linkedETagHeader = "X-Linked-Etag"
	sha256HexLen     = 64

	ggufMode         = 0o644
	manifestMode     = 0o644
	cacheDirMode     = 0o755
	shardIndexFormat = "%s-%05d-of-%05d.gguf"
)

// HubBase is the Hugging Face host the manifest and resolve routes live on.
// A var (not a const) so tests can aim it at an httptest server.
var HubBase = "https://huggingface.co"

// shardName matches llama.cpp's split GGUF naming, <prefix>-00001-of-0000N.gguf.
var shardName = regexp.MustCompile(`^(.+)-(\d{5})-of-(\d{5})\.gguf$`)

// Manifest is the file set of one reference as the Hub describes it: the
// GGUF to load (its first shard when split), an optional multimodal
// projector, and every file to download, shards included.
type Manifest struct {
	Model  string `json:"model"`
	Mmproj string `json:"mmproj,omitempty"`
	Files  []File `json:"files"`
}

// File is one GGUF of a manifest. An empty SHA256 skips verification (the
// Hub exposes none for it).
type File struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256,omitempty"`
	Size   int64  `json:"size"`
}

type hubFile struct {
	Rfilename string `json:"rfilename"`
	Size      int64  `json:"size"`
	LFS       struct {
		SHA256 string `json:"sha256"`
	} `json:"lfs"`
}

type hubManifest struct {
	GGUF   *hubFile `json:"ggufFile"`
	Mmproj *hubFile `json:"mmprojFile"`
}

// Cache maps references to GGUF files under ~/.infer/models/llamacpp and
// fills it from the Hub, downloading only what is missing.
type Cache struct {
	dir    string
	logger logger.Logger
}

func newCache(home string, log logger.Logger) *Cache {
	return &Cache{dir: filepath.Join(home, CacheModelsDir), logger: log}
}

// RepoDir is where a reference's files live.
func (c *Cache) RepoDir(ref Reference) string {
	return filepath.Join(c.dir, ref.Owner, ref.Name)
}

// Path is the on-disk location of a manifest file name.
func (c *Cache) Path(ref Reference, name string) string {
	return filepath.Join(c.RepoDir(ref), filepath.FromSlash(name))
}

func (c *Cache) manifestPath(ref Reference) string {
	return filepath.Join(c.RepoDir(ref), ref.Quant+ManifestSuffix)
}

// Complete returns the stored manifest when every file it lists exists.
func (c *Cache) Complete(ref Reference) (Manifest, bool) {
	m, ok := c.stored(ref)
	if !ok {
		return Manifest{}, false
	}
	for _, f := range m.Files {
		if _, err := os.Stat(c.Path(ref, f.Name)); err != nil {
			return m, false
		}
	}
	return m, true
}

// Cached lists every reference whose files are all present, in path order.
func (c *Cache) Cached() []Reference {
	manifests, _ := filepath.Glob(filepath.Join(c.dir, "*", "*", "*"+ManifestSuffix))
	var refs []Reference
	for _, p := range manifests {
		ref := Reference{
			Owner: filepath.Base(filepath.Dir(filepath.Dir(p))),
			Name:  filepath.Base(filepath.Dir(p)),
			Quant: strings.TrimSuffix(filepath.Base(p), ManifestSuffix),
		}
		if _, ok := c.Complete(ref); ok {
			refs = append(refs, ref)
		}
	}
	return refs
}

// Ensure resolves ref (stored manifest first, then the Hub) and downloads
// the files that are missing, verifying each against its sha256.
func (c *Cache) Ensure(ctx context.Context, dl *assets.Downloader, ref Reference) (Manifest, error) {
	m, ok := c.stored(ref)
	if !ok {
		var err error
		if m, err = c.resolve(ctx, dl, ref); err != nil {
			return Manifest{}, err
		}
	}
	for _, f := range m.Files {
		dest := c.Path(ref, f.Name)
		if _, err := os.Stat(dest); err == nil {
			continue // an existing file is treated as done, whoever wrote it
		}
		if err := dl.Download(ctx, c.resolveURL(ref, f.Name), dest, f.SHA256, ggufMode); err != nil {
			return Manifest{}, fmt.Errorf("downloading %s: %w", f.Name, err)
		}
	}
	return m, nil
}

func (c *Cache) stored(ref Reference) (Manifest, bool) {
	body, err := os.ReadFile(c.manifestPath(ref))
	if err != nil {
		return Manifest{}, false
	}
	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil || m.Model == "" {
		return Manifest{}, false
	}
	return m, true
}

// resolve asks the Hub which files the quant tag names, expands split
// shards, and stores the result for later starts.
func (c *Cache) resolve(ctx context.Context, dl *assets.Downloader, ref Reference) (Manifest, error) {
	hub, err := c.fetchHubManifest(ctx, dl, ref)
	if err != nil {
		return Manifest{}, err
	}
	if hub.GGUF == nil || !safeRelative(hub.GGUF.Rfilename) {
		return Manifest{}, fmt.Errorf("the Hub manifest for %s names no usable GGUF", ref)
	}
	m := Manifest{Model: hub.GGUF.Rfilename, Files: []File{toFile(*hub.GGUF)}}
	shards, err := c.shards(ctx, dl, ref, hub.GGUF.Rfilename)
	if err != nil {
		return Manifest{}, err
	}
	m.Files = append(m.Files, shards...)
	if hub.Mmproj != nil && safeRelative(hub.Mmproj.Rfilename) {
		m.Mmproj = hub.Mmproj.Rfilename
		m.Files = append(m.Files, toFile(*hub.Mmproj))
	}
	for _, f := range m.Files {
		if f.SHA256 == "" {
			c.logger.Warn("llamacpp: the Hub exposes no sha256 for a file; it will not be verified", "model", ref.String(), "file", f.Name)
		}
	}
	if err := c.store(ref, m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func (c *Cache) fetchHubManifest(ctx context.Context, dl *assets.Downloader, ref Reference) (hubManifest, error) {
	url := HubBase + fmt.Sprintf(hubManifestRoute, ref.Repo(), ref.Quant)
	resp, err := dl.Get(ctx, url, http.Header{
		"User-Agent": {hubUserAgent},
		"Accept":     {"application/json"},
	})
	if err != nil {
		return hubManifest{}, fmt.Errorf("resolving %s on the Hub (does the repo have a %s GGUF?): %w", ref, ref.Quant, err)
	}
	defer resp.Body.Close()
	var hub hubManifest
	if err := json.NewDecoder(resp.Body).Decode(&hub); err != nil {
		return hubManifest{}, fmt.Errorf("decoding the Hub manifest for %s: %w", ref, err)
	}
	return hub, nil
}

// shards lists the remaining pieces of a split GGUF, which the manifest
// leaves out, with the sha256 the resolve route advertises for each.
func (c *Cache) shards(ctx context.Context, dl *assets.Downloader, ref Reference, first string) ([]File, error) {
	match := shardName.FindStringSubmatch(first)
	if match == nil {
		return nil, nil
	}
	var index, total int
	if _, err := fmt.Sscanf(match[2]+" "+match[3], "%d %d", &index, &total); err != nil || index != 1 {
		return nil, fmt.Errorf("%s has an unexpected split numbering", first)
	}
	files := make([]File, 0, total-1)
	for i := 2; i <= total; i++ {
		name := fmt.Sprintf(shardIndexFormat, match[1], i, total)
		sha, err := c.linkedSHA256(ctx, dl, c.resolveURL(ref, name))
		if err != nil {
			return nil, fmt.Errorf("checking shard %s: %w", name, err)
		}
		files = append(files, File{Name: name, SHA256: sha})
	}
	return files, nil
}

// linkedSHA256 reads the LFS sha256 the Hub advertises on a resolve
// redirect, without following it. Empty when the Hub exposes none.
func (c *Cache) linkedSHA256(ctx context.Context, dl *assets.Downloader, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return "", err
	}
	dl.Apply(req.Header)
	client := &http.Client{
		Transport:     dl.Client().Transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("%s responded %s", url, resp.Status)
	}
	sha := strings.Trim(resp.Header.Get(linkedETagHeader), `"`)
	if _, err := hex.DecodeString(sha); err != nil || len(sha) != sha256HexLen {
		return "", nil
	}
	return sha, nil
}

func (c *Cache) store(ref Reference, m Manifest) error {
	dest := c.manifestPath(ref)
	if err := os.MkdirAll(filepath.Dir(dest), cacheDirMode); err != nil {
		return err
	}
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), "."+filepath.Base(dest)+".part")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(body); err != nil {
		return errors.Join(err, tmp.Close())
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), manifestMode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dest)
}

func (c *Cache) resolveURL(ref Reference, name string) string {
	return HubBase + fmt.Sprintf(hubResolveRoute, ref.Repo(), name)
}

func toFile(f hubFile) File {
	return File{Name: f.Rfilename, SHA256: f.LFS.SHA256, Size: f.Size}
}

// safeRelative accepts a Hub file name only when it stays inside the repo
// directory once joined to it.
func safeRelative(name string) bool {
	return name != "" && path.Clean(name) == name && !path.IsAbs(name) && name != ".." && !strings.HasPrefix(name, "../")
}
