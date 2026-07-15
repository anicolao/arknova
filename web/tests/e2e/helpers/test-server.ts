import { expect } from '@playwright/test';
import { spawn, type ChildProcess } from 'node:child_process';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

export class TestServer {
  readonly origin = 'http://127.0.0.1:4173';
  private child: ChildProcess | undefined;
  private dataDir = '';

  async initialize() {
    this.dataDir = await mkdtemp(join(tmpdir(), 'arknova-e2e-'));
    await this.start();
  }

  async start() {
    this.child = spawn(resolve('.e2e/arknova'), [
      '-listen', new URL(this.origin).host,
      '-data', this.dataDir,
      '-web', resolve('build'),
      '-public-url', this.origin
    ], {
      env: {
        ...process.env,
        ARKNOVA_E2E: '1',
        ARKNOVA_ALLOW_TEST_CONTROLS: '1',
        ARKNOVA_FIXED_NOW_MS: '1710504000000',
        ARKNOVA_DETERMINISTIC_IDS: '1'
      },
      stdio: ['ignore', 'pipe', 'pipe']
    });
    this.child.stdout?.on('data', (chunk) => process.stdout.write(`[server] ${chunk}`));
    this.child.stderr?.on('data', (chunk) => process.stderr.write(`[server] ${chunk}`));
    await expect.poll(
      async () => fetch(`${this.origin}/healthz`).then((response) => response.ok).catch(() => false),
      { timeout: 2_000 }
    ).toBe(true);
  }

  async stop() {
    if (!this.child || this.child.exitCode !== null) return;
    this.child.kill('SIGTERM');
    await new Promise<void>((resolveExit) => this.child?.once('exit', () => resolveExit()));
  }

  async dispose() {
    await this.stop();
    if (this.dataDir) await rm(this.dataDir, { recursive: true, force: true });
  }
}
