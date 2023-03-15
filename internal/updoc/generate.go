package updoc

import (
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/adrg/frontmatter"
	"github.com/spf13/afero"
)

const (
	indexFN        = "index.json"
	sectionFN      = "_index.md"
	errDisplayName = "unable to find meta for %s"
)

// Sortable represents a sortable document section, like the title or a section
// of a document.
type Sortable interface {
	w() int
	d() string
	l() string
}

// Title is the title of a document.
type Title struct {
	Title        string `yaml:"title"`
	Weight       int    `yaml:"weight"`
	FileLocation string
}

func (t *Title) w() int {
	return t.Weight
}
func (t *Title) d() string {
	return t.Title
}
func (t *Title) l() string {
	return t.FileLocation
}

// Section is the section in a document.
type Section struct {
	Section string `yaml:"section"`
	Weight  int    `yaml:"weight"`
	Items   []Sortable
}

func (s *Section) w() int {
	return s.Weight
}
func (s *Section) d() string {
	return s.Section
}
func (s *Section) l() string {
	return ""
}

// Item represents an item that will ultimately be represented in the uploaded
// table of contents.
type Item struct {
	DisplayName string `json:"name"`
	Location    string `json:"location"`
}

// Indexer indexes docs.
type Indexer struct {
	fs   afero.Fs
	root string
}

// IndexerOpt is an indexer option.
type IndexerOpt func(i *Indexer)

// WithFs sets the Indexer file system.
func WithFs(fs afero.Fs) IndexerOpt {
	return func(i *Indexer) {
		i.fs = fs
	}
}

// NewIndexer constructs an indexer at the specified root.
func NewIndexer(root string, opts ...IndexerOpt) *Indexer {
	i := &Indexer{
		fs:   afero.NewOsFs(),
		root: filepath.Clean(root),
	}
	for _, o := range opts {
		o(i)
	}
	return i
}

// Run runs the indexer.
func (i *Indexer) Run() error {
	dt, err := i.processDir(i.root)
	if err != nil {
		return err
	}

	jb, err := json.MarshalIndent(flatten(*dt, "", make([]Item, 0)), "", "\t")
	if err != nil {
		return err
	}

	return afero.WriteFile(i.fs, filepath.Join(i.root, indexFN), jb, 0777)
}

func flatten(s Section, p string, a []Item) []Item {
	for _, i := range s.Items {
		switch v := i.(type) {
		case *Section:
			a = flatten(*v, path.Join(p, v.Section), a)
		case *Title:
			a = append(a, Item{DisplayName: path.Join(p, i.d()), Location: i.l()})
		}
	}
	return a
}

func (i *Indexer) processDir(dir string) (*Section, error) {
	fi, err := afero.ReadDir(i.fs, dir)
	if err != nil {
		return nil, err
	}
	section := Section{}
	if exists, _ := afero.Exists(i.fs, path.Join(dir, sectionFN)); exists {
		s, err := getMeta(i.fs, path.Join(dir, sectionFN), &section)
		if err != nil {
			return nil, err
		}
		section = *s
	}
	for _, e := range fi {
		if e.IsDir() {
			sec, err := i.processDir(path.Join(dir, e.Name()))
			if err != nil {
				return nil, err
			}
			section.Items = append(section.Items, sec)
		}
		if e.Name() == sectionFN || filepath.Ext(e.Name()) != ".md" {
			continue
		}

		item, err := getMeta(i.fs, path.Join(dir, e.Name()), &Title{
			// TODO(hasheddan): consider constructing directory from
			// prefix when opening file rather than trimming prefix.
			FileLocation: strings.TrimPrefix(path.Join(dir, e.Name()), i.root+"/"),
		})
		if err != nil {
			return nil, err
		}
		section.Items = append(section.Items, item)
	}
	sort.SliceStable(section.Items, func(i, j int) bool {
		return section.Items[i].w() < section.Items[j].w()
	})
	return &section, nil
}

func getMeta[T Sortable](afs afero.Fs, path string, i T) (T, error) {
	// get display name, section and weight from file meta
	r, err := afs.Open(path)
	if err != nil {
		return i, err
	}

	_, err = frontmatter.Parse(r, &i)
	if err != nil {
		return i, err
	}
	if i.d() == "" {
		return i, fmt.Errorf(errDisplayName, path)
	}

	return i, nil
}
