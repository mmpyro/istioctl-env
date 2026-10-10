#!/usr/bin/env bats
# tests/integration.bats

setup() {
	# Load helper libraries
	load '/usr/local/lib/bats-support/load'
	load '/usr/local/lib/bats-assert/load'

	# Set up a clean environment for each test
	export ISTIOENV_ROOT="${BATS_TMPDIR}/istioenv"
	export PATH="${ISTIOENV_ROOT}/shims:${PATH}"
	mkdir -p "${ISTIOENV_ROOT}"
}

teardown() {
	# Clean up after each test
	rm -rf "${ISTIOENV_ROOT}"
}

@test "istioctl-env init creates necessary directories and shims" {
	run istioctl-env init
	assert_success
	[ -d "${ISTIOENV_ROOT}/versions" ]
	[ -f "${ISTIOENV_ROOT}/shims/istioctl" ]
}

@test "istioctl-env list-remote returns a list of versions" {
	istioctl-env init
	run istioctl-env list-remote
	assert_success
	assert_output --regexp "[0-9]+\.[0-9]+\.[0-9]+"
}

@test "istioctl-env latest returns a stable version" {
	istioctl-env init
	run istioctl-env latest
	assert_success
	assert_output --regexp "[0-9]+\.[0-9]+\.[0-9]+$"
}

@test "istioctl-env latest --prerelease returns a version" {
	istioctl-env init
	run istioctl-env latest --prerelease
	assert_success
	assert_output --regexp "[0-9]+\.[0-9]+\.[0-9]+"
}

@test "istioctl-env latest --help shows help text" {
	run istioctl-env latest --help
	assert_success
	assert_output --partial "latest"
	assert_output --partial "prerelease"
}

@test "istioctl-env install/list/global/which flow" {
	istioctl-env init
	local test_ver="1.24.0"

	# Install
	run istioctl-env install "${test_ver}"
	assert_success
	[ -f "${ISTIOENV_ROOT}/versions/${test_ver}/istioctl" ]

	# List
	run istioctl-env list
	assert_success
	assert_output --partial "${test_ver}"

	# Global
	run istioctl-env global "${test_ver}"
	assert_success

	run istioctl-env global
	assert_success
	assert_output --partial "${test_ver}"

	# Which
	run istioctl-env which
	assert_success
	assert_output --partial "${ISTIOENV_ROOT}/versions/${test_ver}/istioctl"
}

@test "istioctl-env local sets directory-specific version" {
	istioctl-env init
	local test_ver="1.24.0"

	# Must install it first because setup() wipes ISTIOENV_ROOT
	istioctl-env install "${test_ver}"

	local work_dir="${BATS_TMPDIR}/work"
	mkdir -p "${work_dir}"

	pushd "${work_dir}"
	run istioctl-env local "${test_ver}"
	assert_success
	[ -f ".istioctl-version" ]
	grep -q "${test_ver}" ".istioctl-version"

	run istioctl-env local
	assert_success
	assert_output --partial "${test_ver}"
	popd

	rm -rf "${work_dir}"
}

@test "istioctl-env shell outputs export command" {
	istioctl-env init
	local test_ver="1.24.0"
	istioctl-env install "${test_ver}"
	run istioctl-env shell "${test_ver}"
	assert_success
	assert_output --partial "export ISTIOENV_VERSION=${test_ver}"
}

@test "istioctl-env uninstall removes version" {
	istioctl-env init
	local test_ver="1.24.0"
	# Pre-install
	run istioctl-env install "${test_ver}"
	assert_success

	# Uninstall
	run istioctl-env uninstall "${test_ver}"
	assert_success
	[ ! -d "${ISTIOENV_ROOT}/versions/${test_ver}" ]

	run istioctl-env list
	assert_success
	refute_output --partial "${test_ver}"
}

@test "istioctl-env version" {
	run istioctl-env version
	assert_success
	assert_output --regexp "[0-9]+\.[0-9]+\.[0-9]+"
}

@test "istioctl-env help" {
	run istioctl-env help
	assert_success
	assert_output --partial "Commands:"
}

