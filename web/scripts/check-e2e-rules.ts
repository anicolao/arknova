import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, relative } from 'node:path';

const root = join(import.meta.dir, '..');
const e2eRoot = join(root, 'tests', 'e2e');
const failures: string[] = [];

for (const path of files(e2eRoot)) {
  if (!path.endsWith('.ts')) continue;
  const source = readFileSync(path, 'utf8');
  const name = relative(root, path);
  const isHelper = path.includes(`${join('e2e', 'helpers')}`);

  check(name, source, /waitForTimeout\s*\(/, 'uses a time-based wait');
  check(name, source, /timeout\s*:\s*(?:[3-9]\d{3}|\d{5,})/, 'sets a wait or assertion timeout above 2000 ms');
  check(name, source, /\b(?:test|describe)\.(?:only|skip)\s*\(/, 'commits an exclusive or skipped test');

  if (!isHelper) {
    check(name, source, /\.toHaveScreenshot\s*\(/, 'calls toHaveScreenshot outside the step helper');
    check(name, source, /\btest\.step\s*\(/, 'uses test.step instead of the documented step helper');
    check(name, source, /['"]\d{3}[-_][^'"]+\.png['"]/, 'manually names a numbered screenshot');
    if (path.endsWith('.spec.ts')) {
      requireText(name, source, 'MultiSurfaceStepHelper', 'does not use the multi-surface step helper');
      requireText(name, source, '.generateDocs()', 'does not generate its scenario README');
    }
  }
}

if (failures.length) {
  console.error('E2E policy violations:\n' + failures.map((failure) => `- ${failure}`).join('\n'));
  process.exit(1);
}

console.log('E2E policy checks passed');

function check(name: string, source: string, pattern: RegExp, message: string) {
  if (pattern.test(source)) failures.push(`${name}: ${message}`);
}

function requireText(name: string, source: string, expected: string, message: string) {
  if (!source.includes(expected)) failures.push(`${name}: ${message}`);
}

function files(directory: string): string[] {
  return readdirSync(directory).flatMap((entry) => {
    const path = join(directory, entry);
    return statSync(path).isDirectory() ? files(path) : [path];
  });
}
