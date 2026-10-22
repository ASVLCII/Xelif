'use strict';
const assert = require('node:assert/strict');
const { spawn, spawnSync } = require('node:child_process');
const { copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, realpathSync, rmSync, writeFileSync } = require('node:fs');
const { tmpdir } = require('node:os');
const { basename, join, resolve } = require('node:path');
const { after, before, test } = require('node:test');
const { once } = require('node:events');
const { resolveBinary } = require('../bin/xelif.cjs');
const manifest = require('../package.json');

const root = mkdtempSync(join(tmpdir(), 'xelif launcher space '));
const fixturePackage = join(root, 'node_modules', '@asvlcii', 'xelif');
const name = `${manifest.name}-${process.platform}-${process.arch}`;
const dependency = join(root, 'node_modules', ...name.split('/'));
const exe = process.platform === 'win32' ? 'xelif.exe' : 'xelif';
const launcher = join(fixturePackage, 'bin', 'xelif.cjs');

before(() => {
  mkdirSync(join(fixturePackage, 'bin'), { recursive: true });
  mkdirSync(dependency, { recursive: true });
  copyFileSync(resolve(__dirname, '../bin/xelif.cjs'), launcher);
  copyFileSync(resolve(__dirname, '../package.json'), join(fixturePackage, 'package.json'));
  writeFileSync(join(dependency, 'package.json'), JSON.stringify({ name, version: manifest.version }));
  const result = spawnSync('go', ['build', '-o', join(dependency, exe), join(__dirname, 'fixture.go')], {
    encoding: 'utf8', env: { ...process.env, CGO_ENABLED: '0' },
  });
  assert.equal(result.status, 0, result.stderr || result.error?.message);
});
after(() => rmSync(root, { recursive: true, force: true }));

test('all declared platforms select their exact package and executable', () => {
  for (const key of Object.keys(manifest.optionalDependencies)) {
    const [platform, arch] = key.slice(manifest.name.length + 1).split('-');
    const path = join(root, `${platform}-${arch}.json`);
    writeFileSync(path, JSON.stringify({ name: key, version: manifest.version }));
    const selected = resolveBinary(platform, arch, (id) => {
      assert.equal(id, `${key}/package.json`);
      return path;
    });
    assert.equal(basename(selected), platform === 'win32' ? 'xelif.exe' : 'xelif');
  }
});
test('unsupported targets never attempt resolution or download', () => {
  assert.throws(() => resolveBinary('freebsd', 'x64', () => assert.fail('must not resolve')), /Unsupported platform/);
});
test('missing optional dependency has an actionable, version-pinned error', () => {
  assert.throws(() => resolveBinary('win32', 'x64', () => {
    throw Object.assign(new Error('missing'), { code: 'MODULE_NOT_FOUND' });
  }), new RegExp(`--include=optional`));
});
test('unexpected resolver errors are not disguised as missing dependencies', () => {
  const error = Object.assign(new Error('denied'), { code: 'EACCES' });
  assert.throws(() => resolveBinary('win32', 'x64', () => { throw error; }), (actual) => actual === error);
});
test('mismatched binary versions fail closed', () => {
  const path = join(root, 'stale.json');
  writeFileSync(path, JSON.stringify({ name, version: '0.0.0' }));
  assert.throws(() => resolveBinary(process.platform, process.arch, () => path), /Binary package mismatch/);
});
test('stdio, arguments, cwd, environment and exit status are inherited without a shell', async () => {
  const args = ['space in path', '夜', 'literal&|;$HOME', '"quoted"', '--flag=value'];
  const child = spawn(process.execPath, [launcher, ...args], {
    cwd: root, env: { ...process.env, XELIF_LAUNCHER_TEST: 'sentinel' },
  });
  let stdout = '', stderr = '';
  child.stdout.on('data', (data) => { stdout += data; });
  child.stderr.on('data', (data) => { stderr += data; });
  child.stdin.end('stdin bytes\n');
  const [code, signal] = await once(child, 'close');
  assert.equal(code, 37);
  assert.equal(signal, null);
  const { cwd, ...result } = JSON.parse(stdout);
  assert.deepEqual(result, { args, input: 'stdin bytes\n', env: 'sentinel' });
  assert.equal(realpathSync(cwd), realpathSync(root));
  assert.equal(stderr, 'fixture stderr');
});
test('SIGTERM reaches the foreground app and is reflected by the launcher', { skip: process.platform === 'win32' }, async () => {
  const child = spawn(process.execPath, [launcher, 'wait']);
  try {
    await once(child.stdout, 'data');
    child.kill('SIGTERM');
    const [code, signal] = await once(child, 'close');
    assert.equal(code, null);
    assert.equal(signal, 'SIGTERM');
  } finally {
    if (child.exitCode === null && child.signalCode === null) child.kill('SIGKILL');
  }
});
test('the published launcher has no machine-local paths or install hooks', () => {
  assert.equal(manifest.scripts, undefined);
  const source = readFileSync(resolve(__dirname, '../bin/xelif.cjs'), 'utf8');
  assert.doesNotMatch(source, /X:\\|powershell|pwsh|https?:|fetch\(/i);
  assert.ok(existsSync(launcher));
});
