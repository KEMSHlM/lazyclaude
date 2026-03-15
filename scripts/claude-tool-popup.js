#!/usr/bin/env node
'use strict';

// claude-tool-popup.js - permission confirmation popup for Claude Code tool use
// Args: window
// Captures the current Claude pane content, shows it with y/a/n bar,
// and writes the choice to CHOICE_FILE for MCP server to send-keys.

const fs = require('fs');
const { spawnSync } = require('child_process');

const WINDOW     = process.argv[2] ?? '';
const safeWindow = WINDOW.replace(/[^a-zA-Z0-9_-]/g, '_');
const CHOICE_FILE = `/tmp/tmux-claude-tool-choice-${safeWindow}.txt`;

// ANSI helpers (matching claude-diff.js style)
const A      = (c) => `\x1b[${c}m`;
const R      = A(0);
const BOLD   = A(1);
const GREEN  = A('38;2;64;160;43');
const YELLOW = A('38;2;223;142;29');
const RED    = A('38;2;192;72;72');

function stripAnsi(s) { return s.replace(/\x1b\[[0-9;]*[mGKHJFABCDEF]/g, ''); }

const COLS = process.stdout.columns || Number(process.env.COLUMNS) || 80;
const ROWS = process.stdout.rows    || Number(process.env.LINES)   || 24;

// Capture visible pane content (with ANSI codes)
let paneLines = [];
try {
  const r = spawnSync('tmux', ['capture-pane', '-t', `claude:=${WINDOW}`, '-p', '-e'], { encoding: 'utf8' });
  paneLines = (r.stdout ?? '').split('\n');
} catch (e) {
  process.stderr.write(`[claude-tool-popup] capture-pane failed: ${e.message}\n`);
}

// Find the last separator line (long ─ line) = start of permission dialog
let dialogStart = -1;
for (let i = paneLines.length - 1; i >= 0; i--) {
  const plain = stripAnsi(paneLines[i]);
  if (plain.includes('─') && plain.replace(/[─\s]/g, '').length === 0 && plain.length > 20) {
    dialogStart = i;
    break;
  }
}

const rawDialogLines = dialogStart >= 0
  ? paneLines.slice(dialogStart)
  : paneLines.slice(-20);

// Remove captured y/a/n prompt lines (we render our own)
const contentLines = rawDialogLines.filter(l => {
  const plain = stripAnsi(l).trim();
  return !/^\[1\].*allow once|\[2\].*allow always|\[3\].*deny/i.test(plain);
});

// Render content in alternate screen
process.stdout.write('\x1b[?1049h\x1b[?25l');

const viewHeight = Math.max(1, ROWS - 4);
for (const l of contentLines.slice(0, viewHeight)) {
  process.stdout.write(l + '\x1b[K\n');
}
for (let i = contentLines.length; i < viewHeight; i++) {
  process.stdout.write('\x1b[K\n');
}

// y/a/n bar (matching claude-diff.js)
process.stdout.write('─'.repeat(COLS) + '\x1b[K\n');
process.stdout.write(`  ${GREEN}${BOLD}y${R}  Yes        ${YELLOW}${BOLD}a${R}  Allow all in session        ${RED}${BOLD}n${R}  No\x1b[K\n`);
process.stdout.write(`  ${BOLD}❯${R} \x1b[K`);

function cleanup() {
  process.stdout.write('\x1b[?25h\x1b[?1049l');
}

// Wait for keypress
if (!process.stdin.isTTY) {
  // Not interactive — default deny
  cleanup();
  try { fs.writeFileSync(CHOICE_FILE, '3'); } catch {}
  process.exit(0);
}

process.stdin.setRawMode(true);
process.stdin.resume();
process.stdin.once('data', (buf) => {
  const ch = buf.toString()[0];
  process.stdin.setRawMode(false);
  cleanup();
  const choice = (ch === 'y' || ch === 'Y' || ch === '1') ? '1'
               : (ch === 'a' || ch === 'A' || ch === '2') ? '2'
               : '3';
  try {
    fs.writeFileSync(CHOICE_FILE, choice);
  } catch (e) {
    process.stderr.write(`[claude-tool-popup] failed to write choice: ${e.message}\n`);
  }
  process.exit(0);
});