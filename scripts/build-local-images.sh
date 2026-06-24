#!/usr/bin/env bash
set -euo pipefail

SERVER_TAG="${SERVER_TAG:-memoh-server:mimo-local}"
WEB_TAG="${WEB_TAG:-memoh-web:mimo-local}"
OUTPUT_DIR="${OUTPUT_DIR:-dist/local-images}"
MIRROR_PREFIX="${MIRROR_PREFIX:-docker.1ms.run}"
PLATFORM="${PLATFORM:-}"
SKIP_SAVE=0
KEEP_TEMP_DOCKERFILES=0
NO_SYNTAX_FRONTEND=0

usage() {
  cat <<'EOF'
Usage: scripts/build-local-images.sh [options]

Options:
  --server-tag TAG         Server image tag. Default: memoh-server:mimo-local
  --web-tag TAG            Web image tag. Default: memoh-web:mimo-local
  --output-dir DIR         Output directory for tar files. Default: dist/local-images
  --mirror-prefix PREFIX   Registry mirror prefix. Default: docker.1ms.run
  --platform PLATFORM      Optional docker build platform, e.g. linux/amd64
  --skip-save              Build images only, do not export tar files
  --keep-temp-dockerfiles  Keep rewritten temporary Dockerfiles
  --no-syntax-frontend     Strip the '# syntax=' line from temporary Dockerfiles
  -h, --help               Show this help text
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --server-tag)
      SERVER_TAG="$2"
      shift 2
      ;;
    --web-tag)
      WEB_TAG="$2"
      shift 2
      ;;
    --output-dir)
      OUTPUT_DIR="$2"
      shift 2
      ;;
    --mirror-prefix)
      MIRROR_PREFIX="$2"
      shift 2
      ;;
    --platform)
      PLATFORM="$2"
      shift 2
      ;;
    --skip-save)
      SKIP_SAVE=1
      shift
      ;;
    --keep-temp-dockerfiles)
      KEEP_TEMP_DOCKERFILES=1
      shift
      ;;
    --no-syntax-frontend)
      NO_SYNTAX_FRONTEND=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

step() {
  echo "==> $1"
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Required command not found: $1" >&2
    exit 1
  fi
}

git_value() {
  local fallback="$1"
  shift
  if git "$@" >/dev/null 2>&1; then
    git "$@" | head -n 1
  else
    echo "$fallback"
  fi
}

map_image() {
  local image="$1"
  if [[ -z "$MIRROR_PREFIX" || "$image" == "scratch" ]]; then
    echo "$image"
    return
  fi
  if [[ "$image" == "$MIRROR_PREFIX/"* ]]; then
    echo "$image"
  elif [[ "$image" == */* ]]; then
    echo "$MIRROR_PREFIX/$image"
  else
    echo "$MIRROR_PREFIX/library/$image"
  fi
}

rewrite_dockerfile() {
  local src="$1"
  local dst="$2"

  : >"$dst"
  while IFS= read -r line || [[ -n "$line" ]]; do
    if [[ "$NO_SYNTAX_FRONTEND" -eq 1 && "$line" =~ ^[[:space:]]*#[[:space:]]*syntax= ]]; then
      continue
    fi
    if [[ "$line" =~ ^(FROM([[:space:]]+--platform=[^[:space:]]+)?[[:space:]]+)([^[:space:]]+)(.*)$ ]]; then
      local prefix="${BASH_REMATCH[1]}"
      local image="${BASH_REMATCH[3]}"
      local suffix="${BASH_REMATCH[4]}"
      printf '%s%s%s\n' "$prefix" "$(map_image "$image")" "$suffix" >>"$dst"
      continue
    fi
    printf '%s\n' "$line" >>"$dst"
  done <"$src"
}

archive_name() {
  echo "$1" | sed 's#[:/\\]#-#g'
}

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

require_command docker
require_command git

VERSION="$(git_value dev describe --tags --always --dirty)"
COMMIT_HASH="$(git_value unknown rev-parse --short HEAD)"
BUILD_TIME="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/memoh-local-image-build.XXXXXX")"
TEMP_SERVER_DOCKERFILE="$TEMP_ROOT/Dockerfile.server.local"
TEMP_WEB_DOCKERFILE="$TEMP_ROOT/Dockerfile.web.local"

cleanup() {
  if [[ "$KEEP_TEMP_DOCKERFILES" -ne 1 ]]; then
    rm -rf "$TEMP_ROOT"
  fi
}
trap cleanup EXIT

step "Preparing temporary Dockerfiles"
rewrite_dockerfile "$REPO_ROOT/docker/Dockerfile.server" "$TEMP_SERVER_DOCKERFILE"
rewrite_dockerfile "$REPO_ROOT/docker/Dockerfile.web" "$TEMP_WEB_DOCKERFILE"

SERVER_ARGS=(
  build
  -f "$TEMP_SERVER_DOCKERFILE"
  -t "$SERVER_TAG"
  --build-arg "VERSION=$VERSION"
  --build-arg "COMMIT_HASH=$COMMIT_HASH"
  --build-arg "BUILD_TIME=$BUILD_TIME"
)
WEB_ARGS=(
  build
  -f "$TEMP_WEB_DOCKERFILE"
  -t "$WEB_TAG"
)

if [[ -n "$PLATFORM" ]]; then
  SERVER_ARGS+=(--platform "$PLATFORM")
  WEB_ARGS+=(--platform "$PLATFORM")
fi

SERVER_ARGS+=(.)
WEB_ARGS+=(.)

step "Building server image $SERVER_TAG"
docker "${SERVER_ARGS[@]}"

step "Building web image $WEB_TAG"
docker "${WEB_ARGS[@]}"

if [[ "$SKIP_SAVE" -ne 1 ]]; then
  OUT_DIR="$REPO_ROOT/$OUTPUT_DIR"
  mkdir -p "$OUT_DIR"

  SERVER_TAR="$OUT_DIR/$(archive_name "$SERVER_TAG").tar"
  WEB_TAR="$OUT_DIR/$(archive_name "$WEB_TAG").tar"
  OVERRIDE_PATH="$OUT_DIR/docker-compose.local-images.yml"

  step "Exporting tarballs to $OUT_DIR"
  docker save -o "$SERVER_TAR" "$SERVER_TAG"
  docker save -o "$WEB_TAR" "$WEB_TAG"

  cat >"$OVERRIDE_PATH" <<EOF
services:
  migrate:
    image: $SERVER_TAG
  server:
    image: $SERVER_TAG
  web:
    image: $WEB_TAG
EOF

  echo
  echo "Artifacts:"
  echo "  $SERVER_TAR"
  echo "  $WEB_TAR"
  echo "  $OVERRIDE_PATH"
fi

echo
echo "Build complete."
echo "Server image: $SERVER_TAG"
echo "Web image:    $WEB_TAG"
echo "Version:      $VERSION"
echo "Commit:       $COMMIT_HASH"
