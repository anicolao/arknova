import { expect, test, type BrowserContext, type Page } from '@playwright/test';
import { MultiSurfaceStepHelper, type Surface } from '../helpers/multi-surface-step-helper';
import { TestServer } from '../helpers/test-server';

const server = new TestServer();

test.beforeAll(async () => server.initialize());
test.afterAll(async () => server.dispose());

test('host creates a game and every device recovers after restart', async ({ browser }, testInfo) => {
  const walkthrough = new MultiSurfaceStepHelper(testInfo);
  walkthrough.setMetadata(
    'Create and Connect',
    'A host creates a two-player game on the shared table, both players join on private devices, and every surface recovers from the canonical action log after the server restarts.'
  );

  const tableContext = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const seat1Context = await browser.newContext({ viewport: { width: 390, height: 844 } });
  const seat2Context = await browser.newContext({ viewport: { width: 390, height: 844 } });

  try {
    await tableContext.addInitScript(() => {
      Object.defineProperty(crypto, 'randomUUID', { value: undefined });
    });
    const table = await tableContext.newPage();
    await table.goto(`${server.origin}/table`);
    await walkthrough.step('table_ready', {
      description: 'The host opens the shared table and can start a two-player game.',
      surfaces: [{
        id: 'table', name: 'Shared table', page: table,
        verifications: [
          { spec: 'Ark Nova table heading is visible', check: async () => expect(table.getByRole('heading', { name: 'Ark Nova' })).toBeVisible() },
          { spec: 'Two-player game can be started', check: async () => expect(table.getByRole('button', { name: 'Start two-player game' })).toBeEnabled() }
        ]
      }]
    });

    await table.getByRole('button', { name: 'Start two-player game' }).click();
    await walkthrough.step('game_created', {
      description: 'Creating the game durably configures two seats and displays their stable QR URLs.',
      surfaces: [{
        id: 'table', name: 'Shared table', page: table,
        verifications: [
          { spec: 'Deterministic game code WILD is visible', check: async () => expect(table.getByTestId('game-code')).toHaveText('WILD') },
          { spec: 'Projection revision is 1', check: async () => expect(table.getByTestId('projection-revision')).toHaveText('Revision 1') },
          { spec: 'Player 1 QR code is visible', check: async () => expect(table.getByTestId('seat-1-qr').getByRole('img')).toBeVisible() },
          { spec: 'Player 2 QR code is visible', check: async () => expect(table.getByTestId('seat-2-qr').getByRole('img')).toBeVisible() },
          { spec: 'Exactly one GameConfigured action exists in the canonical log', check: verifyCanonicalAction }
        ]
      }]
    });

    const seat1 = await openCompanion(seat1Context, 1);
    const seat2 = await openCompanion(seat2Context, 2);
    await walkthrough.step('players_connected', {
      description: 'Both players join from isolated private devices while the table retains the public view.',
      surfaces: [tableSurface(table), companionSurface(seat1, 1), companionSurface(seat2, 2)]
    });

    await server.stop();
    await server.start();
    await Promise.all([table.reload(), seat1.reload(), seat2.reload()]);
    await walkthrough.step('server_restarted', {
      description: 'After a real server restart, all devices recover the same game and revision without credentials or reconfiguration.',
      surfaces: [tableSurface(table), companionSurface(seat1, 1), companionSurface(seat2, 2)]
    });

    walkthrough.generateDocs();
  } finally {
    await Promise.all([tableContext.close(), seat1Context.close(), seat2Context.close()]);
  }
});

async function openCompanion(context: BrowserContext, player: number): Promise<Page> {
  const page = await context.newPage();
  await page.goto(`${server.origin}/play?gameid=WILD&player=${player}`);
  return page;
}

function tableSurface(page: Page): Surface {
  return {
    id: 'table', name: 'Shared table', page,
    verifications: [
      { spec: 'Game code remains WILD', check: async () => expect(page.getByTestId('game-code')).toHaveText('WILD') },
      { spec: 'Public projection remains at revision 1', check: async () => expect(page.getByTestId('projection-revision')).toHaveText('Revision 1') },
      { spec: 'Both companion QR codes remain visible', check: async () => expect(page.locator('[data-testid$="-qr"] img')).toHaveCount(2) }
    ]
  };
}

function companionSurface(page: Page, player: number): Surface {
  return {
    id: `player-${player}`, name: `Player ${player} companion`, page,
    verifications: [
      { spec: `Device is assigned to player ${player}`, check: async () => expect(page.getByRole('heading', { name: `Player ${player}` })).toBeVisible() },
      { spec: 'Private hand is empty', check: async () => expect(page.getByRole('heading', { name: 'Your hand is empty' })).toBeVisible() },
      { spec: 'Private projection is at revision 1', check: async () => expect(page.getByTestId('projection-revision')).toHaveText('Revision 1') }
    ]
  };
}

async function verifyCanonicalAction() {
  const response = await fetch(`${server.origin}/api/games/WILD/actions`);
  expect(response.ok).toBe(true);
  const actions = await response.json();
  expect(actions).toHaveLength(1);
  expect(actions[0]).toMatchObject({ type: 'GameConfigured', payload: { playerCount: 2 } });
}
