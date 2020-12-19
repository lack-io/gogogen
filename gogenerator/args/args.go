// Copyright 2020 The kubernetes Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package args

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	ccli "github.com/lack-io/cli"

	"github.com/lack-io/gogogen/gogenerator/generator"
	"github.com/lack-io/gogogen/gogenerator/namer"
	"github.com/lack-io/gogogen/gogenerator/parser"
	"github.com/lack-io/gogogen/gogenerator/types"
)

// Default returns a defaulted GeneratorArgs. You may change the defaults
// before calling AddFlags.
func Default() *GeneratorArgs {
	return &GeneratorArgs{
		OutputBase:                 DefaultSourceTree(),
		GoHeaderFilePath:           filepath.Join(DefaultSourceTree(), "github.com/lack-io/gogogen/gogenerator/boilerplate/boilerplate.go.txt"),
		GeneratedBuildTag:          "ignore_autogenerated",
		GeneratedByCommentTemplate: "// Code generated by GENERATOR_NAME. Do NOT EDIT.",
		defaultCommandLineFlags:    true,
	}
}

// GeneratorArgs has arguments that are passed to generators.
type GeneratorArgs struct {
	// Which directories to parse.
	InputDirs []string

	// Source tree to write results to.
	OutputBase string

	// Package path within the source tree.
	OutputPackagePath string

	// Output file name.
	OutputFileBaseName string

	// Where to get copyright header text.
	GoHeaderFilePath string

	// If GeneratedByCommentTemplate is set, generator a "Code generated by" comment
	// below the boilerplate, of the format defined by this string.
	// Any instances of "GENERATOR_NAME" will be replaced with the name of the code generator
	GeneratedByCommentTemplate string

	// If true, only verify, don't write anything.
	VerifyOnly bool

	// If true, include *_test.go files
	IncludeTestFile bool

	// GeneratedBuildTag is the tag used to identify code generated by execution
	// of the type. Each generator should use a different tag, and different
	// groups of generators (external API that depends on vine generators) should
	// keep tags distinct as well
	GeneratedBuildTag string

	// Any custom arguments go here
	CustomArgs interface{}

	// Whether to use default command line flags
	defaultCommandLineFlags bool
}

// WithoutDefaultFlagParsing disables implicit addition of command line flags and parsing.
func (g *GeneratorArgs) WithoutDefaultFlagParsing() *GeneratorArgs {
	g.defaultCommandLineFlags = false
	return g
}

func (g *GeneratorArgs) AddFlags(app *ccli.App) {
	app.StringSliceVarP(&g.InputDirs, "input-dirs", "i", g.InputDirs,
		"Comma-separated list of import paths to get input type from.", "")
	app.StringVarP(&g.OutputBase, "output-base", "o", g.OutputBase,
		"Output base; defaults to $GOPATH/src/ or ./ if $GOPATH is not set.", "")
	app.StringVarP(&g.OutputPackagePath, "output-package", "p", g.OutputPackagePath,
		"Base package path.", "")
	app.StringVarP(&g.OutputFileBaseName, "output-file-base", "O", g.OutputFileBaseName,
		"Base name (without .go suffix) for output files.", "")
	app.StringVarP(&g.GoHeaderFilePath, "go-header-file", "H", g.GoHeaderFilePath,
		"File containing boilerplate header text. The string YEAR will be replace with the current 4-digit year.", "")
	app.BoolVarP(&g.VerifyOnly, "verify-only", "", g.VerifyOnly,
		"If true, only verify existing output, do not write anything.", "")
	app.StringVarP(&g.GeneratedBuildTag, "build-tag", "", g.GeneratedBuildTag,
		"A go build tag to use to identify files generated by this command. Should be unique.", "")
}

// LoadGoBoilerplate loads the boilerplate file passed to --go-header-file.
func (g *GeneratorArgs) LoadGoBoilerplate() ([]byte, error) {
	b, err := ioutil.ReadFile(g.GoHeaderFilePath)
	if err != nil {
		return nil, err
	}
	b = bytes.Replace(b, []byte("YEAR"), []byte(strconv.Itoa(time.Now().UTC().Year())), -1)

	if g.GeneratedByCommentTemplate != "" {
		if len(b) != 0 {
			b = append(b, byte('\n'))
			generatorName := path.Base(os.Args[0])
			generatedByComment := strings.Replace(g.GeneratedByCommentTemplate, "GENERATOR_NAME", generatorName, -1)
			s := fmt.Sprintf("%s\n\n", generatedByComment)
			b = append(b, []byte(s)...)
		}
	}
	return b, nil
}

// NewBuilder makes a new parser.Builder and populates it with the input
// directories.
func (g *GeneratorArgs) NewBuilder() (*parser.Builder, error) {
	b := parser.New()

	// flag for including *_test.go
	b.IncludeTestFiles = g.IncludeTestFile

	// Ignore all auto-generated files.
	b.AddBuildTags(g.GeneratedBuildTag)

	for _, d := range g.InputDirs {
		var err error
		if strings.HasSuffix(d, "/...") {
			err = b.AddDirRecursive(strings.TrimSuffix(d, "/..."))
		} else {
			err = b.AddDir(d)
		}
		if err != nil {
			return nil, fmt.Errorf("unable to add directory %q: %v", d, err)
		}
	}

	return b, nil
}

// InputIncludes returns true if the given package is a (sub) package of one of
// the InputDirs.
func (g *GeneratorArgs) InputIncludes(p *types.Package) bool {
	for _, dir := range g.InputDirs {
		d := dir
		if strings.HasSuffix(d, "...") {
			d = strings.TrimSuffix(d, "...")
		}
		if strings.HasPrefix(d, "./vendor/") {
			d = strings.TrimPrefix(d, "./vendor/")
		}
		if strings.HasPrefix(p.Path, d) {
			return true
		}
	}
	return false
}

// DefaultSourceTree returns the /src directory of the first entry in $GOPATH.
// If &GOPATH is empty, it returns "./". Useful as default output location.
func DefaultSourceTree() string {
	paths := strings.Split(os.Getenv("GOPATH"), string(filepath.ListSeparator))
	if len(paths) > 0 && len(paths[0]) > 0 {
		return filepath.Join(paths[0], "src")
	}
	return "./"
}

// Execute implements main().
// If you don't need any non-default behavior, use as:
// args.Default().Execute(...)
func (g *GeneratorArgs) Execute(nameSystems namer.NameSystems, defaultSystem string, pkgs func(*generator.Context, *GeneratorArgs) generator.Packages) error {
	if g.defaultCommandLineFlags {
		cmd := ccli.CommandLine
		g.AddFlags(cmd)
		cmd.RunAndExitOnError()
	}

	b, err := g.NewBuilder()
	if err != nil {
		return err
	}

	// pass though the flag on whether to include *_test.go files
	b.IncludeTestFiles = g.IncludeTestFile

	c, err := generator.NewContext(b, nameSystems, defaultSystem)
	if err != nil {
		return fmt.Errorf("failed making a context: %v", err)
	}

	c.Verify = g.VerifyOnly
	packages := pkgs(c, g)
	if err := c.ExecutePackages(g.OutputBase, packages); err != nil {
		return fmt.Errorf("failed executing generator: %v", err)
	}

	return nil
}
