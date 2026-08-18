<script lang="ts">
  import QRCode from 'qrcode';
  import { onMount } from 'svelte';
  import { contentUrl, createGame, getProjection, takeXToken, watchProjection, type CreatedGame } from './api';
  import Revision from './Revision.svelte';

  let created: CreatedGame | null = $state(null);
  let busy = $state(false);
  let error = $state('');
  let qrCodes: string[] = $state([]);
  let stopWatching: (() => void) | null = null;

  onMount(async () => {
    const gameId = localStorage.getItem('arknova.gameId');
    if (!gameId) return;
    try {
      const projection = await getProjection(gameId);
      const companionUrls = Array.from({ length: projection.playerCount }, (_, index) => `${location.origin}/play?gameid=${projection.gameId}&player=${index + 1}`);
      await showGame({ projection, companionUrls });
    } catch { localStorage.removeItem('arknova.gameId'); }
  });

  async function showGame(game: CreatedGame) {
    created = game;
    localStorage.setItem('arknova.gameId', game.projection.gameId);
    qrCodes = await Promise.all(game.companionUrls.map((url) => QRCode.toDataURL(url, { width: 140, margin: 1, color: { dark: '#102d28', light: '#f4efe1' } })));
    stopWatching?.();
    stopWatching = watchProjection(game.projection.gameId, 0, (projection) => { if (created) created.projection = projection; });
  }

  async function start() {
    busy = true; error = '';
    try { await showGame(await createGame(2)); }
    catch (reason) { error = reason instanceof Error ? reason.message : String(reason); }
    finally { busy = false; }
  }

  async function chooseAction(actionCard: string) {
    if (!created || !created.projection.activePlayer) return;
    busy = true; error = '';
    try {
      await takeXToken(created.projection.gameId, created.projection.activePlayer, actionCard, created.projection.revision);
    } catch (reason) { error = reason instanceof Error ? reason.message : String(reason); }
    finally { busy = false; }
  }
</script>

