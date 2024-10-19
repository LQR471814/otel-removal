package main

import (
	"flag"
	"fmt"
	"log"

	"golang.org/x/tools/go/packages"
)

func main() {
	verbose := flag.Bool("v", false, "Enable debug logging.")
	flag.Parse()
	matchers := flag.Args()

	fmt.Println("loading all your packages...")

	conf := &packages.Config{
		Mode: packages.NeedTypes | packages.NeedSyntax | packages.NeedFiles | packages.NeedName,
	}
	pkgs, err := packages.Load(conf, matchers...)
	if err != nil {
		log.Fatalf("load packages: %v\n", err)
	}

	for _, pkg := range pkgs {
		fmt.Println("transforming", pkg.PkgPath)

		transformer := NewTransformer(
			pkg.TypesInfo,
			pkg.Fset,
			pkg.Syntax,
			pkg.GoFiles,
		)
		transformer.Debug = *verbose
		err = transformer.Run()
		if err != nil {
			log.Fatalf("transform (%s): %v\n", pkg.Name, err)
		}
	}
}
