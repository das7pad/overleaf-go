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

package frontendBuild

import (
	"path/filepath"

	"github.com/evanw/esbuild/pkg/api"
)

type buildOptions struct {
	api.BuildOptions
	Description      string
	ListenForRebuild bool
}

func baseConfig(root string, desc string) buildOptions {
	return buildOptions{
		Description: desc,
		BuildOptions: api.BuildOptions{
			AbsWorkingDir:    root,
			AssetNames:       "assets/[name]-[hash]",
			Bundle:           true,
			ChunkNames:       "chunks/[name]-[hash]",
			EntryNames:       "[dir]/[name]-[hash]",
			Inject:           nil,
			MinifyWhitespace: true,
			MinifySyntax:     true,
			Metafile:         true,
			Sourcemap:        api.SourceMapLinked,
			SourceRoot:       "src:///",
			Target:           api.DefaultTarget,
			Engines:          []api.Engine{}, // TODO
			Outdir:           join(root, "public"),
			Tsconfig:         join(root, "tsconfig.json"),
			LogLevel:         api.LogLevelInfo,
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

func mainBundlesConfig(root string) buildOptions {
	c := baseConfig(root, "main bundles")
	c.ListenForRebuild = true
	c.Splitting = true
	c.Format = api.FormatESModule
	c.EntryPoints = []string{
		join(root, "frontend/js/ide.js"),
		join(root, "frontend/js/main.js"),
	}
	c.Outbase = join(root, "frontend/js/")
	c.Outdir = join(root, "public/js/")
	c.Inject = []string{join(root, "esbuild/inject/bootstrap.js")}
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

func marketingBundlesConfig(root string) buildOptions {
	c := baseConfig(root, "marketing bundles")
	c.ListenForRebuild = true
	c.Splitting = true
	c.Format = api.FormatESModule
	c.EntryPoints = []string{
		join(root, "frontend/js/marketing.js"),
	}
	pages, err := filepath.Glob(join(root, "frontend/js/pages/*/*.js"))
	if err != nil {
		panic(err)
	}
	c.EntryPoints = append(c.EntryPoints, pages...)

	pages, err = filepath.Glob(join(root, "modules/*/frontend/js/pages/*.js"))
	if err != nil {
		panic(err)
	}
	c.EntryPoints = append(c.EntryPoints, pages...)

	c.Outbase = join(root, "frontend/js/")
	c.Outdir = join(root, "public/js/")
	c.Inject = []string{join(root, "esbuild/inject/bootstrapMarketing.js")}
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

func mathJaxBundleConfig(root string) buildOptions {
	c := baseConfig(root, "MathJax bundle")
	c.EntryPoints = []string{join(root, "frontend/js/MathJaxBundle.js")}
	c.Outbase = join(root, "frontend/js/")
	c.Outdir = join(root, "public/js/")
	c.Tsconfig = join(root, "esbuild/tsconfig-no-strict.json")
	return c
}

func translationsBundlesConfig(root string) buildOptions {
	entryPoints, err := filepath.Glob(join(root, "generated/lng/*.js"))
	if err != nil {
		panic(err)
	}

	c := baseConfig(root, "translations bundles")
	c.EntryPoints = entryPoints
	c.Outbase = join(root, "generated/lng/")
	c.Outdir = join(root, "public/js/t/")
	return c
}

func stylesheetsConfig(root string) buildOptions {
	c := baseConfig(root, "stylesheet bundles")
	c.Plugins = []api.Plugin{lessLoaderPlugin(root)}
	c.EntryPoints = []string{
		join(root, "frontend/stylesheets/style.less"),
		join(root, "frontend/stylesheets/light-style.less"),
	}
	c.Outbase = join(root, "frontend/stylesheets/")
	c.Outdir = join(root, "public/stylesheets/")
	return c
}

func getConfigs(root string) []buildOptions {
	return []buildOptions{
		mainBundlesConfig(root),
		marketingBundlesConfig(root),
		translationsBundlesConfig(root),
		mathJaxBundleConfig(root),
		stylesheetsConfig(root),
	}
}
