export type Projection = { gameId: string; playerCount: number; player?: number; revision: number };
export type CreatedGame = { projection: Projection; companionUrls: string[] };

export async function createGame(playerCount: number): Promise<CreatedGame> {
  const response = await fetch('/api/games', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ playerCount, clientActionId: crypto.randomUUID() })
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