@test "istioctl-env status shows environment information" {
	istioctl-env init
	local test_ver="1.24.0"
	istioctl-env install "${test_ver}"
	istioctl-env global "${test_ver}"

	run istioctl-env status
	assert_success
	assert_output --partial "ISTIOENV_ROOT"
	assert_output --regexp "Active version:[[:space:]]+${test_ver}"
	assert_output --partial "set by global version file"
	assert_output --partial "* ${test_ver}"
}

@test "istioctl-env exec runs specific version" {
	istioctl-env init
	local test_ver="1.24.0"
	istioctl-env install "${test_ver}"

	# Run a command via exec
	run istioctl-env exec "${test_ver}" version
	assert_success
	assert_output --regexp "([vV]ersion|[0-9]+\.[0-9]+\.[0-9])"
}

@test "istioctl shim end-to-end" {
	istioctl-env init
	local test_ver="1.24.0"

	istioctl-env install "${test_ver}"
	istioctl-env global "${test_ver}"

	# Ensure shim is in PATH
	run istioctl version
	assert_output --regexp "([vV]ersion|[0-9]+\.[0-9]+\.[0-9])"
}

@test "istioctl-env completion outputs bash script" {
	# Force bash to avoid depending on $SHELL in the test container
	run istioctl-env completion --shell bash
	assert_success
	assert_output --partial "_istioctl_env_completions()"
	assert_output --partial "complete -F _istioctl_env_completions istioctl-env"
}

@test "istioctl-env completion --shell zsh emits #compdef header" {
	run istioctl-env completion --shell zsh
	assert_success
	assert_output --partial "#compdef istioctl-env"
	assert_output --partial "_arguments"
	assert_output --partial "istioctl-env list-remote --cached"
}

@test "istioctl-env completion --shell fish emits complete lines" {
	run istioctl-env completion --shell fish
	assert_success
	assert_output --partial "complete -c istioctl-env"
	assert_output --partial "__fish_use_subcommand"
	assert_output --partial "istioctl-env list-remote --cached"
}

@test "istioctl-env completion rejects unknown --shell value" {
	run istioctl-env completion --shell ksh
	assert_failure
	assert_output --partial "ksh"
}

@test "istioctl-env init --shell zsh emits function block" {
	istioctl-env init --shell zsh
	run istioctl-env init --shell zsh
	assert_success
	assert_output --partial "function istioctl-env()"
	assert_output --partial "shims:\$PATH"
}

@test "istioctl-env init --shell fish emits set -gx PATH" {
	run istioctl-env init --shell fish
	assert_success
	assert_output --partial "set -gx PATH"
	assert_output --partial "function istioctl-env"
}

@test "istioctl-env list-remote --cached returns versions without network" {
	istioctl-env init --shell bash
	run istioctl-env list-remote --cached
	assert_success
	assert_output --regexp "[0-9]+\.[0-9]+\.[0-9]+"
}

@test "istioctl-env completion --help shows help text" {
	run istioctl-env completion --help
	assert_success
	assert_output --partial "istioctl-env completion"
	assert_output --partial "--shell"
	assert_output --partial "powershell"
}

# ── Helpers ───────────────────────────────────────────────────────────────────

# fake_istioctl <version> <script body>: install a stub istioctl so tests can
# exercise resolution/exec without downloading anything.
fake_istioctl() {
	local dir="${ISTIOENV_ROOT}/versions/$1"
	mkdir -p "${dir}"
	printf '#!/bin/sh\n%s\n' "$2" > "${dir}/istioctl"
	chmod +x "${dir}/istioctl"
}

# ── completion ────────────────────────────────────────────────────────────────

@test "istioctl-env completion accepts every shell positionally and via --shell" {
	local shell
	for shell in bash zsh fish powershell pwsh; do
		run istioctl-env completion "${shell}"
		assert_success
		[ -n "${output}" ]
		run istioctl-env completion --shell "${shell}"
		assert_success
		[ -n "${output}" ]
	done
}

@test "istioctl-env completion powershell registers an argument completer" {
	run istioctl-env completion powershell
	assert_success
	assert_output --partial "Register-ArgumentCompleter -Native -CommandName istioctl-env"
	refute_output --partial "{BT}"
}

@test "istioctl-env completion scripts list completion, not autocompletion" {
	local shell
	for shell in bash zsh fish powershell; do
		run istioctl-env completion "${shell}"
		assert_success
		assert_output --partial "completion"
		refute_output --partial "autocompletion"
	done
}

