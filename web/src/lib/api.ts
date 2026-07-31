export type Projection = { gameId: string; playerCount: number; player?: number; revision: number };
export type CreatedGame = { projection: Projection; companionUrls: string[] };

function newClientActionId(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  return `browser-${Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')}`;
}

export async function createGame(playerCount: number): Promise<CreatedGame> {
  const response = await fetch('/api/games', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ playerCount, clientActionId: newClientActionId() })
  });
  if (!response.ok) throw new Error(await response.text());
  return response.json();
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
