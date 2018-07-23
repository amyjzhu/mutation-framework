package mutesting

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/loader"
	"path/filepath"
	"github.com/spf13/afero"
	log "github.com/sirupsen/logrus"
)

var fs = afero.NewOsFs()
var afs = &afero.Afero{Fs: fs}

// ParseFile parses the content of the given file and returns the corresponding ast.File node and its file set for positional information.
// If a fatal error is encountered the error return argument is not nil.
func ParseFile(file string) (*ast.File, *token.FileSet, error) {
	data, err := afs.ReadFile(file)
	if err != nil {
		fmt.Println("error in ParseFile")
		return nil, nil, err
	}

	return ParseSource(data)
}

func LoadFile(file string) (data []byte, err error) {
	data, err = afs.ReadFile(file)
	if err != nil {
		fmt.Println("error in LoadFile")
		return nil, err
	}
	return data, nil
}

// ParseSource parses the given source and returns the corresponding ast.File node and its file set for positional information.
// If a fatal error is encountered the error return argument is not nil.
func ParseSource(data interface{}) (*ast.File, *token.FileSet, error) {
	fset := token.NewFileSet()

	src, err := parser.ParseFile(fset, "", data, parser.ParseComments|parser.AllErrors)
	if err != nil {
		fmt.Println("error in ParseSource")
		return nil, nil, err
	}

	return src, fset, err
}

// ParseAndTypeCheckFile parses and type-checks the given file, and returns everything interesting about the file.
// If a fatal error is encountered the error return argument is not nil.
func ParseAndTypeCheckFile(file string) (*ast.File, *token.FileSet, *types.Package, *types.Info, error) {
	fileAbs, err := filepath.Abs(file)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not absolute the file path of %q: %v", file, err)
	}
	dir := filepath.Dir(fileAbs)

	buildPkg, err := build.ImportDir(dir, build.FindOnly)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not create build package of %q: %v", file, err)
	}

	var conf = loader.Config{
		ParserMode: parser.AllErrors | parser.ParseComments,
	}

	if buildPkg.ImportPath != "." {
		conf.Import(buildPkg.ImportPath)
	} else {
		// This is most definitely the case for files inside a "testdata" package
		conf.CreateFromFilenames(dir, fileAbs)
	}

	prog, err := conf.Load()
	if err != nil {
		log.Error("Error in ParseAndTypeCheckFile.")
		return nil, nil, nil, nil, fmt.Errorf("could not load package of file %q: %v", file, err)
	}

	pkgInfo := prog.InitialPackages()[0]

	var src *ast.File
	for _, f := range pkgInfo.Files {
		if prog.Fset.Position(f.Pos()).Filename == fileAbs {
			src = f

			break
		}
	}

	return src, prog.Fset, pkgInfo.Pkg, &pkgInfo.Info, nil
}
