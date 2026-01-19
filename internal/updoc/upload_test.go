package updoc

import (
	"context"
	"io"
	"testing"

	"github.com/pkg/errors"

	"cloud.google.com/go/storage"
	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
)

func TestProcessMeta(t *testing.T) {
	type args struct {
		cdn  string
		path string
		fs   func() afero.Fs
		up   func(ctx context.Context, bucket *storage.BucketHandle, name string, r io.Reader) error
	}

	type want struct {
		meta map[string]string
		err  error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessfulSimple": {
			reason: "Should be able to process simple markdown file",
			args: args{
				cdn:  "http://upbound.io/",
				path: "/docs",
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					fs = Write(t, fs, "/docs/1.md", `1`)
					return fs
				},
				up: func(ctx context.Context, bucket *storage.BucketHandle, name string, r io.Reader) error {
					b, _ := io.ReadAll(r)
					if string(b) != `1` {
						return errors.Errorf("file contents incorrect +got:%s, -want:%s", string(b), `1`)
					}
					return nil
				},
			},
			want: want{
				meta: map[string]string{
					"1.md": "http://upbound.io/6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b.md",
				},
			},
		},
		"SuccessfulComplex": {
			reason: "Should be able to process simple markdown file",
			args: args{
				cdn:  "http://upbound.io/",
				path: "/docs",
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					fs = Write(t, fs, "/docs/_index.md", ``)
					fs = Write(t, fs, "/docs/Basic/_index.md", `1`)
					fs = Write(t, fs, "/docs/Advanced/_index.md", `1`)
					fs = Write(t, fs, "/docs/1.md", `1`)
					fs = Write(t, fs, "/docs/Basic/1.md", `1`)
					fs = Write(t, fs, "/docs/Advanced/1.md", `1`)
					return fs
				},
				up: func(ctx context.Context, bucket *storage.BucketHandle, name string, r io.Reader) error {
					b, _ := io.ReadAll(r)
					if string(b) != `1` {
						return errors.Errorf("file contents incorrect +got:%s, -want:%s", string(b), `1`)
					}
					return nil
				},
			},
			want: want{
				meta: map[string]string{
					"1.md":          "http://upbound.io/6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b.md",
					"Advanced/1.md": "http://upbound.io/6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b.md",
					"Basic/1.md":    "http://upbound.io/6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b.md",
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			u := New(WithUpload(tc.args.up))
			got, err := u.getMeta(context.Background(), nil, tc.args.cdn, tc.args.fs(), tc.args.path)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nprocess(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.meta, got); diff != "" {
				t.Errorf("\n%s\nprocess(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}

}

func TestProcessIndex(t *testing.T) {
	type args struct {
		cdn  string
		path string
		fs   func() afero.Fs
		up   func(ctx context.Context, bucket *storage.BucketHandle, name string, r io.Reader) error
		meta map[string]string
	}

	type want struct {
		index []Item
		err   error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessfulSimple": {
			reason: "Should be able to process index file using uploaded metadata",
			args: args{
				cdn:  "http://upbound.io/",
				path: "/docs",
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					fs = Write(t, fs, "/docs/index.json", `[
						{"name": "Configuration", "location": "/docs/Configuration.md"},
						{"name": "Quickstart/Accounts", "location": "/docs/qblah/Accounts.md"},
						{"name": "Quickstart/Kubernetes", "location": "/docs/qblah/k8s.md"},
						{"name": "Quickstart/Advanced/Config", "location": "/docs/qblah/a/con.md"},
						{"name": "Quickstart", "location": "/docs/Quickstart.md"},
						{"name": "Beginner/Resource", "location": "/docs/blah/r1.md"},
						{"name": "Beginner/Secrets", "location": "/docs/blah/s1.md"}
					]`)
					return fs
				},
				up: func(ctx context.Context, bucket *storage.BucketHandle, name string, r io.Reader) error {
					b, _ := io.ReadAll(r)
					if string(b) != `1` {
						return errors.Errorf("file contents incorrect +got:%s, -want:%s", string(b), `1`)
					}
					return nil
				},
				meta: map[string]string{
					"blah/r1.md":        "http://upbound.io/1.md",
					"blah/s1.md":        "http://upbound.io/2.md",
					"qblah/Accounts.md": "http://upbound.io/3.md",
					"qblah/k8s.md":      "http://upbound.io/4.md",
					"Configuration.md":  "http://upbound.io/5.md",
					"Quickstart.md":     "http://upbound.io/6.md",
					"qblah/a/con.md":    "http://upbound.io/7.md",
				},
			},
			want: want{
				index: []Item{
					{DisplayName: "Configuration", Location: "http://upbound.io/5.md"},
					{DisplayName: "Quickstart/Accounts", Location: "http://upbound.io/3.md"},
					{DisplayName: "Quickstart/Kubernetes", Location: "http://upbound.io/4.md"},
					{DisplayName: "Quickstart/Advanced/Config", Location: "http://upbound.io/7.md"},
					{DisplayName: "Quickstart", Location: "http://upbound.io/6.md"},
					{DisplayName: "Beginner/Resource", Location: "http://upbound.io/1.md"},
					{DisplayName: "Beginner/Secrets", Location: "http://upbound.io/2.md"},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			u := New(WithUpload(tc.args.up))
			got, err := u.getIndex(tc.args.fs(), tc.args.path, tc.args.meta)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nprocess(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.index, got); diff != "" {
				t.Errorf("\n%s\nprocess(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}

}

func Write(t *testing.T, fs afero.Fs, fn string, content string) afero.Fs {
	f, _ := fs.Create(fn)
	defer func() {
		_ = f.Close()
	}()

	if _, err := f.WriteString(content); err != nil {
		t.Error(err.Error())
	}
	return fs
}
