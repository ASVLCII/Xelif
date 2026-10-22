#!/usr/bin/env node
'use strict';

const { spawn } = require('node:child_process');
const { readFileSync } = require('node:fs');
const { dirname, join } = require('node:path');
const manifest = require('../package.json');

function resolveBinary(platform = process.platform, arch = process.arch, resolve = require.resolve) {
  const name = `${manifest.name}-${platform}-${arch}`;
  if (!Object.hasOwn(manifest.optionalDependencies, name)) {
    throw new Error(`Unsupported platform: ${platform}/${arch}. This preview supports Windows, macOS and Linux on x64/arm64.`);
  }
  let path;
  try {
    path = resolve(`${name}/package.json`);
  } catch (error) {
    if (error.code !== 'MODULE_NOT_FOUND') throw error;
    throw new Error(`Missing ${name}@${manifest.version}. Reinstall with optional dependencies enabled: npm install -g ${manifest.name}@${manifest.version} --include=optional`, { cause: error });
  }
  const binaryManifest = JSON.parse(readFileSync(path, 'utf8'));
  if (binaryManifest.name !== name || binaryManifest.version !== manifest.version) {
    throw new Error(`Binary package mismatch: expected ${name}@${manifest.version}. Reinstall Xelif with optional dependencies enabled.`);
  }
  return join(dirname(path), platform === 'win32' ? 'xelif.exe' : 'xelif');
}

function main() {
  let binary;
  try {
    binary = resolveBinary();
  } catch (error) {
    console.error(`Xelif: ${error.message}`);
    process.exitCode = 1;
    return;
  }
  const child = spawn(binary, process.argv.slice(2), { stdio: 'inherit', shell: false });
  const forward = (signal) => { child.kill(signal); };
  const interrupt = () => forward('SIGINT');
  const terminate = () => forward('SIGTERM');
  process.on('SIGINT', interrupt);
  process.on('SIGTERM', terminate);
  const cleanup = () => {
    process.removeListener('SIGINT', interrupt);
    process.removeListener('SIGTERM', terminate);
  };
  child.once('error', (error) => {
    cleanup();
    console.error(`Xelif: could not start the bundled executable (${error.code || error.message}). Reinstall ${manifest.name}@${manifest.version}.`);
    process.exitCode = 1;
  });
  child.once('exit', (code, signal) => {
    cleanup();
    if (signal) process.kill(process.pid, signal);
    else process.exitCode = code ?? 1;
  });
}

module.exports = { resolveBinary };
if (require.main === module) main();
