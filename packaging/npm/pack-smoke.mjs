import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { createHash } from 'node:crypto';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { createRequire } from 'node:module';
import { existsSync, mkdirSync, readFileSync, statSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const [stageArg, workArg] = process.argv.slice(2);
if (!stageArg || !workArg) throw new Error('Usage: node packaging/npm/pack-smoke.mjs <build-output> <new-test-directory>');
const stage = resolve(stageArg), work = resolve(workArg);
if (existsSync(work)) throw new Error(`Refusing to overwrite ${work}`);
mkdirSync(work, { recursive: true });
const manifest = JSON.parse(readFileSync(join(stage, 'launcher/package.json'), 'utf8'));
const native = JSON.parse(readFileSync(join(stage, 'native/package.json'), 'utf8'));
assert.equal(native.name, `${manifest.name}-${process.platform}-${process.arch}`);
const exe = process.platform === 'win32' ? 'xelif.exe' : 'xelif';
const npmCLI = [
  process.env.npm_execpath,
  join(dirname(process.execPath), 'node_modules/npm/bin/npm-cli.js'),
  resolve(dirname(process.execPath), '../lib/node_modules/npm/bin/npm-cli.js'),
].find((path) => path && existsSync(path));
if (!npmCLI) throw new Error('Cannot locate npm-cli.js next to Node.js');
const home = join(work, 'home');
mkdirSync(home);
const userConfig = join(work, 'npm-user-config');
const globalConfig = join(work, 'npm-global-config');
writeFileSync(userConfig, '');
writeFileSync(globalConfig, '');
const env = {
  ...process.env,
  HOME: home, USERPROFILE: home,
  npm_config_userconfig: userConfig, npm_config_globalconfig: globalConfig,
  npm_config_cache: join(work, 'npm-cache'), npm_config_audit: 'false', npm_config_fund: 'false',
  XDG_CONFIG_HOME: join(work, 'config'), XDG_DATA_HOME: join(work, 'data'),
  XDG_STATE_HOME: join(work, 'state'), XDG_CACHE_HOME: join(work, 'cache'),
  NO_COLOR: '1',
};
async function run(command, args, options = {}) {
  const child = spawn(command, args, { cwd: work, env, ...options });
  let stdout = '', stderr = '';
  child.stdout.on('data', (data) => { stdout += data; });
  child.stderr.on('data', (data) => { stderr += data; });
  const done = once(child, 'close');
  child.stdin.end();
  const timer = setTimeout(() => child.kill(), 180_000);
  try {
    const [code, signal] = await done;
    assert.equal(code, 0, `${command} ${args.join(' ')}\n${signal || ''}\n${stdout}\n${stderr}`);
    return stdout;
  } finally { clearTimeout(timer); }
}
const npm = (args) => run(process.execPath, [npmCLI, ...args]);
const packs = join(work, 'packs');
mkdirSync(packs);
const packed = {};
for (const kind of ['native', 'launcher']) {
  const result = JSON.parse(await npm(['pack', join(stage, kind), '--ignore-scripts', '--json', '--pack-destination', packs]))[0];
  const allowed = ['package.json', 'README.md', 'LICENSE', 'NOTICE.md', kind === 'native' ? exe : 'bin/xelif.cjs'].sort();
  assert.deepEqual(result.files.map((file) => file.path).sort(), allowed, 'unexpected packed content');
  packed[kind] = join(packs, result.filename);
}
const nativeTar = readFileSync(packed.native);
const requests = [], unexpected = [];
const server = createServer((req, res) => {
  const path = decodeURIComponent(req.url.split('?')[0]);
  requests.push(path);
  if (path === '/native.tgz') {
    res.setHeader('Content-Type', 'application/octet-stream');
    res.end(nativeTar);
    return;
  }
  const name = path.slice(1);
  if (Object.hasOwn(manifest.optionalDependencies, name)) {
    const [os, cpu] = name.slice(manifest.name.length + 1).split('-');
    const version = {
      name, version: manifest.version, os: [os], cpu: [cpu],
      dist: { tarball: `${registry}/${name === native.name ? 'native' : 'foreign'}.tgz`,
        integrity: `sha512-${createHash('sha512').update(nativeTar).digest('base64')}` },
    };
    res.setHeader('Content-Type', 'application/json');
    res.end(JSON.stringify({ name, 'dist-tags': { latest: manifest.version, preview: manifest.version }, versions: { [manifest.version]: version } }));
    return;
  }
  unexpected.push(path);
  res.writeHead(404).end();
});
server.listen(0, '127.0.0.1');
await once(server, 'listening');
const registry = `http://127.0.0.1:${server.address().port}`;
env.npm_config_registry = registry;
let expected;
async function checkInstalled(packageRoot, runtime = process.execPath) {
  const launcher = join(packageRoot, 'bin/xelif.cjs');
  assert.equal(await run(runtime, [launcher, '--version']), expected);
  const selected = createRequire(import.meta.url)(launcher).resolveBinary();
  assert.ok(statSync(selected).isFile());
  if (process.platform !== 'win32') assert.ok(statSync(selected).mode & 0o111, 'native executable mode lost');
}
try {
  expected = await run(join(stage, 'native', exe), ['--version']);
  assert.ok(expected.includes(manifest.version), expected);
  const prefix = join(work, 'npm prefix');
  await npm(['install', '--global', '--prefix', prefix, '--ignore-scripts', '--registry', registry, packed.launcher]);
  const moduleRoot = join(prefix, process.platform === 'win32' ? '' : 'lib', 'node_modules');
  const installed = join(moduleRoot, '@asvlcii', 'xelif');
  await checkInstalled(installed);
  const shim = join(prefix, process.platform === 'win32' ? 'xelif.cmd' : 'bin/xelif');
  const shimOutput = process.platform === 'win32'
    ? await run(process.env.ComSpec || 'cmd.exe', ['/d', '/s', '/c', `""${shim}" --version"`], { windowsVerbatimArguments: true })
    : await run(shim, ['--version']);
  assert.equal(shimOutput, expected, 'npm global shim failed');
  const bunEnv = {
    ...env, BUN_INSTALL_GLOBAL_DIR: join(work, 'bun-global'), BUN_INSTALL_BIN: join(work, 'bun-bin'),
    BUN_INSTALL_CACHE_DIR: join(work, 'bun-cache'),
  };
  await run('bun', ['add', '--global', '--ignore-scripts', '--no-progress', '--registry', registry, packed.launcher], { env: bunEnv });
  const bunInstalled = join(bunEnv.BUN_INSTALL_GLOBAL_DIR, 'node_modules', '@asvlcii', 'xelif');
  await checkInstalled(bunInstalled);
  await checkInstalled(bunInstalled, 'bun');
  const bunShim = join(bunEnv.BUN_INSTALL_BIN, process.platform === 'win32' ? 'xelif.exe' : 'xelif');
  assert.equal(await run(bunShim, ['--version'], { env: bunEnv }), expected, 'Bun global shim failed');
  assert.deepEqual(unexpected, [], 'an unexpected package or foreign-platform tarball was requested');
  assert.ok(requests.includes('/native.tgz'));
  writeFileSync(join(work, 'results.json'), JSON.stringify({ target: `${process.platform}-${process.arch}`, version: manifest.version,
    npmGlobalShim: true, bunGlobalShim: true, npmInstall: true, bunInstall: true, bunRuntime: true, packed, requests }, null, 2) + '\n');
  console.log(`Package smoke passed: ${process.platform}/${process.arch} ${manifest.version}`);
} finally {
  server.closeAllConnections();
  server.close();
}
