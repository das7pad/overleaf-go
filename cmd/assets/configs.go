// Golang port of Overleaf
// Copyright (C) 2023 Jakob Ackermann <das7pad@outlook.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"path"
	"path/filepath"

	"github.com/evanw/esbuild/pkg/api"
)

type buildOptions struct {
	api.BuildOptions
	Description string
}

func baseConfig(p string, desc string) buildOptions {
	return buildOptions{
		Description: desc,
		BuildOptions: api.BuildOptions{
			AbsWorkingDir:    p,
			AssetNames:       "assets/[name]-[hash]",
			Bundle:           true,
			ChunkNames:       "chunks/[name]-[hash]",
			EntryNames:       "[dir]/[name]-[hash]",
			Inject:           nil,
			MinifyWhitespace: true,
			MinifySyntax:     true,
			Metafile:         true,
			Sourcemap:        api.SourceMapLinked,
			Target:           api.DefaultTarget,
			Engines:          []api.Engine{}, // TODO
			Outdir:           path.Join(p, "public"),
			Tsconfig:         path.Join(p, "tsconfig.json"),
			Loader: map[string]api.Loader{
				".js":    api.LoaderJSX,
				".woff":  api.LoaderFile,
				".woff2": api.LoaderFile,
				".png":   api.LoaderFile,
				".svg":   api.LoaderFile,
				".gif":   api.LoaderFile,
				".node":  api.LoaderCopy,
			},
		},
	}
}

func mainBundlesConfig(p string) buildOptions {
	c := baseConfig(p, "main bundles")
	c.Splitting = true
	c.Format = api.FormatESModule
	c.EntryPoints = []string{
		path.Join(p, "frontend/js/ide.js"),
		path.Join(p, "frontend/js/main.js"),
	}
	c.Outbase = path.Join(p, "frontend/js/")
	c.Outdir = path.Join(p, "public/js/")
	c.Inject = []string{path.Join(p, "esbuild/inject/bootstrap.js")}
	c.Define = map[string]string{
		"process.env.NODE_ENV": `"production"`,
		// work around 'process' usage in algoliasearch
		"process.env.RESET_APP_DATA_TIMER": "null",
		// silence ad
		"__REACT_DEVTOOLS_GLOBAL_HOOK__": `{ "isDisabled": true }`,
	}
	c.JSX = api.JSXAutomatic
	c.Alias = map[string]string{
		"ace": "ace-builds/src-noconflict",
	}
	return c
}

func marketingBundlesConfig(p string) buildOptions {
	c := baseConfig(p, "marketing bundles")
	c.Splitting = true
	c.Format = api.FormatESModule
	c.EntryPoints = []string{
		path.Join(p, "frontend/js/marketing.js"),
	}
	pages, err := filepath.Glob(path.Join(p, "frontend/js/pages/*/*.js"))
	if err != nil {
		panic(err)
	}
	c.EntryPoints = append(c.EntryPoints, pages...)

	pages, err = filepath.Glob(path.Join(p, "modules/*/frontend/js/pages/*.js"))
	if err != nil {
		panic(err)
	}
	c.EntryPoints = append(c.EntryPoints, pages...)

	c.Outbase = path.Join(p, "frontend/js/")
	c.Outdir = path.Join(p, "public/js/")
	c.Inject = []string{path.Join(p, "esbuild/inject/bootstrapMarketing.js")}
	c.Define = map[string]string{
		"process.env.NODE_ENV": `"production"`,
		// work around 'process' usage in algoliasearch
		"process.env.RESET_APP_DATA_TIMER": "null",
		// silence ad
		"__REACT_DEVTOOLS_GLOBAL_HOOK__": `{ "isDisabled": true }`,
	}
	c.JSX = api.JSXAutomatic
	return c
}

func mathJaxBundleConfig(p string) buildOptions {
	c := baseConfig(p, "MathJax bundle")
	c.EntryPoints = []string{path.Join(p, "frontend/js/MathJaxBundle.js")}
	c.Outbase = path.Join(p, "frontend/js/")
	c.Outdir = path.Join(p, "public/js/")
	c.Tsconfig = path.Join(p, "esbuild/tsconfig-no-strict.json")
	return c
}

func translationsBundlesConfig(p string) buildOptions {
	entryPoints, err := filepath.Glob(path.Join(p, "generated/lng/*.js"))
	if err != nil {
		panic(err)
	}

	c := baseConfig(p, "translations bundles")
	c.EntryPoints = entryPoints
	c.Outbase = path.Join(p, "generated/lng/")
	c.Outdir = path.Join(p, "public/js/t/")
	return c
}

func stylesheetsConfig(p string) buildOptions {
	c := baseConfig(p, "stylesheet bundles")
	c.Plugins = []api.Plugin{lessLoaderPlugin(p)}
	c.EntryPoints = []string{
		path.Join(p, "frontend/stylesheets/style.less"),
		path.Join(p, "frontend/stylesheets/light-style.less"),
	}
	c.Outbase = path.Join(p, "frontend/stylesheets/")
	c.Outdir = path.Join(p, "public/stylesheets/")
	return c
}

func getConfigs(p string) []buildOptions {
	return []buildOptions{
		mainBundlesConfig(p),
		marketingBundlesConfig(p),
		translationsBundlesConfig(p),
		mathJaxBundleConfig(p),
		stylesheetsConfig(p),
	}
}
