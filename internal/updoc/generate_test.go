package updoc

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestIndexerRun(t *testing.T) {
	type args struct {
		path string
		fs   func() afero.Fs
	}

	type want struct {
		m   func() ([]byte, error)
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Successful": {
			reason: "Should be able to generate json from directory structure for files",
			args: args{
				path: "/docs",
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					fs = Write(t, fs, "/docs/1.md", "---\ntitle: Title\nweight: 99\n---")
					return fs
				},
			},
			want: want{
				m: func() ([]byte, error) {
					return json.MarshalIndent([]Item{{
						DisplayName: "Title",
						Location:    "1.md",
					}}, "", "\t")
				},
			},
		},
		"SuccessfulMultiLevel": {
			reason: "Should be able to generate json from directory structure for multi-directory files",
			args: args{
				path: "/docs",
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					fs = Write(t, fs, "/docs/advanced/_index.md", "---\nsection: Advanced\nweight: 1\n---")
					fs = Write(t, fs, "/docs/advanced/1.md", "---\ntitle: Title\nweight: 99\n---")
					fs = Write(t, fs, "/docs/1.md", "---\ntitle: Title 2\nweight: 1\n---")
					return fs
				},
			},
			want: want{
				m: func() ([]byte, error) {
					return json.MarshalIndent([]Item{{
						DisplayName: "Title 2",
						Location:    "1.md",
					}, {
						DisplayName: "Advanced/Title",
						Location:    "advanced/1.md",
					}}, "", "\t")
				},
			},
		},
		"SuccessfulMultiLevelOtherStuff": {
			reason: "Should be able to generate json from directory structure for multi-directory files with other content",
			args: args{
				path: "/docs",
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					fs = Write(t, fs, "/docs/advanced/_index.md", "---\nsection: Advanced\nweight: 1\n---")
					fs = Write(t, fs, "/docs/advanced/1.md", "---\ntitle: Title\nweight: 99\n---")
					fs = Write(t, fs, "/docs/1.md", "---\ntitle: Title 2\nweight: 1\n---")
					fs = Write(t, fs, "/docs/quickstart.sh", "#bash script here")
					fs = Write(t, fs, "/docs/image.gif", "image contents")
					return fs
				},
			},
			want: want{
				m: func() ([]byte, error) {
					return json.MarshalIndent([]Item{{
						DisplayName: "Title 2",
						Location:    "1.md",
					}, {
						DisplayName: "Advanced/Title",
						Location:    "advanced/1.md",
					}}, "", "\t")
				},
			},
		},
		"SuccessfulMultiLevelComplex": {
			reason: "Should be able to generate json from directory structure for multi-directory complex files with other content",
			args: args{
				path: "/docs",
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					fs = Write(t, fs, "/docs/advanced/_index.md", "---\nsection: Advanced\nweight: 1\n---")
					fs = Write(t, fs, "/docs/advanced/1.md", "---\ntitle: Title\nweight: 4\n---")
					fs = Write(t, fs, "/docs/advanced/4.md", "---\ntitle: Title 4\nweight: 99\n---")
					fs = Write(t, fs, "/docs/advanced/expert/_index.md", "---\nsection: Expert\nweight: 5\n---")
					fs = Write(t, fs, "/docs/advanced/expert/1.md", "---\ntitle: Title 5\nweight: 10\n---")
					fs = Write(t, fs, "/docs/advanced/expert/5.md", "---\ntitle: Title 5\nweight: 99\n---")
					fs = Write(t, fs, "/docs/advanced/expert/6.md", "---\ntitle: Title 6\nweight: 10\n---")
					fs = Write(t, fs, "/docs/1.md", "---\ntitle: Title 2\nweight: 1\n---")
					fs = Write(t, fs, "/docs/2.md", "---\ntitle: Title 7\nweight: 98\n---")
					fs = Write(t, fs, "/docs/3.md", "---\ntitle: Title 8\nweight: 10000000\n---")
					fs = Write(t, fs, "/docs/4.md", "---\ntitle: Title 9\nweight: -1\n---")
					fs = Write(t, fs, "/docs/quickstart.sh", "#bash script here")
					fs = Write(t, fs, "/docs/image.gif", "image contents")
					fs = Write(t, fs, "/docs/beginner/_index.md", "---\nsection: Beg\nweight: -1\n---")
					fs = Write(t, fs, "/docs/beginner/99.md", "---\ntitle: Title 99\nweight: 4\n---")
					return fs
				},
			},
			want: want{
				m: func() ([]byte, error) {
					return json.MarshalIndent([]Item{{
						DisplayName: "Title 9",
						Location:    "4.md",
					}, {
						DisplayName: "Beg/Title 99",
						Location:    "beginner/99.md",
					}, {
						DisplayName: "Title 2",
						Location:    "1.md",
					}, {
						DisplayName: "Advanced/Title",
						Location:    "advanced/1.md",
					}, {
						DisplayName: "Advanced/Expert/Title 5",
						Location:    "advanced/expert/1.md",
					}, {
						DisplayName: "Advanced/Expert/Title 6",
						Location:    "advanced/expert/6.md",
					}, {
						DisplayName: "Advanced/Expert/Title 5",
						Location:    "advanced/expert/5.md",
					}, {
						DisplayName: "Advanced/Title 4",
						Location:    "advanced/4.md",
					}, {
						DisplayName: "Title 7",
						Location:    "2.md",
					}, {
						DisplayName: "Title 8",
						Location:    "3.md",
					}}, "", "\t")
				},
			},
		},
		"ErrorCantFindDocs": {
			reason: "Should return an error if we can't find the docs location",
			args: args{
				path: "/docs",
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					return fs
				},
			},
			want: want{
				err: &fs.PathError{Op: "open", Path: "/docs", Err: errors.New("file does not exist")},
			},
		},
		"ErrorCantDetermineDisplayName": {
			reason: "Should return an error if we can't determine the display name for a doc",
			args: args{
				path: "/docs",
				fs: func() afero.Fs {
					fs := afero.NewMemMapFs()
					f, _ := fs.Create("/docs/.md")
					f.Close()
					return fs
				},
			},
			want: want{
				err: fmt.Errorf(errDisplayName, "/docs/.md"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fs := tc.args.fs()
			err := NewIndexer(tc.args.path, WithFs(fs)).Run()
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(...): -want err, +got err:\n%s", tc.reason, diff)
			}
			if tc.want.err != nil {
				return
			}
			b, err := afero.ReadFile(fs, filepath.Join(tc.args.path, indexFN))
			if err != nil {
				t.Error(err)
			}
			m, err := tc.want.m()
			if err != nil {
				t.Fatalf("invalid test JSON document: %s", err.Error())
			}
			if diff := cmp.Diff(string(m), string(b)); diff != "" {
				t.Errorf("\n%s\nRun(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}

}
