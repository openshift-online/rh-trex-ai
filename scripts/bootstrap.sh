#!/usr/bin/env bash
# TRex bootstrap: kickstart a new TRex-based application.
#
# Usage:
#   curl -sSL <url>/scripts/bootstrap.sh | bash -s -- <project-name> [repo-base] [dir]
#
# Arguments:
#   <project-name>  required, e.g. my-service
#   [repo-base]     Go module base path, default github.com/example
#   [dir]           parent directory to create the project in, default current dir
#
# Environment:
#   TREX_REPO_URL   git URL of the TRex source (default: https://github.com/openshift-online/rh-trex-ai)
#   TREX_BRANCH    branch/tag to fetch (default: remote's default branch)
#   TREX_SRC       existing local TRex source checkout (skips cloning)
#   TREX_HOME      cache dir for the source clone (default: $HOME/.trex)
#   SKIP_BUILD     set to 1 to skip make generate + go mod tidy + make binary
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: curl -sSL <url>/scripts/bootstrap.sh | bash -s -- <project-name> [repo-base] [dir]

Creates a new TRex-based application in <dir>/<project-name>.

  <project-name>  required, e.g. my-service
  [repo-base]     module base path, default github.com/example
  [dir]           parent directory, default current directory

Environment:
  TREX_REPO_URL   git URL of TRex source (default: https://github.com/openshift-online/rh-trex-ai)
  TREX_BRANCH    branch/tag to fetch (default: remote's default branch)
  TREX_SRC       existing local TRex source checkout (skips cloning)
  TREX_HOME      cache dir for the source clone (default: $HOME/.trex)
  SKIP_BUILD     set to 1 to skip make generate + go mod tidy + make binary
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

NAME="${1:-}"
REPO="${2:-github.com/example}"
DIR="${3:-.}"

if [[ -z "$NAME" ]]; then
  echo "error: project name is required" >&2
  usage >&2
  exit 1
fi

for tool in git go; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "error: required tool '$tool' not found in PATH" >&2
    exit 1
  fi
done

TREX_REPO_URL="${TREX_REPO_URL:-https://github.com/openshift-online/rh-trex-ai}"
TREX_BRANCH="${TREX_BRANCH:-}"
TREX_HOME="${TREX_HOME:-$HOME/.trex}"

# Locate or fetch the TRex source.
if [[ -n "${TREX_SRC:-}" ]]; then
  SRC="$TREX_SRC"
else
  SRC="$TREX_HOME/src"
  if [[ ! -d "$SRC/components/api-server" ]]; then
    echo "==> fetching TRex source ($TREX_REPO_URL)"
    mkdir -p "$TREX_HOME"
    if [[ -n "$TREX_BRANCH" ]]; then
      git clone --depth 1 --branch "$TREX_BRANCH" "$TREX_REPO_URL" "$SRC"
    else
      # Use the remote's default branch.
      git clone --depth 1 "$TREX_REPO_URL" "$SRC"
    fi
  else
    echo "==> reusing TRex source at $SRC"
  fi
fi

SRC="$(cd "$SRC" && pwd)"
API_SERVER="$SRC/components/api-server"
if [[ ! -f "$API_SERVER/go.mod" ]]; then
  echo "error: TRex source has no components/api-server module at $API_SERVER" >&2
  exit 1
fi

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

echo "==> building trex CLI"
( cd "$API_SERVER" && go build -o "$WORK/trex" ./cmd/trex )

echo "==> scaffolding $NAME"
"$WORK/trex" new-project "$NAME" --repo "$REPO" --dir "$DIR"

PROJ="$DIR/$NAME"
if [[ ! -f "$PROJ/go.mod" ]]; then
  echo "error: scaffold did not produce $PROJ/go.mod" >&2
  exit 1
fi

# Point the replace directive at the TRex source we used.
REPLACE_OLD="replace github.com/openshift-online/rh-trex-ai/components/api-server => ../path/to/rh-trex-ai/components/api-server"
REPLACE_NEW="replace github.com/openshift-online/rh-trex-ai/components/api-server => $API_SERVER"
if grep -qF "$REPLACE_OLD" "$PROJ/go.mod"; then
  sed -i.bak "s#$REPLACE_OLD#$REPLACE_NEW#" "$PROJ/go.mod"
  rm -f "$PROJ/go.mod.bak"
fi

echo "==> project created at $PROJ"

if [[ "${SKIP_BUILD:-0}" != "1" ]]; then
  echo "==> building project (make generate && go mod tidy && make binary)"
  if ( cd "$PROJ" && make generate && go mod tidy && make binary ); then
    echo "==> build OK"
  else
    echo "warning: automated build failed; continue manually (see next steps)" >&2
  fi
fi

cat <<EOF

Next steps:
  cd $PROJ
  ./$NAME new-entity --kind HelloWorld --fields "message:string:required" \\
    --project $NAME --repo $REPO \\
    --library github.com/openshift-online/rh-trex-ai/components/api-server \\
    --module $REPO/$NAME --proto-package ${NAME//-/_}
  make proto && make generate && go build ./...
EOF
