package main

import (
    "fmt"
    "github.com/arran4/g2"
)

func main() {
    b := g2.NewEbuildBuilder()
    b.SetHeader("# Copyright 1999-2023 Gentoo Authors\n# Distributed under the terms of the GNU General Public License v2")
    b.SetEAPI("8")
    b.SetVariable("DESCRIPTION", "My awesome package")
    b.SetVariable("HOMEPAGE", "https://example.com")
    b.AddSrcURI("https://example.com/foo.tar.gz", "foo-1.0.tar.gz")
    b.SetFunction("src_prepare", "{\n\tdefault\n}")
    e := b.Build()
    fmt.Println(e.String())
}
