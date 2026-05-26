#!/usr/bin/env node
// Thin shim: resolves the platform-specific inference-gateway binary
// (downloading it from GitHub releases on first run) and execs it with
// the caller's arguments and stdio.

'use strict';

const { spawn } = require('child_process');
const { resolveBinary } = require('../lib/install');

(async () => {
  try {
    const binaryPath = await resolveBinary();
    const child = spawn(binaryPath, process.argv.slice(2), { stdio: 'inherit' });

    const forward = (signal) => () => {
      if (!child.killed) {
        child.kill(signal);
      }
    };
    process.on('SIGINT', forward('SIGINT'));
    process.on('SIGTERM', forward('SIGTERM'));

    child.on('exit', (code, signal) => {
      if (signal) {
        process.kill(process.pid, signal);
        return;
      }
      process.exit(code ?? 0);
    });
    child.on('error', (err) => {
      console.error(`[inference-gateway] Failed to launch binary: ${err.message}`);
      process.exit(1);
    });
  } catch (err) {
    console.error(`[inference-gateway] ${err.message || err}`);
    process.exit(1);
  }
})();
