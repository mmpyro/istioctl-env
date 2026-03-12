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

@test "istioctl-env autocompletion outputs bash script" {
	run istioctl-env autocompletion
	assert_success
	assert_output --partial "_istioctl_env_completions()"
	assert_output --partial "complete -F _istioctl_env_completions istioctl-env"
}

@test "istioctl-env autocompletion --help shows help text" {
	run istioctl-env autocompletion --help
	assert_success
	assert_output --partial "autocompletion"
	assert_output --partial "source <(istioctl-env autocompletion)"
}
