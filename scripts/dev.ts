import { mkdirSync, watch } from 'node:fs';

const repositoryRoot = new URL('..', import.meta.url).pathname;
const publicOrigin = process.env.ARKNOVA_DEV_ORIGIN ?? 'http://localhost:5173';
const backendOrigin = process.env.ARKNOVA_DEV_API_ORIGIN ?? 'http://127.0.0.1:8081';
const dataDirectory = process.env.ARKNOVA_DATA_DIR ?? `${repositoryRoot}data/manual`;

const commit = await commandOutput(['git', 'rev-parse', '--short', 'HEAD']);
const dirty = (await commandOutput(['git', 'status', '--porcelain'])) !== '';

console.log(`Manual testing revision: ${commit}${dirty ? ' (with uncommitted changes)' : ''}`);
console.log(`Table URL: ${publicOrigin}/table`);
console.log(`Go API: ${backendOrigin}`);
console.log(`Data: ${dataDirectory}`);
if (publicOrigin.includes('localhost') || publicOrigin.includes('127.0.0.1')) {
  console.log('Set ARKNOVA_DEV_ORIGIN to this Mac\'s LAN URL before scanning QR codes on another device.');
}

const environment = {
  ...process.env,
  ARKNOVA_DEV_API_ORIGIN: backendOrigin
};
mkdirSync(`${repositoryRoot}.dev`, { recursive: true });
let backend = await startBackend();
const frontend = Bun.spawn({
  cmd: ['bun', 'run', 'dev', '--', '--host', '0.0.0.0', '--port', '5173', '--strictPort'],
  cwd: `${repositoryRoot}web`,
  env: environment,
  stdin: 'inherit',
  stdout: 'inherit',
  stderr: 'inherit'
});
let restartTimer: ReturnType<typeof setTimeout> | undefined;
let restarting = false;
let restartAgain = false;
const watchers = ['cmd', 'internal'].map((directory) => watch(
  `${repositoryRoot}${directory}`,
  { recursive: true },
  (_event, filename) => {
    if (!filename?.endsWith('.go') || stopping) return;
    clearTimeout(restartTimer);
    restartTimer = setTimeout(restartBackend, 100);
  }
));

async function startBackend() {
  const build = Bun.spawn({
    cmd: ['go', 'build', '-o', '.dev/arknova', './cmd/arknova'],
    cwd: repositoryRoot,
    env: environment,
    stdout: 'inherit',
    stderr: 'inherit'
  });
  const buildStatus = await build.exited;
  if (buildStatus !== 0) throw new Error(`Go build failed with status ${buildStatus}`);
  return Bun.spawn({
    cmd: [
      `${repositoryRoot}.dev/arknova`,
      '-listen', new URL(backendOrigin).host,
      '-data', dataDirectory,
      '-web', `${repositoryRoot}web/build`,
      '-public-url', publicOrigin
    ],
    cwd: repositoryRoot,
    env: environment,
    stdin: 'inherit',
    stdout: 'inherit',
    stderr: 'inherit'
  });
}

async function restartBackend() {
  if (restarting) { restartAgain = true; return; }
  restarting = true;
  console.log('Go source changed; rebuilding and restarting the server…');
  backend.kill();
  await backend.exited;
  if (!stopping) backend = await startBackend();
  restarting = false;
  if (restartAgain) { restartAgain = false; await restartBackend(); }
}

let stopping = false;
let stopRequested = false;
const stop = () => {
  if (stopping) return;
  stopping = true;
  clearTimeout(restartTimer);
  for (const watcher of watchers) watcher.close();
  backend.kill();
  frontend.kill();
};
const requestStop = () => { stopRequested = true; stop(); };
process.on('SIGINT', requestStop);
process.on('SIGTERM', requestStop);

const result = await Promise.race([
  frontend.exited.then((code) => ({ name: 'Vite', code }))
]);
stop();
await Promise.all([backend.exited, frontend.exited]);
if (!stopRequested && result.code !== 0) console.error(`${result.name} exited with status ${result.code}`);
process.exit(stopRequested ? 0 : result.code);

async function commandOutput(command: string[]) {
  const process = Bun.spawn({ cmd: command, cwd: repositoryRoot, stdout: 'pipe', stderr: 'pipe' });
  const output = await new Response(process.stdout).text();
  const error = await new Response(process.stderr).text();
  const status = await process.exited;
  if (status !== 0) throw new Error(`${command.join(' ')} failed: ${error.trim()}`);
  return output.trim();
}
