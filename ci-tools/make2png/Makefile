# This makefile scans a given folder and generates a png
# with the dependency graph of the Makefile in that folder
#
# It relies on dot and pipenv to run
.PHONY: run check-deps
.SILENT: run check-deps

MAKE2PNG_DIR=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

run: | check-deps
	[[ -n "$(folder)" ]] || { echo "Missing folder=<folder to process> argument. Please check and try again" 1>&2; exit 1; }
	bash $(MAKE2PNG_DIR)/generate.sh $(folder)

check-deps:
	which make2graph > /dev/null || { echo "Missing make2graph, please install and try again" 1>&2; exit 1; }
	which dot > /dev/null || { echo "Missing dot (graphviz), please install and try again" 1>&2; exit 1; }