@test "istioctl-env completion bash offers remote versions for install" {
	run istioctl-env completion bash
	assert_success
	assert_output --partial "install|resolve)"
	assert_output --partial "istioctl-env list-remote --cached"
}

@test "istioctl-env completion rejects unknown positional shell" {
	run istioctl-env completion ksh
	assert_failure
	assert_output --partial "ksh"
}

@test "istioctl-env autocompletion is a deprecated alias with a warning" {
	run istioctl-env autocompletion bash
	assert_success
	assert_output --partial "complete -F _istioctl_env_completions istioctl-env"
	assert_output --partial "istioctl-env: 'autocompletion' is deprecated; use 'istioctl-env completion'"

	# The warning goes to stderr only: stdout stays a clean script.
	run bash -c "istioctl-env autocompletion --shell zsh 2>/dev/null | head -n1"
	assert_success
	assert_output "#compdef istioctl-env"
}

@test "istioctl-env help lists completion but not autocompletion" {
	run istioctl-env help
	assert_success
	assert_output --partial "completion"
	refute_output --partial "autocompletion"
}

# ── -h / --help on every command ──────────────────────────────────────────────

@test "istioctl-env upgrade --help prints help and does not upgrade" {
	local bin
	bin="$(command -v istioctl-env)"
	local before
	before="$(cksum < "${bin}")"

	run istioctl-env upgrade --help
	assert_success
	assert_output --partial "Usage: istioctl-env upgrade"
	refute_output --partial "is already up to date (version"
	refute_output --partial "Upgraded istioctl-env"
	refute_output --partial "Downloading istioctl-env"

	run istioctl-env upgrade -h
	assert_success
	assert_output --partial "Usage: istioctl-env upgrade"

	[ "$(cksum < "${bin}")" = "${before}" ]
}

@test "istioctl-env -h short-circuits uninstall, shell, local, global, which, status, exec" {
	istioctl-env init
	local cmd
	for cmd in uninstall shell local global which status exec; do
		run istioctl-env "${cmd}" -h
		assert_success
		assert_output --partial "Usage: istioctl-env ${cmd}"
	done

	# Help must never be treated as a version.
	run istioctl-env global --help
	assert_success
	[ ! -f "${ISTIOENV_ROOT}/version" ]
}

# ── resolve ───────────────────────────────────────────────────────────────────

@test "istioctl-env resolve with no argument uses the active global version" {
	istioctl-env init
	fake_istioctl 1.24.0 'exit 0'
	istioctl-env global 1.24.0
	run istioctl-env resolve
	assert_success
	assert_output "1.24.0"
}

@test "istioctl-env resolve <constraint> matches an installed version" {
	istioctl-env init
	fake_istioctl 1.24.0 'exit 0'
	fake_istioctl 1.24.3 'exit 0'
	fake_istioctl 1.25.0 'exit 0'
	istioctl-env global 1.25.0
	run istioctl-env resolve "~1.24.0"
	assert_success
	assert_output "1.24.3"
}

@test "istioctl-env resolve fails for an unresolved spec" {
	istioctl-env init
	fake_istioctl 1.23.0 'exit 0'
	run istioctl-env resolve "^1.24.0"
	assert_failure
	assert_output --partial "no installed"
}

@test "istioctl-env resolve --install installs a missing version" {
	istioctl-env init
	run istioctl-env resolve 1.24.0 --install --silent
	assert_success
	assert_line "1.24.0"
	[ -x "${ISTIOENV_ROOT}/versions/1.24.0/istioctl" ]
}

# ── exec ──────────────────────────────────────────────────────────────────────

@test "istioctl-env exec strips a leading -- and sets ISTIOENV_VERSION" {
	istioctl-env init
	fake_istioctl 1.24.0 'echo "v=$ISTIOENV_VERSION args=$*"'
	run istioctl-env exec 1.24.0 -- analyze -- x
	assert_success
	assert_output "v=1.24.0 args=analyze -- x"
}

@test "istioctl-env exec propagates the child exit code" {
	istioctl-env init
	fake_istioctl 1.24.0 'exit 3'
	run istioctl-env exec 1.24.0 version
	[ "$status" -eq 3 ]
	refute_output --partial "exit status"
}

