package updoc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/spf13/afero"
	"google.golang.org/api/option"
)

const (
	bucketIndexFileName = "upbound_%s_%s_index.json"
)

// BucketIndex represents an index file in the storage bucket.
type BucketIndex struct {
	Items []Item            `json:"tableOfContents"`
	Meta  map[string]string `json:"metadata"`
}

// UploadManager represents a document uploader capable of
// uploading contents of the specified io.Reader
// under the specified name.
type UploadManager struct {
	upload func(ctx context.Context, bucket *storage.BucketHandle, name string, r io.Reader) error
}

// Option is an option that modifies a UploadManager
type Option func(u *UploadManager)

// New constructs a new UploadManager
func New(opts ...Option) *UploadManager {
	u := &UploadManager{
		upload: upload,
	}

	for _, o := range opts {
		o(u)
	}

	return u
}

// WithUpload sets the `upload` function to be used by the UploadManager.
func WithUpload(up func(ctx context.Context, bucket *storage.BucketHandle, name string, r io.Reader) error) Option {
	return func(u *UploadManager) {
		u.upload = up
	}
}

// ProcessIndex processes the index documents and uploads them to the storage.
func (u *UploadManager) ProcessIndex(opts UploadOptions, afs afero.Fs) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()

	c, err := storage.NewClient(ctx, option.WithScopes(storage.ScopeReadWrite))
	if err != nil {
		return err
	}
	b := c.Bucket(opts.BucketName)

	// META
	metadata, err := u.getMeta(ctx, b, opts.CDNDomain, afs, opts.DocsDir)
	if err != nil {
		return err
	}

	// INDEX
	index, err := u.getIndex(afs, opts.DocsDir, metadata)
	if err != nil {
		return err
	}

	jb, err := json.MarshalIndent(BucketIndex{
		Items: index,
		Meta:  metadata,
	}, "", "\t")
	if err != nil {
		return err
	}
	return upload(ctx, b, fmt.Sprintf(bucketIndexFileName, opts.Name, opts.Version), bytes.NewReader(jb))
}

func (u *UploadManager) getIndex(afs afero.Fs, dir string, meta map[string]string) ([]Item, error) {
	n := filepath.Join(dir, indexFN)
	file, err := afs.Open(n)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Failed to close file %q: %s\n", n, err.Error())
		}
	}()

	var index = make([]Item, 0)
	err = json.NewDecoder(file).Decode(&index)
	if err != nil {
		return nil, err
	}
	for x := range index {
		index[x].Location = meta[strings.TrimPrefix(index[x].Location, path.Clean(dir)+"/")]
	}
	return index, nil
}

func (u *UploadManager) getMeta(ctx context.Context, bucket *storage.BucketHandle, cdn string, afs afero.Fs, dir string) (map[string]string, error) {
	meta := make(map[string]string)
	if err := afero.Walk(afs, dir, func(p string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			if info.Name() == indexFN || info.Name() == sectionFN {
				return nil
			}
			b, err := afero.ReadFile(afs, p)
			if err != nil {
				return err
			}
			bl, err := u.hashAndUpload(ctx, bucket, cdn, filepath.Ext(p), b)
			if err != nil {
				return err
			}
			relative := strings.TrimPrefix(p, path.Clean(dir)+"/")
			meta[relative] = bl
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return meta, nil

}

func (u *UploadManager) hashAndUpload(ctx context.Context, bucket *storage.BucketHandle, cdn string, ext string, fb []byte) (string, error) {
	h, err := hash(fb)
	if err != nil {
		return "", err
	}

	bn := fmt.Sprintf("%s%s", h, ext)

	if err := u.upload(ctx, bucket, bn, bytes.NewReader(fb)); err != nil {
		return "", err
	}

	l, err := url.Parse(cdn)
	if err != nil {
		return "", err
	}
	l.Path = path.Join(l.Path, bn)

	return l.String(), nil
}

func upload(ctx context.Context, bucket *storage.BucketHandle, name string, r io.Reader) error {
	w := bucket.Object(name).NewWriter(ctx)
	defer func() {
		if err := w.Close(); err != nil {
			log.Printf("Failed to upload %q: %s\n", name, err.Error())
		}
	}()
	switch filepath.Ext(name) {
	case ".md":
		w.ContentType = "text/markdown" // https://www.rfc-editor.org/rfc/rfc7763
	case ".json":
		w.ContentType = "application/json"
	} // otherwise gcp will attempt to infer from extension
	if _, err := io.Copy(w, r); err != nil {
		return err
	}
	return nil
}

func hash(fb []byte) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, bytes.NewReader(fb)); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
