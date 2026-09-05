#!/bin/sh
# Builds the deterministic git repositories the test-suite measures against.
#
# Every commit uses an explicit "<unix-timestamp> <offset>" date, which git
# accepts verbatim, so no `date` invocation is needed and the script produces
# byte-identical repositories on Linux, macOS and Windows/Git-Bash. Running it
# twice yields identical commit hashes.

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
root="$script_dir/fixtures"

rm -rf "$root"
mkdir -p "$root"

# 2025-10-01T00:00:00Z. Fifty commits at a three-day stride reach 2026-02-25,
# so the fixture spans two calendar years and exercises year-over-year output.
BASE_TS=1759276800
DAY=86400

# --------------------------------------------------------------------------
# helpers
# --------------------------------------------------------------------------

init_repo() {
	dir="$1"
	mkdir -p "$dir"
	git -C "$dir" init -q
	git -C "$dir" symbolic-ref HEAD refs/heads/main
	git -C "$dir" config user.name "Fixture Builder"
	git -C "$dir" config user.email "fixtures@example.com"
	git -C "$dir" config commit.gpgsign false
	git -C "$dir" config core.autocrlf false
	git -C "$dir" config core.safecrlf false
}

# commit <dir> <timestamp> <tz> <author-name> <author-email> <subject>
commit() {
	dir="$1"; ts="$2"; tz="$3"; name="$4"; email="$5"; subject="$6"
	GIT_AUTHOR_NAME="$name" \
	GIT_AUTHOR_EMAIL="$email" \
	GIT_AUTHOR_DATE="$ts $tz" \
	GIT_COMMITTER_NAME="$name" \
	GIT_COMMITTER_EMAIL="$email" \
	GIT_COMMITTER_DATE="$ts $tz" \
	git -C "$dir" commit -q --no-verify -m "$subject"
}

# The i-th author. Ada appears under two addresses so identity resolution has
# something real to merge.
author_name() {
	case $(($1 % 4)) in
		0) echo "Ada Lovelace" ;;
		1) echo "Grace Hopper" ;;
		2) echo "Ada L." ;;
		*) echo "Alan Turing" ;;
	esac
}

author_email() {
	case $(($1 % 4)) in
		0) echo "ada@example.com" ;;
		1) echo "grace@example.com" ;;
		2) echo "ada.lovelace@corp.example.com" ;;
		*) echo "alan@example.com" ;;
	esac
}

commit_tz() {
	case $(($1 % 3)) in
		0) echo "+0000" ;;
		1) echo "+0300" ;;
		*) echo "-0500" ;;
	esac
}

# Hours cycle through business hours, late nights and early mornings.
commit_hour() {
	case $(($1 % 8)) in
		0) echo 9 ;;
		1) echo 14 ;;
		2) echo 23 ;;
		3) echo 2 ;;
		4) echo 17 ;;
		5) echo 11 ;;
		6) echo 20 ;;
		*) echo 4 ;;
	esac
}

commit_subject() {
	case $(($1 % 10)) in
		0) echo "feat(core): add handler number $1" ;;
		1) echo "fix: correct off-by-one in parser $1" ;;
		2) echo "docs: update README section $1" ;;
		3) echo "wip" ;;
		4) echo "refactor: extract helper $1" ;;
		5) echo "chore: bump dependency $1" ;;
		6) echo "test: add coverage for case $1" ;;
		7) echo "Fix typo in comment $1" ;;
		8) echo "Merge branch 'topic-$1' into main" ;;
		*) echo "ci: update workflow $1" ;;
	esac
}

# Writes the 50-commit history shared by the basic and mailmap fixtures.
build_history() {
	dir="$1"
	i=0
	while [ "$i" -lt 50 ]; do
		hour=$(commit_hour "$i")
		ts=$((BASE_TS + i * 3 * DAY + hour * 3600))
		mkdir -p "$dir/src" "$dir/docs"
		printf 'line %s\n' "$i" >>"$dir/src/main.go"
		if [ $((i % 3)) -eq 0 ]; then
			printf 'entry %s\n' "$i" >>"$dir/docs/notes.md"
		fi
		if [ $((i % 5)) -eq 0 ]; then
			printf 'helper %s\n' "$i" >>"$dir/src/util.go"
		fi
		git -C "$dir" add -A
		commit "$dir" "$ts" "$(commit_tz "$i")" "$(author_name "$i")" \
			"$(author_email "$i")" "$(commit_subject "$i")"
		i=$((i + 1))
	done
}

# --------------------------------------------------------------------------
# basic/ - 50 commits, 3 people (one with two addresses), 3 timezone offsets
# --------------------------------------------------------------------------
echo "building basic/"
init_repo "$root/basic"
build_history "$root/basic"

