export type Card = { id: string; name: string; image: string };
export type PlayerState = { seat: number; actionCards: string[]; xTokens: number };
export type HistoryEntry = { revision: number; seat?: number; summary: string };
export type Projection = {
  gameId: string;
  playerCount: number;
  player?: number;
  revision: number;
  activePlayer?: number;
  display?: Card[];
  players?: PlayerState[];
  hand?: Card[];
  history?: HistoryEntry[];
};
export type CreatedGame = { projection: Projection; companionUrls: string[] };

function randomHex(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('');
}

function newClientActionId(): string {
  return `browser-${randomHex()}`;
}

export async function createGame(playerCount: number): Promise<CreatedGame> {
  const response = await fetch('/api/games', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ playerCount, clientActionId: newClientActionId(), seed: randomHex() })
  });
  if (!response.ok) throw new Error(await response.text());
  return response.json();
}

export async function takeXToken(gameId: string, player: number, actionCard: string, expectedRevision: number): Promise<void> {
  const response = await fetch(`/api/games/${encodeURIComponent(gameId)}/actions`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({
      player,
      type: 'XTokenTaken',
      schemaVersion: 1,
      expectedRevision,
      clientActionId: newClientActionId(),
      payload: { actionCard }
    })
  });
  if (!response.ok) throw new Error(await response.text());
}

export async function getProjection(gameId: string, player = 0): Promise<Projection> {
  const response = await fetch(`/api/games/${encodeURIComponent(gameId)}/projection?player=${player}`);
  if (!response.ok) throw new Error(await response.text());
  return response.json();
}

export function watchProjection(gameId: string, player: number, update: (value: Projection) => void): () => void {
  const scheme = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const socket = new WebSocket(`${scheme}//${location.host}/ws?gameid=${encodeURIComponent(gameId)}&player=${player}`);
  socket.addEventListener('message', (event) => update(JSON.parse(event.data)));
  return () => socket.close();
}

export function contentUrl(path: string): string {
  return `/content/${path.split('/').map(encodeURIComponent).join('/')}`;
}
