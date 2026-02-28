#!/usr/bin/env bash
# Compares release artifacts produced by GoReleaser/Make against those produced
# by Bazel.  Exits 0 when all checks pass, 1 on any failure.
#
# Usage:
#   scripts/verify-bazel-artifacts.sh <goreleaser-dir> <bazel-dir> [--version VERSION]
#
# Both directories should contain the same kinds of artifacts:
#   *.tar.gz / *.zip   — platform archives
#   *.tgz              — Node.js npm tarball
#   *.whl              — Python wheel

set -euo pipefail

# ── Colours ──────────────────────────────────────────────────────────────────

if [[ -t 1 ]]; then
    RED=$'\033[0;31m'
    GREEN=$'\033[0;32m'
    YELLOW=$'\033[0;33m'
    BOLD=$'\033[1m'
    RESET=$'\033[0m'
else
    RED="" GREEN="" YELLOW="" BOLD="" RESET=""
fi

PASS=0 FAIL=0 WARN=0

pass_check()  { ((PASS++)); echo "  ${GREEN}PASS${RESET}  $1"; }
fail_check()  { ((FAIL++)); echo "  ${RED}FAIL${RESET}  $1"; }
warn_check()  { ((WARN++)); echo "  ${YELLOW}WARN${RESET}  $1"; }

# ── Helpers ──────────────────────────────────────────────────────────────────

extract_archive() {
    local archive="$1" dest="$2"
    mkdir -p "$dest"
    case "$archive" in
        *.tar.gz) tar xzf "$archive" -C "$dest" ;;
        *.tgz)    tar xzf "$archive" -C "$dest" ;;
        *.zip)    unzip -qo "$archive" -d "$dest" ;;
        *.whl)    unzip -qo "$archive" -d "$dest" ;;
        *)        echo "Unknown archive format: $archive" >&2; return 1 ;;
    esac
}

file_listing() {
    (cd "$1" && find . -type f | sed 's|^\./||' | sort)
}

portable_filesize() {
    # Works on both macOS (BSD stat) and Linux (GNU stat).
    if stat -f%z "$1" &>/dev/null; then
        stat -f%z "$1"
    else
        stat -c%s "$1"
    fi
}

