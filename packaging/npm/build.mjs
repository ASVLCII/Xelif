import { chmodSync, copyFileSync, existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';

const source = dirname(fileURLToPath(import.meta.url));
const root = resolve(source, '../..');
const output = process.argv[2];
if (!output || process.argv.length !== 3) throw new Error('Usage: node packaging/npm/build.mjs <new-output-directory>');
const out = resolve(output);
if (existsSync(out)) throw new Error(`Refusing to overwrite an existing output directory: ${out}`);
const manifest = JSON.parse(readFileSync(join(source, 'package.json'), 'utf8'));
const platform = process.platform;
const arch = process.arch;
const name = `${manifest.name}-${platform}-${arch}`;
if (manifest.optionalDependencies[name] !== manifest.version) throw new Error(`Unsupported or inconsistent native target: ${name}`);
for (const version of Object.values(manifest.optionalDependencies)) {
  if (version !== manifest.version) throw new Error('All binary packages must use the exact launcher version');
}
const goConfig = readFileSync(join(root, 'src/config/fixed_variable.go'), 'utf8');
const goVersion = /CurrentVersion\s*=\s*"([^"]+)"/.exec(goConfig)?.[1];
const suffix = /PreReleaseSuffix\s*=\s*"([^"]*)"/.exec(goConfig)?.[1];
if (goVersion + suffix !== `v${manifest.version}`) throw new Error('Go and npm versions must match exactly');
const binaryDir = join(out, 'native');
const launcherDir = join(out, 'launcher');
mkdirSync(binaryDir, { recursive: true });
mkdirSync(join(launcherDir, 'bin'), { recursive: true });
const executable = join(binaryDir, platform === 'win32' ? 'xelif.exe' : 'xelif');
const build = spawnSync('go', ['build', '-trimpath', '-ldflags=-s -w', '-o', executable, '.'], {
  cwd: root,
  stdio: 'inherit',
  env: {
    ...process.env,
    GOOS: platform === 'win32' ? 'windows' : platform,
    GOARCH: arch === 'x64' ? 'amd64' : arch,
    CGO_ENABLED: platform === 'darwin' ? '1' : '0',
  },
});
if (build.error) throw build.error;
if (build.status !== 0) process.exit(build.status ?? 1);
chmodSync(executable, 0o755);
const nativeManifest = {
  name, version: manifest.version, description: `Xelif executable for ${platform}/${arch}`,
  license: manifest.license, repository: manifest.repository,
  os: [platform], cpu: [arch], files: [platform === 'win32' ? 'xelif.exe' : 'xelif', 'LICENSE', 'NOTICE.md', 'README.md'],
  publishConfig: manifest.publishConfig,
};
writeFileSync(join(binaryDir, 'package.json'), JSON.stringify(nativeManifest, null, 2) + '\n');
copyFileSync(join(source, 'package.json'), join(launcherDir, 'package.json'));
copyFileSync(join(source, 'bin/xelif.cjs'), join(launcherDir, 'bin/xelif.cjs'));
chmodSync(join(launcherDir, 'bin/xelif.cjs'), 0o755);
for (const dir of [binaryDir, launcherDir]) {
  for (const file of ['LICENSE', 'NOTICE.md']) copyFileSync(join(root, file), join(dir, file));
  copyFileSync(join(source, 'README.md'), join(dir, 'README.md'));
}
console.log(JSON.stringify({ version: manifest.version, target: `${platform}-${arch}`, native: binaryDir, launcher: launcherDir }));