<svelte:head><title>Ark Nova Table</title></svelte:head>
<main>
  <header>
    <div><p class="eyebrow">DIGITAL TABLETOP</p><h1>Ark Nova</h1></div>
    {#if created}<div class="game-meta"><strong data-testid="game-code">{created.projection.gameId}</strong><Revision revision={created.projection.revision} /></div>{/if}
  </header>
  {#if !created}
    <section class="welcome">
      <div class="mark" aria-hidden="true">A</div>
      <p>Bring your zoo to life around the table.</p>
      <button onclick={start} disabled={busy}>{busy ? 'Creating game…' : 'Start two-player game'}</button>
      {#if error}<p class="error" role="alert">{error}</p>{/if}
    </section>
  {:else}
    <section class="game">
      <div class="turn" data-testid="active-player">Player {created.projection.activePlayer}’s turn</div>
      <section class="display" aria-label="Public card display">
        <h2>Public card display</h2>
        <div class="cards">
          {#each created.projection.display ?? [] as card}
            <article class="card" data-testid="display-card"><img src={contentUrl(card.image)} alt="" /><strong>{card.name}</strong></article>
          {/each}
        </div>
      </section>
      <section class="player-rows" aria-label="Player action cards">
        {#each created.projection.players ?? [] as player}
          <article class:active={player.seat === created.projection.activePlayer} class="player-row" data-testid={`player-${player.seat}-actions`}>
            <div class="player-label"><span>PLAYER {player.seat}</span><strong>{player.xTokens} X-token{player.xTokens === 1 ? '' : 's'}</strong></div>
            <div class="actions">
              {#each player.actionCards as actionCard, index}
                <button
                  data-testid={`action-card-player-${player.seat}-${actionCard.toLowerCase()}`}
                  aria-label={`${actionCard} at strength ${index + 1}; take an X-token`}
                  disabled={busy || player.seat !== created.projection.activePlayer}
                  onclick={() => chooseAction(actionCard)}
                ><span>{index + 1}</span>{actionCard}</button>
              {/each}
            </div>
          </article>
        {/each}
      </section>
      <div class="lower">
        <section class="history" aria-label="Action history">
          <h2>History</h2>
          {#each created.projection.history ?? [] as entry}<p>{entry.summary}</p>{/each}
        </section>
        <section class="seats" aria-label="Companion links">
          {#each created.companionUrls as url, index}
            <article data-testid={`seat-${index + 1}-qr`}><span>PLAYER {index + 1}</span><img src={qrCodes[index]} alt={`QR code for player ${index + 1}`} /><a href={url}>{url}</a></article>
          {/each}
        </section>
      </div>
      {#if error}<p class="error" role="alert">{error}</p>{/if}
    </section>
  {/if}
</main>

<style>
  :global(*){box-sizing:border-box}:global(body){margin:0;background:#0b211e;color:#f4efe1;font-family:Inter,ui-sans-serif,system-ui;min-height:100vh}main{min-height:100vh;padding:1.4rem 2rem;background:radial-gradient(circle at 50% 20%,#28574c 0,#133a33 42%,#0b211e 80%)}
  header{display:flex;justify-content:space-between;align-items:center;border-bottom:1px solid #ffffff25;padding-bottom:.8rem}.eyebrow{font-size:.65rem;letter-spacing:.3em;color:#8fc8b5;margin:0 0 .15rem}h1{font:400 1.8rem Georgia;margin:0}.game-meta{display:flex;align-items:center;gap:1.2rem}.game-meta>strong{font:1.6rem Georgia;letter-spacing:.15em;color:#e6c971}
  .welcome{height:75vh;display:grid;place-content:center;text-align:center;justify-items:center}.mark{border:1px solid #cfb46b;border-radius:50%;width:8rem;height:8rem;display:grid;place-items:center;font:italic 5rem Georgia;color:#cfb46b}.welcome p{font:1.45rem Georgia;color:#dbe7df;margin:1.7rem}.welcome button{background:#e6c971;border:0;border-radius:.35rem;color:#102d28;font-weight:800;font-size:1rem;padding:1rem 1.6rem;cursor:pointer}
  .game{max-width:1260px;margin:0 auto}.turn{text-align:center;color:#102d28;background:#e6c971;border-radius:0 0 .5rem .5rem;width:max-content;margin:0 auto .65rem;padding:.4rem 1.2rem;font-weight:800}h2{font:1rem Georgia;margin:.35rem 0 .45rem;color:#dbe7df}.cards{display:grid;grid-template-columns:repeat(6,1fr);gap:.55rem}.card{display:flex;align-items:center;gap:.55rem;background:#f4efe1;color:#102d28;border-radius:.35rem;padding:.4rem;min-width:0}.card img{width:38px;height:54px;object-fit:cover;border-radius:.2rem}.card strong{font:clamp(.65rem,1vw,.85rem) Georgia}
  .player-rows{display:grid;grid-template-columns:repeat(2,1fr);gap:.7rem;margin-top:.75rem}.player-row{border:1px solid #ffffff20;border-radius:.5rem;padding:.55rem;background:#ffffff08}.player-row.active{border-color:#e6c971}.player-label{display:flex;justify-content:space-between;font-size:.7rem;letter-spacing:.1em;color:#a4c7bb}.actions{display:grid;grid-template-columns:repeat(5,1fr);gap:.35rem;margin-top:.45rem}.actions button{display:flex;flex-direction:column;align-items:center;gap:.15rem;padding:.45rem .2rem;border:1px solid #ffffff25;border-radius:.3rem;background:#183f37;color:#f4efe1;font-size:.68rem;cursor:pointer}.actions button span{color:#e6c971;font-weight:900;font-size:1rem}.actions button:disabled{opacity:.45;cursor:default}
  .lower{display:grid;grid-template-columns:1fr 1.25fr;gap:.8rem;margin-top:.75rem}.history{border:1px solid #ffffff18;border-radius:.5rem;padding:.55rem .8rem;max-height:150px;overflow:auto}.history p{font-size:.75rem;color:#b8d0c7;margin:.3rem 0}.seats{display:grid;grid-template-columns:repeat(2,1fr);gap:.65rem}.seats article{display:grid;grid-template-columns:auto 80px;grid-template-rows:auto 1fr;column-gap:.6rem;align-items:center;background:#f4efe1;color:#102d28;border-radius:.5rem;padding:.45rem .65rem}.seats span{letter-spacing:.15em;font-size:.65rem;color:#47695f;font-weight:800}.seats img{grid-column:2;grid-row:1/3;width:80px}.seats a{font-size:.58rem;color:#47695f;overflow-wrap:anywhere}.error{color:#ff9f91;text-align:center}
  @media(max-width:800px){main{padding:1rem}.cards{grid-template-columns:repeat(3,1fr)}.player-rows,.lower{grid-template-columns:1fr}.seats{display:none}}
</style>
