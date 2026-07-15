import { expect, test, type BrowserContext, type Page } from '@playwright/test';
import { spawn, type ChildProcess } from 'node:child_process';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

let serverProcess: ChildProcess;
let dataDir: string;
let origin: string;

async function startServer() {
  serverProcess = spawn(resolve('.e2e/arknova'), ['-listen', new URL(origin).host, '-data', dataDir, '-web', resolve('build'), '-public-url', origin], {
    env: { ...process.env, ARKNOVA_E2E: '1', ARKNOVA_ALLOW_TEST_CONTROLS: '1', ARKNOVA_FIXED_NOW_MS: '1710504000000', ARKNOVA_DETERMINISTIC_IDS: '1' },
    stdio: ['ignore', 'pipe', 'pipe']
  });
  serverProcess.stdout?.on('data', (chunk) => process.stdout.write(`[server] ${chunk}`));
  serverProcess.stderr?.on('data', (chunk) => process.stderr.write(`[server] ${chunk}`));
  await expect.poll(async () => fetch(`${origin}/healthz`).then((response) => response.ok).catch(() => false), { timeout: 2_000 }).toBe(true);
}

async function stopServer() {
  if (!serverProcess || serverProcess.exitCode !== null) return;
  serverProcess.kill('SIGTERM');
  await new Promise<void>((resolveExit) => serverProcess.once('exit', () => resolveExit()));
}

test.beforeAll(async () => {
  dataDir = await mkdtemp(join(tmpdir(), 'arknova-e2e-'));
  origin = 'http://127.0.0.1:4173';
  await startServer();
});

test.afterAll(async () => { await stopServer(); await rm(dataDir, { recursive: true, force: true }); });

test('host creates a game and every device recovers after restart', async ({ browser }) => {
  const tableContext = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const seat1Context = await browser.newContext({ viewport: { width: 390, height: 844 } });
  const seat2Context = await browser.newContext({ viewport: { width: 390, height: 844 } });
  const table = await tableContext.newPage();

  await test.step('The host creates a two-player game', async () => {
    await table.goto(`${origin}/table`);
    await table.getByRole('button', { name: 'Start two-player game' }).click();
    await expect(table.getByTestId('game-code')).toHaveText('WILD');
    await expect(table.getByTestId('projection-revision')).toHaveText('Revision 1');
    await expect(table.getByTestId('seat-1-qr').getByRole('img')).toBeVisible();
    await expect(table).toHaveScreenshot('000-game-created--table.png');
  });

  const actions = await fetch(`${origin}/api/games/WILD/actions`).then((response) => response.json());
  expect(actions).toHaveLength(1);
  expect(actions[0]).toMatchObject({ type: 'GameConfigured', payload: { playerCount: 2 } });

  const seat1 = await openCompanion(seat1Context, 1);
  const seat2 = await openCompanion(seat2Context, 2);
  await expect(seat1).toHaveScreenshot('001-connected--seat-1.png');
  await expect(seat2).toHaveScreenshot('001-connected--seat-2.png');

  await test.step('All devices recover from the canonical action log after server restart', async () => {
    await stopServer();
    await startServer();
    await Promise.all([table.reload(), seat1.reload(), seat2.reload()]);
    await expect(table.getByTestId('game-code')).toHaveText('WILD');
    await expect(table.getByTestId('projection-revision')).toHaveText('Revision 1');
    await expect(seat1.getByRole('heading', { name: 'Player 1' })).toBeVisible();
    await expect(seat2.getByRole('heading', { name: 'Player 2' })).toBeVisible();
    await expect(seat1.getByTestId('projection-revision')).toHaveText('Revision 1');
    await expect(seat2.getByTestId('projection-revision')).toHaveText('Revision 1');
  });
});

async function openCompanion(context: BrowserContext, player: number): Promise<Page> {
  const page = await context.newPage();
  await page.goto(`${origin}/play?gameid=WILD&player=${player}`);
  await expect(page.getByRole('heading', { name: `Player ${player}` })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Your hand is empty' })).toBeVisible();
  await expect(page.getByTestId('projection-revision')).toHaveText('Revision 1');
  return page;
}