check_size_tolerance() {
    local file_a="$1" file_b="$2" pct="$3"
    local size_a size_b
    size_a=$(portable_filesize "$file_a")
    size_b=$(portable_filesize "$file_b")
    local max=$(( size_a > size_b ? size_a : size_b ))
    if [[ $max -eq 0 ]]; then return 0; fi
    local diff=$(( size_a - size_b ))
    diff=${diff#-}  # abs
    (( diff * 100 < pct * max ))
}

# ── Argument parsing ─────────────────────────────────────────────────────────

if [[ $# -lt 2 ]]; then
    echo "Usage: $0 <goreleaser-dir> <bazel-dir> [--version VERSION]" >&2
    exit 2
fi

GR_DIR="$1"; shift
BZ_DIR="$1"; shift
VERSION=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --version) VERSION="$2"; shift 2 ;;
        *)         echo "Unknown option: $1" >&2; exit 2 ;;
    esac
done

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

# ── Go binary names (without extension) ─────────────────────────────────────

GO_BINARIES=(pulumi pulumi-language-go pulumi-language-nodejs pulumi-language-python)

SCRIPTS=(pulumi-resource-pulumi-nodejs pulumi-resource-pulumi-python pulumi-language-python-exec)

EXTERNAL_BINARIES=(pulumi-language-dotnet pulumi-language-java pulumi-language-yaml pulumi-watch)

is_go_binary() {
    local name="${1%.exe}"
    for b in "${GO_BINARIES[@]}"; do [[ "$name" == "$b" ]] && return 0; done
    return 1
}

is_script() {
    local name="${1%.cmd}"
    for s in "${SCRIPTS[@]}"; do [[ "$name" == "$s" ]] && return 0; done
    return 1
}

is_external_binary() {
    local name="${1%.exe}"
    for e in "${EXTERNAL_BINARIES[@]}"; do [[ "$name" == "$e" ]] && return 0; done
    return 1
}

# ── 1. Platform archives ────────────────────────────────────────────────────

compare_platform_archives() {
    echo ""
    echo "${BOLD}=== Platform Archives ===${RESET}"

    # Discover archives in the GoReleaser directory.
    local found_any=false
    for gr_archive in "$GR_DIR"/pulumi-*.tar.gz "$GR_DIR"/pulumi-*.zip; do
        [[ -f "$gr_archive" ]] || continue
        found_any=true

        local base
        base=$(basename "$gr_archive")

        # Find corresponding Bazel archive.
        local bz_archive="$BZ_DIR/$base"
        if [[ ! -f "$bz_archive" ]]; then
            fail_check "Archive $base: missing from Bazel output"
            continue
        fi

        echo ""
        echo "  ${BOLD}--- $base ---${RESET}"

        # Extract both.
        local gr_ext="$TMPDIR/gr_${base}" bz_ext="$TMPDIR/bz_${base}"
        extract_archive "$gr_archive" "$gr_ext"
        extract_archive "$bz_archive" "$bz_ext"

        # File listings.
        local gr_listing="$TMPDIR/gr_listing_${base}.txt"
        local bz_listing="$TMPDIR/bz_listing_${base}.txt"
        file_listing "$gr_ext" > "$gr_listing"
        file_listing "$bz_ext" > "$bz_listing"

        local listing_diff
        listing_diff=$(diff "$gr_listing" "$bz_listing" || true)
        if [[ -z "$listing_diff" ]]; then
            pass_check "File listing matches"
        else
            fail_check "File listing differs:"
            echo "$listing_diff" | head -20 | sed 's/^/         /'
        fi

        # Determine the prefix (pulumi/ or pulumi/bin/).
        local prefix="pulumi"
        if [[ "$base" == *windows* ]]; then
            prefix="pulumi/bin"
        fi

        # Compare individual files.
        while IFS= read -r relpath; do
            local gr_file="$gr_ext/$relpath"
            local bz_file="$bz_ext/$relpath"
            local fname
            fname=$(basename "$relpath")

            if [[ ! -f "$bz_file" ]]; then
                # Already reported above.
                continue
            fi

            if is_go_binary "$fname"; then
                # Structural comparison for Go binaries.
                if check_size_tolerance "$gr_file" "$bz_file" 20; then
                    pass_check "$fname: size within 20% tolerance"
                else
                    local gs bs
                    gs=$(portable_filesize "$gr_file")
                    bs=$(portable_filesize "$bz_file")
                    fail_check "$fname: size mismatch (goreleaser=${gs}, bazel=${bs})"
                fi

                if [[ -n "$VERSION" ]]; then
                    if strings "$bz_file" 2>/dev/null | grep -qF "$VERSION"; then
                        pass_check "$fname: version string '$VERSION' present in Bazel binary"
                    else
                        fail_check "$fname: version string '$VERSION' NOT found in Bazel binary"
                    fi
                fi
            elif is_script "$fname"; then
                if cmp -s "$gr_file" "$bz_file"; then
                    pass_check "$fname: byte-identical"
                else
                    fail_check "$fname: content differs"
                fi
            elif is_external_binary "$fname"; then
                if cmp -s "$gr_file" "$bz_file"; then
                    pass_check "$fname: byte-identical"
                else
                    fail_check "$fname: content differs (external binary should be identical)"
                fi
            else
                warn_check "$fname: unknown file, skipping comparison"
            fi
        done < "$gr_listing"
    done

    if [[ "$found_any" == false ]]; then
        warn_check "No platform archives found in $GR_DIR"
    fi
}

# ── 2. npm tarball ───────────────────────────────────────────────────────────

compare_npm_tarball() {
    echo ""
    echo "${BOLD}=== Node.js npm Tarball ===${RESET}"

    local gr_tgz bz_tgz
    gr_tgz=$(find "$GR_DIR" -maxdepth 1 -name '*.tgz' | head -1)
    bz_tgz=$(find "$BZ_DIR" -maxdepth 1 -name '*.tgz' | head -1)

    if [[ -z "$gr_tgz" ]]; then
        warn_check "No npm tarball (*.tgz) found in GoReleaser dir"
        return
    fi
    if [[ -z "$bz_tgz" ]]; then
        fail_check "No npm tarball (*.tgz) found in Bazel dir"
        return
    fi

    echo "  GoReleaser: $(basename "$gr_tgz")"
    echo "  Bazel:      $(basename "$bz_tgz")"

    local gr_ext="$TMPDIR/gr_npm" bz_ext="$TMPDIR/bz_npm"
    extract_archive "$gr_tgz" "$gr_ext"
    extract_archive "$bz_tgz" "$bz_ext"

    # File listings.
    local gr_listing="$TMPDIR/gr_npm_listing.txt"
    local bz_listing="$TMPDIR/bz_npm_listing.txt"
    file_listing "$gr_ext" > "$gr_listing"
    file_listing "$bz_ext" > "$bz_listing"

    local listing_diff
    listing_diff=$(diff "$gr_listing" "$bz_listing" || true)
    if [[ -z "$listing_diff" ]]; then
        pass_check "File listing matches ($(wc -l < "$gr_listing" | tr -d ' ') files)"
    else
        local only_gr only_bz
        only_gr=$(echo "$listing_diff" | grep '^< ' | wc -l | tr -d ' ')
        only_bz=$(echo "$listing_diff" | grep '^> ' | wc -l | tr -d ' ')
        fail_check "File listing differs (${only_gr} only in GoReleaser, ${only_bz} only in Bazel)"
        echo "$listing_diff" | head -30 | sed 's/^/         /'
    fi

    # Compare files present in both.
    local common_files
    common_files=$(comm -12 "$gr_listing" "$bz_listing")

    local identical=0 different=0 warned=0
    while IFS= read -r relpath; do
        [[ -z "$relpath" ]] && continue
        local gr_file="$gr_ext/$relpath"
        local bz_file="$bz_ext/$relpath"

        # Skip source maps — paths will differ.
        if [[ "$relpath" == *.js.map ]]; then
            ((warned++))
            continue
        fi

        if cmp -s "$gr_file" "$bz_file"; then
            ((identical++))
        else
            ((different++))
            if [[ $different -le 10 ]]; then
                fail_check "npm: $relpath differs"
            fi
        fi
    done <<< "$common_files"

    if [[ $different -eq 0 ]]; then
        pass_check "All $identical common files are identical (${warned} .js.map files skipped)"
    else
        fail_check "$different files differ out of $((identical + different)) compared"
    fi

    # Version check.
    if [[ -n "$VERSION" ]]; then
        local bz_pkgjson="$bz_ext/package/package.json"
        if [[ -f "$bz_pkgjson" ]]; then
            if grep -q "\"version\": *\"$VERSION\"" "$bz_pkgjson"; then
                pass_check "package.json version is $VERSION"
            else
                local actual
                actual=$(grep '"version"' "$bz_pkgjson" | head -1)
                fail_check "package.json version mismatch (expected $VERSION, got: $actual)"
            fi
        fi
    fi
}

# ── 3. Python wheel ─────────────────────────────────────────────────────────

compare_python_wheel() {
    echo ""
    echo "${BOLD}=== Python Wheel ===${RESET}"

    local gr_whl bz_whl
    gr_whl=$(find "$GR_DIR" -maxdepth 1 -name '*.whl' | head -1)
    bz_whl=$(find "$BZ_DIR" -maxdepth 1 -name '*.whl' | head -1)

    if [[ -z "$gr_whl" ]]; then
        warn_check "No Python wheel (*.whl) found in GoReleaser dir"
        return
    fi
    if [[ -z "$bz_whl" ]]; then
        fail_check "No Python wheel (*.whl) found in Bazel dir"
        return
    fi

    echo "  GoReleaser: $(basename "$gr_whl")"
    echo "  Bazel:      $(basename "$bz_whl")"

    local gr_ext="$TMPDIR/gr_whl" bz_ext="$TMPDIR/bz_whl"
    extract_archive "$gr_whl" "$gr_ext"
    extract_archive "$bz_whl" "$bz_ext"

    # Normalize file listings by stripping the version from .dist-info dir names.
    local gr_listing="$TMPDIR/gr_whl_listing.txt"
    local bz_listing="$TMPDIR/bz_whl_listing.txt"
    file_listing "$gr_ext" | sed 's/pulumi-[^/]*\.dist-info/pulumi.dist-info/g' > "$gr_listing"
    file_listing "$bz_ext" | sed 's/pulumi-[^/]*\.dist-info/pulumi.dist-info/g' > "$bz_listing"

    local listing_diff
    listing_diff=$(diff "$gr_listing" "$bz_listing" || true)
    if [[ -z "$listing_diff" ]]; then
        pass_check "File listing matches ($(wc -l < "$gr_listing" | tr -d ' ') files)"
    else
        local only_gr only_bz
        only_gr=$(echo "$listing_diff" | grep '^< ' | wc -l | tr -d ' ')
        only_bz=$(echo "$listing_diff" | grep '^> ' | wc -l | tr -d ' ')
        fail_check "File listing differs (${only_gr} only in GoReleaser, ${only_bz} only in Bazel)"
        echo "$listing_diff" | head -30 | sed 's/^/         /'
    fi

    # Compare .py and .pyi files (should be byte-identical from same source tree).
    local identical=0 different=0 skipped=0
    while IFS= read -r relpath; do
        [[ -z "$relpath" ]] && continue
        local gr_file bz_file
        # Reverse the normalization to find actual files.
        gr_file=$(find "$gr_ext" -path "*/$relpath" -o -path "*/${relpath/pulumi.dist-info/pulumi-*.dist-info}" 2>/dev/null | head -1)
        bz_file=$(find "$bz_ext" -path "*/$relpath" -o -path "*/${relpath/pulumi.dist-info/pulumi-*.dist-info}" 2>/dev/null | head -1)

        # Fall back to direct path.
        [[ -z "$gr_file" || ! -f "$gr_file" ]] && gr_file="$gr_ext/$relpath"
        [[ -z "$bz_file" || ! -f "$bz_file" ]] && bz_file="$bz_ext/$relpath"

        [[ ! -f "$gr_file" || ! -f "$bz_file" ]] && continue

        case "$relpath" in
            *.py|*.pyi)
                if cmp -s "$gr_file" "$bz_file"; then
                    ((identical++))
                else
                    ((different++))
                    if [[ $different -le 5 ]]; then
                        fail_check "wheel: $relpath differs"
                    fi
                fi
                ;;
            *RECORD*)
                ((skipped++))
                ;;
            *METADATA*)
                compare_wheel_metadata "$gr_file" "$bz_file"
                ;;
            *WHEEL*)
                compare_wheel_wheel "$gr_file" "$bz_file"
                ;;
            *py.typed*)
                pass_check "py.typed present in both wheels"
                ;;
            *)
                ((skipped++))
                ;;
        esac
    done < <(comm -12 "$gr_listing" "$bz_listing")

    if [[ $different -eq 0 ]]; then
        pass_check "All $identical .py/.pyi files are identical ($skipped metadata files skipped)"
    else
        fail_check "$different .py/.pyi files differ out of $((identical + different)) compared"
    fi
}

