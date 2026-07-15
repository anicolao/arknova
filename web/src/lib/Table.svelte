<script lang="ts">
  import QRCode from 'qrcode';
  import { onMount } from 'svelte';
  import { createGame, getProjection, watchProjection, type CreatedGame } from './api';
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
    qrCodes = await Promise.all(game.companionUrls.map((url) => QRCode.toDataURL(url, { width: 220, margin: 1, color: { dark: '#102d28', light: '#f4efe1' } })));
    stopWatching?.();
    stopWatching = watchProjection(game.projection.gameId, 0, (projection) => { if (created) created.projection = projection; });
  }

  async function start() {
    busy = true; error = '';
    try {
      await showGame(await createGame(2));
    } catch (reason) { error = reason instanceof Error ? reason.message : String(reason); }
    finally { busy = false; }
  }
</script>

<svelte:head><title>Ark Nova Table</title></svelte:head>
<main>
  <header><div><p class="eyebrow">DIGITAL TABLETOP</p><h1>Ark Nova</h1></div>{#if created}<Revision revision={created.projection.revision} />{/if}</header>
  {#if !created}
    <section class="welcome">
      <div class="mark" aria-hidden="true">A</div>
      <p>Bring your zoo to life around the table.</p>
      <button onclick={start} disabled={busy}>{busy ? 'Creating game…' : 'Start two-player game'}</button>
      {#if error}<p class="error" role="alert">{error}</p>{/if}
    </section>
  {:else}
    <section class="game">
      <div class="code"><span>GAME CODE</span><strong data-testid="game-code">{created.projection.gameId}</strong><p>Scan a code to join from a private device</p></div>
      <div class="seats">
        {#each created.companionUrls as url, index}
          <article data-testid={`seat-${index + 1}-qr`}><span>PLAYER {index + 1}</span><img src={qrCodes[index]} alt={`QR code for player ${index + 1}`} /><a href={url}>{url}</a></article>
        {/each}
      </div>
    </section>
  {/if}
</main>

<style>
  :global(*){box-sizing:border-box} :global(body){margin:0;background:#0b211e;color:#f4efe1;font-family:Inter,ui-sans-serif,system-ui;min-height:100vh}
  main{min-height:100vh;padding:2.5rem 4rem;background:radial-gradient(circle at 50% 30%,#28574c 0,#133a33 38%,#0b211e 75%)}
  header{display:flex;justify-content:space-between;align-items:center;border-bottom:1px solid #ffffff25;padding-bottom:1.2rem}.eyebrow{font-size:.7rem;letter-spacing:.3em;color:#8fc8b5;margin:0 0 .2rem}h1{font-family:Georgia,serif;font-weight:400;font-size:2rem;margin:0}
  .welcome{height:70vh;display:grid;place-content:center;text-align:center;justify-items:center}.mark{border:1px solid #cfb46b;border-radius:50%;width:8rem;height:8rem;display:grid;place-items:center;font:italic 5rem Georgia;color:#cfb46b}.welcome p{font:1.45rem Georgia;color:#dbe7df;margin:1.7rem}.welcome button{background:#e6c971;border:0;border-radius:.35rem;color:#102d28;font-weight:800;font-size:1rem;padding:1rem 1.6rem;cursor:pointer}.error{color:#ff9f91!important}
  .game{max-width:950px;margin:3rem auto}.code{text-align:center}.code span,.seats span{letter-spacing:.2em;font-size:.7rem;color:#a4c7bb}.code strong{display:block;font:5rem Georgia;letter-spacing:.18em;color:#e6c971}.code p{color:#a4c7bb}.seats{display:grid;grid-template-columns:repeat(2,1fr);gap:2rem;margin-top:2rem}.seats article{background:#f4efe1;color:#102d28;border-radius:.7rem;padding:1.4rem;text-align:center;box-shadow:0 1rem 3rem #0005}.seats article span{color:#47695f;font-weight:800}.seats img{display:block;width:min(220px,100%);margin:1rem auto}.seats a{display:block;color:#47695f;font-size:.7rem;overflow-wrap:anywhere}
  @media(max-width:650px){main{padding:1.5rem}.seats{grid-template-columns:1fr}.code strong{font-size:3.3rem}}
</style>
