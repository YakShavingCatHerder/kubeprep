#!/bin/sh
# Copy a canonical scenario and catalog.yaml into the embedded pack.
# usage: sh scripts/bundle-lesson.sh pods/pod-creation

set -eu

if [ "$#" -ne 1 ] || [ -z "$1" ]; then
	echo "usage: make bundle-lesson section/lab-id" >&2
	exit 1
fi

rel=$1
rel=${rel#./}
rel=${rel#curriculum/}

case $rel in
*..* | /*)
	echo "bundle-lesson: path must be relative to curriculum/" >&2
	exit 1
	;;
esac

src=curriculum/$rel
if [ ! -f "$src" ]; then
	case $rel in
	*.yaml | *.yml) ;;
	*)
		if [ -f "curriculum/${rel}.yaml" ]; then
			rel=${rel}.yaml
			src=curriculum/$rel
		elif [ -f "curriculum/${rel}.yml" ]; then
			rel=${rel}.yml
			src=curriculum/$rel
		fi
		;;
	esac
fi

if [ ! -f "$src" ]; then
	echo "bundle-lesson: not found: curriculum/$1" >&2
	exit 1
fi

case $rel in
catalog.yaml | catalog.yml)
	echo "bundle-lesson: catalog.yaml is copied automatically when bundling a scenario" >&2
	echo "usage: make bundle-lesson section/lab-id" >&2
	exit 1
	;;
esac

if [ ! -f curriculum/catalog.yaml ]; then
	echo "bundle-lesson: curriculum/catalog.yaml is missing" >&2
	exit 1
fi

lab_id=$(basename "$rel" .yaml)
lab_id=${lab_id%.yml}
escaped=$(printf '%s' "$lab_id" | sed 's/[.[\*^$()+?{|\\]/\\&/g')
if ! grep -Eq "^[[:space:]]+-[[:space:]]+${escaped}[[:space:]]*$" curriculum/catalog.yaml; then
	echo "bundle-lesson: $lab_id is not listed in curriculum/catalog.yaml" >&2
	echo "add it under paths[].sections[].labs, for example:" >&2
	echo "  - $lab_id" >&2
	exit 1
fi

copy_into_bundled() {
	from=$1
	into=$2
	mkdir -p "$(dirname "$into")"
	cp "$from" "$into"
	echo "copied $from -> $into"
}

copy_into_bundled "$src" "internal/curriculum/bundled/$rel"
copy_into_bundled curriculum/catalog.yaml internal/curriculum/bundled/catalog.yaml
