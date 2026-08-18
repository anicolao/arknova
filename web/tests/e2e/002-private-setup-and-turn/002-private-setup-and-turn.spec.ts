import { expect, test, type BrowserContext, type Page } from '@playwright/test';
import { MultiSurfaceStepHelper, type Surface } from '../helpers/multi-surface-step-helper';
import { TestServer } from '../helpers/test-server';

const server = new TestServer();

test.beforeAll(async () => server.initialize());
test.afterAll(async () => server.dispose());

test('private setup and an X-token turn replay from the action log', async ({ browser }, testInfo) => {
  const walkthrough = new MultiSurfaceStepHelper(testInfo);
  walkthrough.setMetadata(
    'Private Setup and Turn',
    'A deterministic synthetic deck creates a public display and private hands, then player 1 takes an X-token with the strength-3 action card and the next turn survives a projection rebuild.'
  );

  const tableContext = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const seat1Context = await browser.newContext({ viewport: { width: 390, height: 844 } });
  const seat2Context = await browser.newContext({ viewport: { width: 390, height: 844 } });

  try {
    const table = await tableContext.newPage();
    await table.goto(`${server.origin}/table`);
    await table.getByRole('button', { name: 'Start two-player game' }).click();
    await expect(table.getByTestId('game-code')).toHaveText('WILD');

    const seat1 = await openCompanion(seat1Context, 1);
    const seat2 = await openCompanion(seat2Context, 2);
    const seat1Cards = await handNames(seat1);
    const seat2Cards = await handNames(seat2);

    await walkthrough.step('private_setup', {
      description: 'The public display and both private starting hands are dealt without leaking either hand to another surface.',
      surfaces: [
        setupTableSurface(table, [...seat1Cards, ...seat2Cards]),
        setupCompanionSurface(seat1, 1, seat2Cards),
        setupCompanionSurface(seat2, 2, seat1Cards)
      ]
    });

    await table.getByTestId('action-card-player-1-animals').click();
    await walkthrough.step('x_token_turn', {
      description: 'Player 1 takes an X-token with Animals at strength 3; Animals moves to slot 1 and player 2 becomes active everywhere.',
      surfaces: [turnTableSurface(table), turnCompanionSurface(seat1, 1), turnCompanionSurface(seat2, 2)]
    });
    await verifyCanonicalActions();

    await server.stop();
    await server.deleteProjectionStore();
    await server.start();
    await Promise.all([table.reload(), seat1.reload(), seat2.reload()]);
    await walkthrough.step('projection_rebuilt', {
      description: 'After deleting SQLite, replaying the two canonical actions restores the same deal, X-token, card order, and active player.',
      surfaces: [turnTableSurface(table), turnCompanionSurface(seat1, 1), turnCompanionSurface(seat2, 2)]
    });

    walkthrough.generateDocs();
  } finally {
    await Promise.all([tableContext.close(), seat1Context.close(), seat2Context.close()]);
  }
});

async function openCompanion(context: BrowserContext, player: number): Promise<Page> {
  const page = await context.newPage();
  await page.goto(`${server.origin}/play?gameid=WILD&player=${player}`);
  await expect(page.getByTestId('hand-card')).toHaveCount(3);
  return page;
}

async function handNames(page: Page): Promise<string[]> {
  return page.getByTestId('hand-card').locator('strong').allTextContents();
}

function setupTableSurface(page: Page, privateCardNames: string[]): Surface {
  return {
    id: 'table', name: 'Shared table', page,
    verifications: [
      { spec: 'Six public display cards are visible', check: async () => expect(page.getByTestId('display-card')).toHaveCount(6) },
      { spec: 'Player 1 is active', check: async () => expect(page.getByTestId('active-player')).toHaveText('Player 1’s turn') },
      { spec: 'Animals is available at strength 3', check: async () => expect(page.getByTestId('action-card-player-1-animals')).toBeEnabled() },
      {
        spec: 'Neither private hand appears on the table',
        check: async () => {
          for (const name of privateCardNames) await expect(page.getByText(name, { exact: true })).toHaveCount(0);
        }
      }
    ]
  };
}

function setupCompanionSurface(page: Page, player: number, otherHand: string[]): Surface {
  return {
    id: `player-${player}`, name: `Player ${player} companion`, page,
    verifications: [
      { spec: 'Exactly three private cards are visible', check: async () => expect(page.getByTestId('hand-card')).toHaveCount(3) },
      {
        spec: `Player ${player} cannot see the other player's hand`,
        check: async () => {
          for (const name of otherHand) await expect(page.getByText(name, { exact: true })).toHaveCount(0);
        }
      },
      { spec: 'Player 1 is identified as active', check: async () => expect(page.getByTestId('active-player')).toHaveText(player === 1 ? 'Your turn' : 'Player 1’s turn') },
      { spec: 'Projection is at revision 1', check: async () => expect(page.getByTestId('projection-revision')).toHaveText('Revision 1') }
    ]
  };
}

function turnTableSurface(page: Page): Surface {
  return {
    id: 'table', name: 'Shared table', page,
    verifications: [
      { spec: 'Player 2 is active', check: async () => expect(page.getByTestId('active-player')).toHaveText('Player 2’s turn') },
      { spec: 'Player 1 has one X-token', check: async () => expect(page.getByTestId('player-1-actions')).toContainText('1 X-token') },
      { spec: 'Animals moved to strength 1', check: async () => expect(page.getByTestId('player-1-actions').getByRole('button').first()).toHaveAccessibleName('Animals at strength 1; take an X-token') },
      { spec: 'Public history explains the accepted action', check: async () => expect(page.getByText('Player 1 took an X-token with Animals at strength 3')).toBeVisible() },
      { spec: 'Projection is at revision 2', check: async () => expect(page.getByTestId('projection-revision')).toHaveText('Revision 2') }
    ]
  };
}

function turnCompanionSurface(page: Page, player: number): Surface {
  return {
    id: `player-${player}`, name: `Player ${player} companion`, page,
    verifications: [
      { spec: 'Private hand is unchanged', check: async () => expect(page.getByTestId('hand-card')).toHaveCount(3) },
      { spec: 'Player 2 is identified as active', check: async () => expect(page.getByTestId('active-player')).toHaveText(player === 2 ? 'Your turn' : 'Player 2’s turn') },
      { spec: 'Projection is at revision 2', check: async () => expect(page.getByTestId('projection-revision')).toHaveText('Revision 2') },
      ...(player === 1 ? [{ spec: 'Player 1 sees the gained X-token', check: async () => expect(page.getByTestId('x-token-count')).toHaveText('1 X-token') }] : [])
    ]
  };
}

async function verifyCanonicalActions() {
  const response = await fetch(`${server.origin}/api/games/WILD/actions`);
  expect(response.ok).toBe(true);
  const actions = await response.json();
  expect(actions).toHaveLength(2);
  expect(actions[0]).toMatchObject({ type: 'GameConfigured', payload: { playerCount: 2, contentVersion: 'synthetic-v1' } });
  expect(actions[1]).toMatchObject({
    type: 'XTokenTaken', actor: { kind: 'player', seat: 1 }, expectedRevision: 1,
    payload: { actionCard: 'Animals' }
  });
}
