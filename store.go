package gomodstats

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
	VcsUrl string
	// is the module proxied
	Proxied bool
	// list of packages
	Pkgs []Package
	// list of module imports
	ModImports []string
	// list of subfolders
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
