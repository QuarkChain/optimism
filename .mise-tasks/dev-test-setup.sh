#!/usr/bin/env bash

#MISE description="One-time setup for local dev-test dependencies"
#MISE alias="dt-setup"

set -e
set -E -o pipefail

halt() {
    echo "Error: $*" >&2
    return 1 2>/dev/null || exit 1
}

run_step() {
    local label="$1"
    shift
    echo "==========Starting ${label}..."
    "$@"
    echo "===================${label} done."
}

require_command() {
    local cmd="$1"
    local help_msg="$2"
    if ! command -v "$cmd" >/dev/null 2>&1; then
        halt "Missing required command: $cmd. ${help_msg}"
    fi
}

install_system_package() {
    local pkg="$1"
    local install_hint=""

    case "$pkg" in
        docker) install_hint="docker.io (or docker)" ;;
        *) install_hint="$pkg" ;;
    esac

    case "$(uname -s)" in
        Darwin)
            require_command brew "Homebrew is required to install ${install_hint} on macOS."
            if [ "$pkg" = "docker" ]; then
                brew install --cask docker
            else
                brew install "$pkg"
            fi
            ;;
        Linux)
            local SUDO=""
            if [ "${EUID:-$(id -u)}" -ne 0 ]; then
                if command -v sudo >/dev/null 2>&1; then
                    SUDO="sudo -E"
                else
                    halt "Need to install '${install_hint}' but current user is not root and 'sudo' is unavailable. Please install it manually."
                fi
            fi

            if command -v apt-get >/dev/null 2>&1; then
                ${SUDO} apt-get update
                if [ "$pkg" = "docker" ]; then
                    ${SUDO} apt-get install -y docker.io
                else
                    ${SUDO} apt-get install -y "$pkg"
                fi
            elif command -v dnf >/dev/null 2>&1; then
                if [ "$pkg" = "docker" ]; then
                    ${SUDO} dnf install -y docker
                else
                    ${SUDO} dnf install -y "$pkg"
                fi
            elif command -v apk >/dev/null 2>&1; then
                if [ "$pkg" = "docker" ]; then
                    ${SUDO} apk add --no-cache docker
                else
                    ${SUDO} apk add --no-cache "$pkg"
                fi
            else
                halt "No supported package manager found to install $pkg."
            fi
            ;;
        *)
            halt "Unsupported OS for automatic installation: $(uname -s)"
            ;;
    esac
}

require_clang_c_headers() {
    local probe='#include <stdarg.h>
#include <stdbool.h>
int main(void){return 0;}'

    if ! printf "%s\n" "$probe" | clang -x c - -fsyntax-only >/dev/null 2>&1; then
        halt "Missing C toolchain headers for clang (stdarg.h/stdbool.h). Install system deps manually (Ubuntu/Debian: build-essential clang libc6-dev libclang-dev)."
    fi
}

echo "==========Preparing local dev-test environment..."
require_command mise "Install mise first so this setup script can provision repo-managed tools."

run_step "mise install" mise install

if [ -z "${MISE_SHELL:-}" ]; then
    if [ -n "${ZSH_VERSION:-}" ]; then
        eval "$(mise activate zsh)"
    elif [ -n "${BASH_VERSION:-}" ]; then
        eval "$(mise activate bash)"
    fi
fi

command -v m4 >/dev/null 2>&1 || install_system_package m4
command -v clang >/dev/null 2>&1 || install_system_package clang
command -v docker >/dev/null 2>&1 || install_system_package docker
command -v zstd >/dev/null 2>&1 || install_system_package zstd
require_clang_c_headers

command -v cargo-binstall >/dev/null 2>&1 || cargo install cargo-binstall --locked
command -v cargo-nextest >/dev/null 2>&1 || cargo binstall --no-confirm cargo-nextest
cargo nextest --version >/dev/null 2>&1 || halt "Failed to install cargo-nextest."

echo "==========dev-test setup complete."
echo "Now run: mise run dt"