# --------------------------------------------------------------------------
# mailmap/ - the same history plus a .mailmap unifying Ada's two addresses
# --------------------------------------------------------------------------
echo "building mailmap/"
init_repo "$root/mailmap"
cat >"$root/mailmap/.mailmap" <<'EOF'
Ada Lovelace <ada@example.com> Ada L. <ada.lovelace@corp.example.com>
EOF
git -C "$root/mailmap" add -A
commit "$root/mailmap" "$((BASE_TS - DAY))" "+0000" "Ada Lovelace" \
	"ada@example.com" "chore: add mailmap"
build_history "$root/mailmap"

# --------------------------------------------------------------------------
# merges/ - 5 merge commits and 5 squash-style single-parent commits
# --------------------------------------------------------------------------
echo "building merges/"
init_repo "$root/merges"
printf 'root\n' >"$root/merges/base.txt"
git -C "$root/merges" add -A
commit "$root/merges" "$BASE_TS" "+0000" "Ada Lovelace" "ada@example.com" \
	"feat: initial commit"

n=1
while [ "$n" -le 5 ]; do
	ts=$((BASE_TS + n * 2 * DAY))
	git -C "$root/merges" checkout -q -b "topic-$n" main
	printf 'topic %s\n' "$n" >"$root/merges/topic-$n.txt"
	git -C "$root/merges" add -A
	commit "$root/merges" "$ts" "+0000" "Grace Hopper" "grace@example.com" \
		"feat: work on topic $n"
	git -C "$root/merges" checkout -q main
	GIT_AUTHOR_NAME="Ada Lovelace" \
	GIT_AUTHOR_EMAIL="ada@example.com" \
	GIT_AUTHOR_DATE="$((ts + 3600)) +0000" \
	GIT_COMMITTER_NAME="Ada Lovelace" \
	GIT_COMMITTER_EMAIL="ada@example.com" \
	GIT_COMMITTER_DATE="$((ts + 3600)) +0000" \
	git -C "$root/merges" merge -q --no-ff --no-verify \
		-m "Merge branch 'topic-$n' into main" "topic-$n"
	n=$((n + 1))
done

n=1
while [ "$n" -le 5 ]; do
	ts=$((BASE_TS + (10 + n) * 2 * DAY))
	printf 'squash %s\n' "$n" >"$root/merges/squash-$n.txt"
	git -C "$root/merges" add -A
	commit "$root/merges" "$ts" "+0000" "Alan Turing" "alan@example.com" \
		"feat: squashed change $n (#$n)"
	n=$((n + 1))
done

# --------------------------------------------------------------------------
# binary/ - binary content and awkward path characters
# --------------------------------------------------------------------------
echo "building binary/"
init_repo "$root/binary"
printf 'text\n' >"$root/binary/readme.txt"
# A small PNG header followed by NUL bytes: unambiguously binary to git.
printf '\211PNG\r\n\032\n\000\000\000\015IHDR\000\000\000\001' >"$root/binary/logo.png"
mkdir -p "$root/binary/assets"
printf 'spaced\n' >"$root/binary/assets/a file with spaces.txt"
printf 'unicode\n' >"$root/binary/assets/ünïcödé-ファイル.txt"
git -C "$root/binary" add -A
commit "$root/binary" "$BASE_TS" "+0000" "Ada Lovelace" "ada@example.com" \
	"feat: add assets"

# A quotation mark is legal in a path on POSIX filesystems but not on NTFS, so
# this file is best-effort. Tests that need it skip when it is absent.
if (cd "$root/binary" && : >'quoted".txt') 2>/dev/null; then
	printf 'quoted\n' >"$root/binary/quoted\".txt"
	git -C "$root/binary" add -A
	commit "$root/binary" "$((BASE_TS + DAY))" "+0000" "Ada Lovelace" \
		"ada@example.com" "feat: add file with a quote in its name"
fi

printf '\000\001\002\003binary update\000' >>"$root/binary/logo.png"
git -C "$root/binary" add -A
commit "$root/binary" "$((BASE_TS + 2 * DAY))" "+0000" "Ada Lovelace" \
	"ada@example.com" "chore: update logo"

# A subject that would close the script element the report is embedded in, if
# the renderer failed to escape it.
printf 'escaping\n' >>"$root/binary/readme.txt"
git -C "$root/binary" add -A
commit "$root/binary" "$((BASE_TS + 3 * DAY))" "+0000" "Ada Lovelace" \
	"ada@example.com" 'fix: strip </script> tags from user input'

# --------------------------------------------------------------------------
# noise/ - generated, vendored and bundled content that must not dominate
# --------------------------------------------------------------------------
echo "building noise/"
init_repo "$root/noise"
printf 'package main\n' >"$root/noise/main.go"
git -C "$root/noise" add -A
commit "$root/noise" "$BASE_TS" "+0000" "Ada Lovelace" "ada@example.com" \
	"feat: initial commit"

# One commit carrying both a 40,000-line lockfile, which default exclusions
# must keep out of every line total, and a 12,000-line hand-committed data
# table, which must still push the commit over the bulk threshold.
awk 'BEGIN { print "{"; print "  \"dependencies\": {";
	for (i = 1; i <= 39996; i++) printf "    \"pkg-%d\": \"1.0.%d\",\n", i, i;
	print "    \"pkg-last\": \"1.0.0\""; print "  }"; print "}" }' \
	>"$root/noise/package-lock.json"
