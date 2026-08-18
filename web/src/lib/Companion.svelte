<script lang="ts">
  import { contentUrl, getProjection, watchProjection, type Projection } from './api';
  import Revision from './Revision.svelte';
  const params = new URLSearchParams(location.search);
  const gameId = params.get('gameid')?.toUpperCase() ?? '';
  const player = Number(params.get('player'));
  let projection: Projection | null = $state(null);
  let error = $state('');
  getProjection(gameId, player).then((value) => {
    projection = value;
    watchProjection(gameId, player, (next) => projection = next);
  }).catch((reason) => error = reason instanceof Error ? reason.message : String(reason));
</script>

<svelte:head><title>Player {player} · Ark Nova</title></svelte:head>
<main>
  {#if error}<p role="alert">{error}</p>{:else if !projection}<p>Connecting…</p>{:else}
    <header><div class="animal">{player === 1 ? '◒' : '◇'}</div><div><span>GAME {projection.gameId}</span><h1>Player {projection.player}</h1></div></header>
    <div class:active={projection.activePlayer === player} class="turn" data-testid="active-player">
      {projection.activePlayer === player ? 'Your turn' : `Player ${projection.activePlayer}’s turn`}
    </div>
    <section class="hand" aria-label="Private hand">
      <p class="label">PRIVATE HAND · ONLY YOU CAN SEE THIS</p>
      <h2>Your starting hand</h2>
      <div class="cards">
        {#each projection.hand ?? [] as card}
          <article data-testid="hand-card"><img src={contentUrl(card.image)} alt="" /><strong>{card.name}</strong></article>
        {/each}
      </div>
    </section>
    {@const ownState = projection.players?.find((state) => state.seat === player)}
    {#if ownState}
      <section class="action-row" aria-label="Your action cards">
        <div class="action-heading"><h2>Action cards</h2><strong data-testid="x-token-count">{ownState.xTokens} X-token{ownState.xTokens === 1 ? '' : 's'}</strong></div>
        <div class="actions">{#each ownState.actionCards as action, index}<div><span>{index + 1}</span>{action}</div>{/each}</div>
      </section>
    {/if}
    <footer><Revision revision={projection.revision} /></footer>
  {/if}
</main>
<style>
  :global(*){box-sizing:border-box}:global(body){margin:0;background:#102d28;color:#f4efe1;font-family:Inter,system-ui}main{min-height:100vh;padding:1.4rem;background:linear-gradient(160deg,#1d4c42,#0b211e)}header{display:flex;gap:1rem;align-items:center}.animal{display:grid;place-items:center;width:3.5rem;height:3.5rem;border:1px solid #d9bd6f;border-radius:50%;color:#d9bd6f;font-size:1.8rem}header span,.label{font-size:.62rem;letter-spacing:.16em;color:#9ac5b5}h1{font:2rem Georgia;margin:.15rem 0}.turn{margin:1.2rem 0 -.4rem auto;border:1px solid #ffffff20;border-radius:2rem;width:max-content;padding:.4rem .8rem;color:#a9c3ba;font-size:.78rem}.turn.active{background:#e6c971;color:#102d28;font-weight:800}.hand,.action-row{margin-top:1rem;padding:1rem;border:1px solid #ffffff20;border-radius:.7rem;background:#ffffff08}h2{font:1.35rem Georgia;margin:.35rem 0 .7rem}.cards{display:grid;grid-template-columns:repeat(3,1fr);gap:.55rem}.cards article{background:#f4efe1;color:#102d28;border-radius:.4rem;padding:.35rem;min-width:0}.cards img{display:block;width:100%;aspect-ratio:5/7;object-fit:cover;border-radius:.25rem}.cards strong{display:block;font:.75rem Georgia;margin:.35rem .15rem;overflow-wrap:anywhere}.action-heading{display:flex;align-items:center;justify-content:space-between}.action-heading strong{font-size:.75rem;color:#e6c971}.actions{display:grid;grid-template-columns:repeat(5,1fr);gap:.3rem}.actions div{text-align:center;background:#183f37;border:1px solid #ffffff20;border-radius:.3rem;padding:.4rem .1rem;font-size:.55rem}.actions span{display:block;color:#e6c971;font-size:.85rem;font-weight:900}footer{margin-top:1.2rem}
</style>
