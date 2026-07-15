<script lang="ts">
  let { revision, connected = true }: { revision: number; connected?: boolean } = $props();
</script>

<div class:stale={!connected} class="revision" aria-live="polite">
  <span class="dot"></span>
  <span>{connected ? 'Connected' : 'Reconnecting'}</span>
  <output data-testid="projection-revision">Revision {revision}</output>
</div>

<style>
  .revision { display:flex; gap:.65rem; align-items:center; color:#c9ddd6; font-size:.9rem }
  .dot { width:.55rem; height:.55rem; border-radius:50%; background:#5ee0a0; box-shadow:0 0 0 .2rem #5ee0a025 }
  output { border-left:1px solid #ffffff30; padding-left:.65rem }
  .stale .dot { background:#f6c65b }
</style>
