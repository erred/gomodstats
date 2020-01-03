package gomodstats

import (
	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/vcs"
)

type Store struct {
	// map of module names to module versions
	Mods map[string][]Module
}

type Module struct {
	// canonical name of module
	Name string
	// odule version
	Version string
	// time of index
	Indexed string

	// url to download from
	RepoRoot *vcs.RepoRoot
	// go.mod file
	ModFile *modfile.File

	// is the module proxied
	Proxied bool
	// list of packages
	Pkgs []Package
	// list of module imports
	Folders []string
}

type Package struct {
	// import path of package (dir name)
	ImportPath string
	// actual name of package
	Name string
	// packages imported by this package
	PkgImports []string
	// filenames in package
	Files []string
	// list of exported functions
	ExportedFuncs []string
	// list of unexported functions
	PrivateFuncs []string
	// lines of code
	CodeLines int64
}
