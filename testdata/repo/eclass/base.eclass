# Copyright 2024 Gentoo Authors

# @ECLASS: base.eclass
# @MAINTAINER:
# test@example.com
# @SUPPORTED_EAPIS: 7 8
# @DESCRIPTION:
# A test base eclass for verification.

EAPI=8

DESCRIPTION="A basic eclass"

base_src_configure() {
	echo "Configuring..."
}

base_src_compile() {
	echo "Compiling..."
}

EXPORT_FUNCTIONS src_configure src_compile
