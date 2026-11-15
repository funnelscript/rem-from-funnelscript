#!/usr/bin/env bash
# ==============================================================================
# REM | One-Command Universal Installer
# Unified Reverse Engineering & Clean-Room Software Reconstruction System
# ==============================================================================

set -euo pipefail

BOLD="\033[1m"
GREEN="\033[0;32m"
BLUE="\033[0;34m"
YELLOW="\033[0;33m"
CYAN="\033[0;36m"
RED="\033[0;31m"
RESET="\033[0m"

echo -e "${CYAN}${BOLD}"
cat << 'EOF'
  ____  _____ __  __ 
 |  _ \| ____|  \/  |
 | |_) |  _| | |\/| |
 |  _ <| |___| |  | |
 |_| \_\_____|_|  |_|
  Reverse Engineering & Software Reconstruction System
EOF
echo -e "${RESET}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OS="$(uname -s)"
ARCH="$(uname -m)"

echo -e "${BLUE}▶ Detecting system environment...${RESET}"
echo -e "  OS:           ${BOLD}${OS}${RESET}"
echo -e "  Architecture: ${BOLD}${ARCH}${RESET}"
echo -e "  Source root:  ${BOLD}${SCRIPT_DIR}${RESET}"

# 1. Build or locate Go binary
echo -e "\n${BLUE}▶ Building / verifying compiled binary...${RESET}"
BIN_SOURCE="${SCRIPT_DIR}/rem-from-funnelscript"
if [ ! -d "${BIN_SOURCE}" ]; then
  BIN_SOURCE="${SCRIPT_DIR}"
fi
BIN_TARGET="${SCRIPT_DIR}/rem"

if command -v go >/dev/null 2>&1; then
  echo -e "  Building ${BOLD}rem${RESET} using $(go version)..."
  (cd "${BIN_SOURCE}" && go build -trimpath -ldflags="-s -w" -o "${BIN_TARGET}" ./cmd/rem)
  echo -e "  ${GREEN}✔ Compiled binary successfully:${RESET} ${BIN_TARGET}"
elif [ -f "${BIN_TARGET}" ]; then
  echo -e "  ${GREEN}✔ Using existing compiled binary:${RESET} ${BIN_TARGET}"
else
  echo -e "  ${RED}✖ Go is not installed and precompiled binary was not found.${RESET}"
  echo -e "    Please install Go (https://go.dev) or download a release binary."
  exit 1
fi

chmod +x "${BIN_TARGET}"

# 2. Install binary to user PATH (~/.local/bin or /usr/local/bin)
INSTALL_DIR="${HOME}/.local/bin"
if [ -w "/usr/local/bin" ]; then
  INSTALL_DIR="/usr/local/bin"
fi

mkdir -p "${INSTALL_DIR}"
cp -f "${BIN_TARGET}" "${INSTALL_DIR}/rem"
chmod +x "${INSTALL_DIR}/rem"
echo -e "  ${GREEN}✔ Installed executable to:${RESET} ${INSTALL_DIR}/rem"

# Ensure INSTALL_DIR is in PATH
if [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
  echo -e "  ${YELLOW}⚠ Note: ${INSTALL_DIR} is not in your current PATH.${RESET}"
  echo -e "    Add it with: export PATH=\"${INSTALL_DIR}:\$PATH\""
fi

# 3. Setup Browser Automation Driver (Bun / Node + Playwright)
echo -e "\n${BLUE}▶ Setting up browser extraction drivers...${RESET}"
if command -v bun >/dev/null 2>&1; then
  echo -e "  Using Bun $(bun --version)..."
  (cd "${SCRIPT_DIR}" && bun install --silent)
  echo -e "  ${GREEN}✔ Playwright and dependencies ready via Bun.${RESET}"
elif command -v npm >/dev/null 2>&1; then
  echo -e "  Using Node $(node --version) / npm..."
  (cd "${SCRIPT_DIR}" && npm install --silent)
  echo -e "  ${GREEN}✔ Playwright and dependencies ready via npm.${RESET}"
else
  echo -e "  ${YELLOW}⚠ Neither bun nor node was found. Browser extraction driver requires bun or node.${RESET}"
fi

# 4. Install REM Agent Skill to ~/.agents/skills/rem
echo -e "\n${BLUE}▶ Installing REM Agent Skill for AI coding agents...${RESET}"
SKILL_SOURCE="${SCRIPT_DIR}/skills/rem"
SKILL_TARGET="${HOME}/.agents/skills/rem"

if [ -d "${SKILL_SOURCE}" ]; then
  mkdir -p "${SKILL_TARGET}"
  cp -R "${SKILL_SOURCE}"/* "${SKILL_TARGET}/"
  echo -e "  ${GREEN}✔ Installed REM skill to:${RESET} ${SKILL_TARGET}"
fi

# Also ensure local .agents/skills/rem in workspace
mkdir -p "${SCRIPT_DIR}/.agents/skills/rem"
cp -R "${SKILL_SOURCE}"/* "${SCRIPT_DIR}/.agents/skills/rem/"

# 5. Configure MCP Server for installed AI clients
echo -e "\n${BLUE}▶ Configuring MCP server for AI clients...${RESET}"
"${INSTALL_DIR}/rem" setup --yes || true

# 6. Run System Diagnostics
echo -e "\n${BLUE}▶ Running REM diagnostics...${RESET}"
"${INSTALL_DIR}/rem" doctor

echo -e "\n${GREEN}${BOLD}==============================================================================${RESET}"
echo -e "${GREEN}${BOLD}✔ REM installation completed successfully!${RESET}"
echo -e "${GREEN}${BOLD}==============================================================================${RESET}\n"

echo -e "Quickstart Commands:"
echo -e "  ${BOLD}rem --help${RESET}                                  Show CLI commands"
echo -e "  ${BOLD}rem kb init \"Linear\" --dir re-linear${RESET}        Initialize a clean-room Knowledge Base"
echo -e "  ${BOLD}rem extract https://linear.app --dir re-linear${RESET} Automated DOM, token & screenshot extraction"
echo -e "  ${BOLD}rem kb check re-linear${RESET}                       Audit KB integrity & evidence tags"
echo -e "  ${BOLD}rem kb report re-linear --out REPORT.html${RESET}    Compile interactive standalone report"
echo -e "  ${BOLD}rem kb pack re-linear --out handoff.zip${RESET}      Package handoff for coding agents"
echo -e "  ${BOLD}rem ledger check${RESET}                             Verify .re-ledger/ DAG tasks"
echo -e "  ${BOLD}rem mcp${RESET}                                      Run stdio MCP server for AI agents\n"
