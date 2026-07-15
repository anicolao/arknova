<script lang="ts">
  import Table from '$lib/Table.svelte';
  import Companion from '$lib/Companion.svelte';
  const companion = location.pathname === '/play';
</script>

{#if companion}<Companion />{:else}<Table />{/if}
