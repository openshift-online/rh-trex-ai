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
# How it works:
#   1. Writes a minimal Go harness in a temp dir: a go.mod that requires the
#      TRex library plus a main.go that imports pkg/cmd and inherits the
#      commands (new-project, generate-entity, ...). No separate CLI build.
#   2. Runs the inherited `new-project` command, which scaffolds the complete
#      application from the library's embedded template.
#   3. Fixes the scaffolded project's `replace` directive to point at the
#      TRex source used (or removes it when TREX_PUBLISHED=1).
#   4. Optionally verifies the scaffold builds.
#
# Environment:
#   TREX_REPO_URL   git URL of the TRex source (default: https://github.com/openshift-online/rh-trex-ai)
#   TREX_BRANCH    branch/tag to fetch (default: remote's default branch)
#   TREX_SRC       existing local TRex source checkout (skips cloning; used for replace)
#   TREX_HOME      cache dir for the source clone (default: $TMPDIR/trex (does not pollute the home disk))
#   TREX_PUBLISHED set to 1 when the TRex library is published to a module
#                   proxy; skips cloning and replace entirely
#   SKIP_BUILD     set to 0 to run a verification build (make generate + tidy + binary) after scaffolding; default 1 (skip)
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
  TREX_HOME      cache dir for the source clone (default: $TMPDIR/trex (does not pollute the home disk))
  TREX_PUBLISHED set to 1 when the TRex library is published (skips clone/replace)
  SKIP_BUILD     set to 0 to run a verification build after scaffolding; default 1 (skip)
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

# Resolve DIR to an absolute path so it is independent of the harness cwd.
DIR="$(cd "$DIR" && pwd)"

for tool in git go; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "error: required tool '$tool' not found in PATH" >&2
    exit 1
  fi
done

TREX_REPO_URL="${TREX_REPO_URL:-https://github.com/openshift-online/rh-trex-ai}"
TREX_BRANCH="${TREX_BRANCH:-}"
TREX_HOME="${TREX_HOME:-${TMPDIR:-/tmp}/trex}"

LIB_MODULE=github.com/openshift-online/rh-trex-ai/components/api-server

# A customized origin/branch means the user explicitly asked for a specific
# source; never reuse a possibly-stale cache for it.
CUSTOM_ORIGIN=0
if [[ "$TREX_REPO_URL" != "https://github.com/openshift-online/rh-trex-ai" ]] || [[ -n "$TREX_BRANCH" ]]; then
  CUSTOM_ORIGIN=1
fi

# Resolve the TRex source used for the `replace` directive.
SRC=""
if [[ "${TREX_PUBLISHED:-0}" != "1" ]]; then
  if [[ -n "${TREX_SRC:-}" ]]; then
    SRC="$TREX_SRC"
  else
    SRC="$TREX_HOME/src"
    if [[ -n "$TREX_BRANCH" ]]; then
      SRC="$TREX_HOME/src-$TREX_BRANCH"
    fi
    if [[ "$CUSTOM_ORIGIN" == "1" && -d "$SRC" ]]; then
      echo "==> refreshing TRex source cache for custom origin/branch"
      rm -rf "$SRC"
    fi
    if [[ ! -d "$SRC/components/api-server" ]]; then
      echo "==> fetching TRex source ($TREX_REPO_URL)"
      mkdir -p "$TREX_HOME"
      if [[ -n "$TREX_BRANCH" ]]; then
        git clone --depth 1 --branch "$TREX_BRANCH" "$TREX_REPO_URL" "$SRC"
      else
        git clone --depth 1 "$TREX_REPO_URL" "$SRC"
      fi
    else
      echo "==> reusing TRex source at $SRC"
    fi
  fi
  SRC="$(cd "$SRC" && pwd)"
  if [[ ! -f "$SRC/components/api-server/go.mod" ]]; then
    echo "error: TRex source has no components/api-server module at $SRC" >&2
    exit 1
  fi
  # Guard against an outdated source that predates the new-project generator.
  if [[ ! -f "$SRC/components/api-server/pkg/cmd/newproject.go" ]]; then
    echo "error: the TRex source at $SRC predates the new-project generator." >&2
    echo "       Set TREX_SRC (or TREX_REPO_URL/TREX_BRANCH) to a source that contains" >&2
    echo "       pkg/cmd/newproject.go (e.g. the generator-embedded branch)." >&2
    exit 1
  fi