@test "istioctl-env exec propagates a real istioctl failure" {
	istioctl-env init
	istioctl-env install 1.24.0
	run istioctl-env exec 1.24.0 no-such-subcommand
	assert_failure
	refute_output --partial "exit status"
}

@test "istioctl-env exec resolves a constraint to the newest installed match" {
	istioctl-env init
	fake_istioctl 1.24.0 'echo "v=$ISTIOENV_VERSION"'
	fake_istioctl 1.24.3 'echo "v=$ISTIOENV_VERSION"'
	run istioctl-env exec "~1.24.0" version
	assert_success
	assert_output "v=1.24.3"
}

@test "istioctl-env exec --no-auto overrides ISTIOENV_AUTO_INSTALL" {
	istioctl-env init
	ISTIOENV_AUTO_INSTALL=true run istioctl-env exec --no-auto 1.24.0 version
	assert_failure
	assert_output --partial "not installed"
	[ ! -d "${ISTIOENV_ROOT}/versions/1.24.0" ]
}

@test "istioctl-env exec --auto installs a missing version" {
	istioctl-env init
	run istioctl-env exec --auto 1.24.0 version --remote=false
	assert_success
	[ -x "${ISTIOENV_ROOT}/versions/1.24.0/istioctl" ]
}

# ── prune ─────────────────────────────────────────────────────────────────────

@test "istioctl-env prune is a dry run by default" {
	istioctl-env init
	fake_istioctl 1.23.0 'exit 0'
	fake_istioctl 1.24.0 'exit 0'
	ISTIOENV_PRUNE_SCAN_ROOTS="" run istioctl-env prune
	assert_success
	[ -d "${ISTIOENV_ROOT}/versions/1.23.0" ]
	[ -d "${ISTIOENV_ROOT}/versions/1.24.0" ]
}

@test "istioctl-env prune --yes removes unreferenced versions" {
	istioctl-env init
	fake_istioctl 1.23.0 'exit 0'
	fake_istioctl 1.24.0 'exit 0'
	ISTIOENV_PRUNE_SCAN_ROOTS="" run istioctl-env prune --yes
	assert_success
	[ ! -d "${ISTIOENV_ROOT}/versions/1.23.0" ]
	[ ! -d "${ISTIOENV_ROOT}/versions/1.24.0" ]
}

@test "istioctl-env prune keeps a version referenced by a version file" {
	istioctl-env init
	fake_istioctl 1.23.0 'exit 0'
	fake_istioctl 1.24.0 'exit 0'
	local proj="${BATS_TMPDIR}/prune-proj"
	mkdir -p "${proj}"
	echo "1.24.0" > "${proj}/.istioctl-version"

	ISTIOENV_PRUNE_SCAN_ROOTS="${proj}" run istioctl-env prune --yes
	rm -rf "${proj}"
	assert_success
	[ ! -d "${ISTIOENV_ROOT}/versions/1.23.0" ]
	[ -d "${ISTIOENV_ROOT}/versions/1.24.0" ]
}

@test "istioctl-env prune --older-than rejects a bogus duration" {
	istioctl-env init
	run istioctl-env prune --older-than bogus
	assert_failure
	assert_output --partial "bogus"
}

# ── doctor ────────────────────────────────────────────────────────────────────

@test "istioctl-env doctor succeeds after init with shims on PATH" {
	istioctl-env init
	fake_istioctl 1.24.0 'exit 0'
	istioctl-env global 1.24.0
	run istioctl-env doctor
	assert_success
}

@test "istioctl-env doctor fails when shims are not on PATH" {
	istioctl-env init
	fake_istioctl 1.24.0 'exit 0'
	istioctl-env global 1.24.0
	PATH="${PATH//${ISTIOENV_ROOT}\/shims:/}" run istioctl-env doctor
	assert_failure
	assert_output --partial "[FAIL]"
	assert_output --partial "not on PATH"
}

@test "istioctl-env doctor --fix regenerates a tampered shim" {
	istioctl-env init
	fake_istioctl 1.24.0 'exit 0'
	istioctl-env global 1.24.0
	echo "tampered" > "${ISTIOENV_ROOT}/shims/istioctl"

	run istioctl-env doctor --fix
	refute_output --partial "tampered"
	run grep -q "tampered" "${ISTIOENV_ROOT}/shims/istioctl"
	assert_failure
	run istioctl-env doctor
	assert_success
}
