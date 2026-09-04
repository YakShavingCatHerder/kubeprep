#!/bin/sh
# Copy a canonical scenario and catalog.yaml into the embedded pack.
# usage: sh scripts/bundle-lesson.sh 0x-subject-name/0x-lab-name

set -eu

if [ "$#" -ne 1 ] || [ -z "$1" ]; then
	echo "usage: make bundle-lesson 0x-subject-name/0x-lab-name" >&2
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
	echo "usage: make bundle-lesson 0x-subject-name/0x-lab-name" >&2
	exit 1
	;;
esac

if [ ! -f curriculum/catalog.yaml ]; then
	echo "bundle-lesson: curriculum/catalog.yaml is missing" >&2
	exit 1
fi

escaped=$(printf '%s' "$rel" | sed 's/[.[\*^$()+?{|\\]/\\&/g')
if ! grep -Eq "^[[:space:]]+path:[[:space:]]+${escaped}[[:space:]]*$" curriculum/catalog.yaml; then
	echo "bundle-lesson: $rel is not listed in curriculum/catalog.yaml" >&2
	echo "add a scenarios path entry before bundling, for example:" >&2
	echo "  - id: $(basename "$rel" .yaml | sed 's/\.yml$//; s/^[0-9][0-9]-//')" >&2
	echo "    path: $rel" >&2
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