compare_wheel_metadata() {
    local gr_meta="$1" bz_meta="$2"

    # Compare key fields only.
    local fields=("Name" "Version" "Requires-Python" "License")
    for field in "${fields[@]}"; do
        local gr_val bz_val
        gr_val=$(grep "^${field}:" "$gr_meta" | head -1 | sed "s/^${field}: *//")
        bz_val=$(grep "^${field}:" "$bz_meta" | head -1 | sed "s/^${field}: *//")
        if [[ "$gr_val" == "$bz_val" ]]; then
            pass_check "METADATA ${field}: '$gr_val'"
        elif [[ -z "$gr_val" || -z "$bz_val" ]]; then
            warn_check "METADATA ${field}: goreleaser='${gr_val:-<missing>}' bazel='${bz_val:-<missing>}'"
        else
            fail_check "METADATA ${field}: goreleaser='$gr_val' bazel='$bz_val'"
        fi
    done

    # Compare Requires-Dist (sorted, since ordering may differ).
    local gr_deps bz_deps
    gr_deps=$(grep "^Requires-Dist:" "$gr_meta" | sed 's/^Requires-Dist: *//' | sort)
    bz_deps=$(grep "^Requires-Dist:" "$bz_meta" | sed 's/^Requires-Dist: *//' | sort)
    if [[ "$gr_deps" == "$bz_deps" ]]; then
        pass_check "METADATA Requires-Dist: all dependencies match"
    else
        fail_check "METADATA Requires-Dist differs"
        diff <(echo "$gr_deps") <(echo "$bz_deps") | head -10 | sed 's/^/         /'
    fi

    # Compare Classifiers (sorted).
    local gr_cls bz_cls
    gr_cls=$(grep "^Classifier:" "$gr_meta" | sed 's/^Classifier: *//' | sort)
    bz_cls=$(grep "^Classifier:" "$bz_meta" | sed 's/^Classifier: *//' | sort)
    if [[ "$gr_cls" == "$bz_cls" ]]; then
        pass_check "METADATA Classifiers: match"
    else
        fail_check "METADATA Classifiers differ"
    fi

    # Warn-only: Metadata-Version.
    local gr_mv bz_mv
    gr_mv=$(grep "^Metadata-Version:" "$gr_meta" | head -1)
    bz_mv=$(grep "^Metadata-Version:" "$bz_meta" | head -1)
    if [[ "$gr_mv" != "$bz_mv" ]]; then
        warn_check "METADATA version header: goreleaser='$gr_mv' bazel='$bz_mv'"
    fi
}