fi

# Build a minimal harness that inherits the commands from the library.
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
# Seed from the template's canonical go.mod (full require block) so the
# dependency graph resolves cleanly, then rename the module and the replace.
if [[ -n "$SRC" ]]; then
  cp "$SRC/components/api-server/templates/new-project/go.mod.tmpl" "$WORK/go.mod"
  sed -i.bak \
    -e 's#module github.com/example/my-service#module github.com/acme/bootstrap-harness#' \
    -e "s#../path/to/rh-trex-ai/components/api-server#$SRC/components/api-server#" \
    "$WORK/go.mod"
  rm -f "$WORK/go.mod.bak"
else
  cat > "$WORK/go.mod" <<EOF
module github.com/acme/bootstrap-harness

go 1.24.2

toolchain go1.24.9

require $LIB_MODULE v0.0.0-00010101000000-000000000000
EOF
fi

cat > "$WORK/main.go" <<'EOF'
package main

import (
	"os"

	pkgcmd "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/cmd"
)

func main() {
	root := pkgcmd.NewRootCommand("bootstrap", "Bootstrap harness")
	root.AddCommand(
		pkgcmd.NewProjectCommand(),
		pkgcmd.NewGenerateEntityCommand(),
	)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
EOF

echo "==> preparing harness (go mod tidy)"
( cd "$WORK" && go mod tidy >/dev/null 2>&1 )

echo "==> scaffolding $NAME"
( cd "$WORK" && go run . new-project "$NAME" --repo "$REPO" --dir "$DIR" )

PROJ="$DIR/$NAME"
if [[ ! -f "$PROJ/go.mod" ]]; then
  echo "error: scaffold did not produce $PROJ/go.mod" >&2
  exit 1
fi

# Fix the replace directive in the scaffolded project.
if [[ -n "$SRC" ]]; then
  REPLACE_OLD="replace $LIB_MODULE => ../path/to/rh-trex-ai/components/api-server"
  REPLACE_NEW="replace $LIB_MODULE => $SRC/components/api-server"
  if grep -qF "$REPLACE_OLD" "$PROJ/go.mod"; then
    sed -i.bak "s#$REPLACE_OLD#$REPLACE_NEW#" "$PROJ/go.mod"
    rm -f "$PROJ/go.mod.bak"
  fi
else
  sed -i.bak "/^replace $LIB_MODULE =>/d" "$PROJ/go.mod"
  rm -f "$PROJ/go.mod.bak"
fi

echo "==> project created at $PROJ"

if [[ "${SKIP_BUILD:-1}" != "1" ]]; then
  echo "==> building project (make generate && go mod tidy && make binary)"
  if ( cd "$PROJ" && make generate && go mod tidy && make binary ); then
    echo "==> build OK"
  else
    echo "warning: automated build failed; continue manually (see next steps)" >&2
  fi
fi

cat <<EOF

Next steps:
  1. Declare your entities in $PROJ/specs/entities.yaml
     (see the TRex specs/codegen/entities.yaml for the same examples: Dinosaur,
      Scientist, Fossil). Field syntax: name:type[:required|optional]
  2. Generate the API code from that manifest:
     cd $PROJ
     make generate-entities       # writes plugins/*, protos, OpenAPI refs
     make proto && make generate  # protobuf stubs + OpenAPI client
     make binary                   # build the server binary (./$NAME)
  3. Optional: build the client component (CLI + TUI):
     make ctl                     # builds ./generated/ctl/${NAME}ctl
  4. Run:
     make db/setup                # start PostgreSQL
     ./$NAME migrate
     ./$NAME serve --enable-authz=false --enable-jwt=false
EOF
