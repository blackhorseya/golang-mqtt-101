#!/usr/bin/env bash
# 透過真正的 `task broker:up` 入口驗證 container CLI 的選擇邏輯。
# 用受控 PATH + stub CLI，不需要真的 docker / podman，也不會啟動容器。
#
# 用法：test/taskfile_container_cli_test.sh（在 repo root 執行）
set -u

TASK_BIN="$(command -v task)"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORK="$(mktemp -d)"
trap 'command rm -rf "$WORK"' EXIT

fail=0

# Taskfile 的 vars 只需要這些外部指令；只把它們放進 PATH，
# 不放整個 /usr/bin，否則系統裝了 docker（例如 GitHub Actions 的 ubuntu runner）就測不到「只有 podman」的情況
TOOLS="$WORK/tools"
mkdir -p "$TOOLS"
for tool in sh find sed tr ls dirname; do
  ln -s "$(command -v "$tool")" "$TOOLS/$tool"
done

# make_stub <dir> <name>：建立一個把自己名字與參數寫進 $WORK/calls 的假 CLI
make_stub() {
  mkdir -p "$1"
  cat >"$1/$2" <<STUB
#!/bin/sh
echo "$2 \$*" >>"$WORK/calls"
STUB
  chmod +x "$1/$2"
}

# run_case <名稱> <預期 CLI> <stub 目錄> [env...]
run_case() {
  local name="$1" want="$2" stubs="$3"
  shift 3
  : >"$WORK/calls"
  local out
  out="$(cd "$ROOT" && env -i HOME="$HOME" PATH="$stubs:$TOOLS" "$@" "$TASK_BIN" broker:up 2>&1)"
  local rc=$?
  local got
  got="$(cat "$WORK/calls")"
  if [ $rc -eq 0 ] && [ "$got" = "$want compose -f deployments/docker/docker-compose.yaml up -d" ]; then
    echo "PASS $name"
  else
    echo "FAIL $name (exit=$rc)"
    echo "  want call: $want compose -f deployments/docker/docker-compose.yaml up -d"
    echo "  got calls: ${got:-<none>}"
    echo "  output:    $out"
    fail=1
  fi
}

# 1. 只有 podman（例如 docker 只是 shell alias）→ 用 podman
make_stub "$WORK/only-podman" podman
run_case "only podman on PATH" "podman" "$WORK/only-podman"

# 2. docker 與 podman 都有 → 優先 docker
make_stub "$WORK/both" docker
make_stub "$WORK/both" podman
run_case "docker preferred over podman" "docker" "$WORK/both"

# 3. CONTAINER_CLI 環境變數覆寫自動偵測
make_stub "$WORK/override" nerdctl
make_stub "$WORK/override" docker
run_case "CONTAINER_CLI override" "nerdctl" "$WORK/override" CONTAINER_CLI=nerdctl

exit $fail