awk 'BEGIN { print "package data"; print "var Table = []string{";
	for (i = 1; i <= 11997; i++) printf "\t\"row-%d\",\n", i;
	print "}" }' >"$root/noise/data.go"
git -C "$root/noise" add -A
commit "$root/noise" "$((BASE_TS + DAY))" "+0000" "Ada Lovelace" \
	"ada@example.com" "chore: regenerate lockfile and data tables"

mkdir -p "$root/noise/vendor/github.com/example/lib" "$root/noise/dist"
i=1
while [ "$i" -le 200 ]; do
	printf 'vendored line %s\n' "$i" >>"$root/noise/vendor/github.com/example/lib/lib.go"
	i=$((i + 1))
done
i=1
while [ "$i" -le 300 ]; do
	printf 'bundled %s;' "$i" >>"$root/noise/dist/bundle.min.js"
	i=$((i + 1))
done
printf '\n' >>"$root/noise/dist/bundle.min.js"
git -C "$root/noise" add -A
commit "$root/noise" "$((BASE_TS + 2 * DAY))" "+0000" "Ada Lovelace" \
	"ada@example.com" "chore: vendor dependencies and rebuild bundle"

printf 'func main() {}\n' >>"$root/noise/main.go"
git -C "$root/noise" add -A
commit "$root/noise" "$((BASE_TS + 3 * DAY))" "+0000" "Grace Hopper" \
	"grace@example.com" "feat: add entrypoint"

# linguist-generated marks machine-written code that is not covered by any
# default path pattern.
mkdir -p "$root/noise/api"
printf 'package api\n\n// generated client\n' >"$root/noise/api/client.go"
printf 'package api\n\n// generated types\n' >"$root/noise/api/types.go"
printf 'api/*.go linguist-generated=true\n' >"$root/noise/.gitattributes"
git -C "$root/noise" add -A
commit "$root/noise" "$((BASE_TS + 4 * DAY))" "+0000" "Grace Hopper" \
	"grace@example.com" "chore: regenerate api client"

# --------------------------------------------------------------------------
# bots/ - automated authors that default configuration must exclude
# --------------------------------------------------------------------------
echo "building bots/"
init_repo "$root/bots"
printf 'human\n' >"$root/bots/app.txt"
git -C "$root/bots" add -A
commit "$root/bots" "$BASE_TS" "+0000" "Ada Lovelace" "ada@example.com" \
	"feat: initial commit"

printf 'dep bump\n' >>"$root/bots/app.txt"
git -C "$root/bots" add -A
commit "$root/bots" "$((BASE_TS + DAY))" "+0000" "dependabot[bot]" \
	"49699333+dependabot[bot]@users.noreply.github.com" \
	"chore(deps): bump lodash from 4.17.20 to 4.17.21"

printf 'renovation\n' >>"$root/bots/app.txt"
git -C "$root/bots" add -A
commit "$root/bots" "$((BASE_TS + 2 * DAY))" "+0000" "renovate[bot]" \
	"29139614+renovate[bot]@users.noreply.github.com" \
	"chore(deps): update dependency typescript to v5"

printf 'more human work\n' >>"$root/bots/app.txt"
git -C "$root/bots" add -A
commit "$root/bots" "$((BASE_TS + 3 * DAY))" "+0000" "Grace Hopper" \
	"grace@example.com" "feat: add feature"

# --------------------------------------------------------------------------
# coupling/ - two files that always change together, and one that rarely does
# --------------------------------------------------------------------------
echo "building coupling/"
init_repo "$root/coupling"
i=0
while [ "$i" -lt 12 ]; do
	ts=$((BASE_TS + i * DAY))
	printf 'a %s\n' "$i" >>"$root/coupling/alpha.go"
	printf 'b %s\n' "$i" >>"$root/coupling/beta.go"
	if [ "$i" -lt 4 ]; then
		printf 'c %s\n' "$i" >>"$root/coupling/gamma.go"
	fi
	git -C "$root/coupling" add -A
	commit "$root/coupling" "$ts" "+0000" "Ada Lovelace" "ada@example.com" \
		"feat: change $i"
	i=$((i + 1))
done

# --------------------------------------------------------------------------
# shallow/, empty/, single/
# --------------------------------------------------------------------------
echo "building shallow/"
git clone -q --depth 5 "file://$root/basic" "$root/shallow"

echo "building empty/"
init_repo "$root/empty"

echo "building single/"
init_repo "$root/single"
printf 'only\n' >"$root/single/only.txt"
git -C "$root/single" add -A
commit "$root/single" "$BASE_TS" "+0000" "Ada Lovelace" "ada@example.com" \
	"feat: the one and only commit"

echo "fixtures built under $root"