compare_wheel_wheel() {
    local gr_wheel="$1" bz_wheel="$2"

    # Check Tag.
    local gr_tag bz_tag
    gr_tag=$(grep "^Tag:" "$gr_wheel" | head -1)
    bz_tag=$(grep "^Tag:" "$bz_wheel" | head -1)
    if [[ "$gr_tag" == "$bz_tag" ]]; then
        pass_check "WHEEL Tag: $gr_tag"
    else
        fail_check "WHEEL Tag: goreleaser='$gr_tag' bazel='$bz_tag'"
    fi

    # Warn-only: Generator.
    local gr_gen bz_gen
    gr_gen=$(grep "^Generator:" "$gr_wheel" | head -1)
    bz_gen=$(grep "^Generator:" "$bz_wheel" | head -1)
    if [[ "$gr_gen" != "$bz_gen" ]]; then
        warn_check "WHEEL Generator differs (expected): goreleaser='$gr_gen' bazel='$bz_gen'"
    fi
}

# ── Main ─────────────────────────────────────────────────────────────────────

echo "${BOLD}Comparing release artifacts${RESET}"
echo "  GoReleaser dir: $GR_DIR"
echo "  Bazel dir:      $BZ_DIR"
[[ -n "$VERSION" ]] && echo "  Version:        $VERSION"

compare_platform_archives
compare_npm_tarball
compare_python_wheel

echo ""
echo "${BOLD}=== Summary ===${RESET}"
echo "  ${GREEN}PASS${RESET}: $PASS"
echo "  ${RED}FAIL${RESET}: $FAIL"
echo "  ${YELLOW}WARN${RESET}: $WARN"

if [[ $FAIL -gt 0 ]]; then
    echo ""
    echo "${RED}Verification FAILED${RESET}"
    exit 1
else
    echo ""
    echo "${GREEN}Verification PASSED${RESET}"
    exit 0
fi
