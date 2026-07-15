<script lang="ts">
  import { getProjection, watchProjection, type Projection } from './api';
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
    <section><p class="label">PRIVATE PLAYER AREA</p><h2>Your hand is empty</h2><p>Cards and private choices will appear here.</p></section>
    <footer><Revision revision={projection.revision} /></footer>
  {/if}
</main>
<style>
  :global(*){box-sizing:border-box}:global(body){margin:0;background:#102d28;color:#f4efe1;font-family:Inter,system-ui}main{min-height:100vh;padding:2rem;background:linear-gradient(160deg,#1d4c42,#0b211e)}header{display:flex;gap:1rem;align-items:center}.animal{display:grid;place-items:center;width:4rem;height:4rem;border:1px solid #d9bd6f;border-radius:50%;color:#d9bd6f;font-size:2rem}span,.label{font-size:.7rem;letter-spacing:.18em;color:#9ac5b5}h1{font:2.2rem Georgia;margin:.2rem 0}section{margin-top:3rem;padding:2rem 1.5rem;border:1px solid #ffffff20;border-radius:.7rem;background:#ffffff08}h2{font:1.7rem Georgia;margin:.6rem 0}section p:last-child{color:#a9c3ba}footer{position:fixed;bottom:2rem}
</style>
