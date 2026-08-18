#!/usr/bin/env bash
set -euo pipefail
go_motd_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
agent_dir="$(cd "${go_motd_dir}/../motd-status-agent" && pwd)"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --agent-dir) agent_dir="$2"; shift 2 ;;
    --go-motd-dir) go_motd_dir="$2"; shift 2 ;;
    *) printf 'Usage: %s [--agent-dir PATH] [--go-motd-dir PATH]\n' "$0" >&2; exit 2 ;;
  esac
done
tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/motd-status-integration.XXXXXX")"
agent_pid=""
cleanup() {
  if [[ -n "${agent_pid}" ]] && kill -0 "${agent_pid}" 2>/dev/null; then
    kill "${agent_pid}" 2>/dev/null || true
    wait "${agent_pid}" 2>/dev/null || true
  fi
  rm -rf "${tmp_dir}"
}
trap cleanup EXIT
socket_path="${tmp_dir}/agent.sock"
config_path="${tmp_dir}/motd.json"
fixture_path="${agent_dir}/testdata/v1/status.json"
agent_fixture_bin="${tmp_dir}/motd-status-agent-fixture"
motd_bin="${tmp_dir}/motd"
go -C "${agent_dir}" build -buildvcs=false -o "${agent_fixture_bin}" ./cmd/motd-status-agent-fixture
go -C "${go_motd_dir}" build -buildvcs=false -o "${motd_bin}" .
printf '{"system":{"container_status":{"socket_path":"%s","max_age":"30s"}}}\n' "${socket_path}" >"${config_path}"
"${agent_fixture_bin}" --socket "${socket_path}" --fixture "${fixture_path}" >"${tmp_dir}/agent.log" 2>&1 &
agent_pid=$!
for _ in {1..50}; do
  [[ -S "${socket_path}" ]] && break
  if ! kill -0 "${agent_pid}" 2>/dev/null; then
    sed -n '1,120p' "${tmp_dir}/agent.log" >&2
    exit 1
  fi
  sleep 0.02
done
if [[ ! -S "${socket_path}" ]]; then
  printf 'fixture server did not create %s\n' "${socket_path}" >&2
  sed -n '1,120p' "${tmp_dir}/agent.log" >&2
  exit 1
fi
json_output="$("${motd_bin}" -config "${config_path}" -json -no-color)"
printf '%s\n' "${json_output}" | grep -Fq '"containers"'
printf '%s\n' "${json_output}" | grep -Fq '"online": 2'
printf '%s\n' "${json_output}" | grep -Fq '"total": 5'
printf '%s\n' "${json_output}" | grep -Fq '"name": "c"'
printf '%s\n' "${json_output}" | grep -Fq '"health": "starting"'
terminal_output="$("${motd_bin}" -config "${config_path}" -no-color)"
printf '%s\n' "${terminal_output}" | grep -Fq '2 of 5 online'
if printf '%s\n' "${terminal_output}" | grep -Eq '"name":|"health":'; then
  printf 'terminal output unexpectedly exposed workload details:\n%s\n' "${terminal_output}" >&2
  exit 1
fi
kill "${agent_pid}"
wait "${agent_pid}" 2>/dev/null || true
agent_pid=""
offline_json="$("${motd_bin}" -config "${config_path}" -json -no-color)"
if printf '%s\n' "${offline_json}" | grep -Fq '"containers"'; then
  printf 'offline consumer incorrectly retained container status:\n%s\n' "${offline_json}" >&2
  exit 1
fi
printf 'status-agent consumer integration passed\n'
