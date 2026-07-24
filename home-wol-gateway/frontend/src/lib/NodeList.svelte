<script lang="ts">
	import type { Device, Edge, NodeInfo } from "./api";

	let {
		nodes,
		edges,
		devices,
		selectedId,
		onSelect,
	}: {
		nodes: NodeInfo[];
		edges: Edge[];
		devices: Device[];
		selectedId: string | null;
		onSelect: (id: string) => void;
	} = $props();

	function connectedTo(id: string): string[] {
		const peers = new Set<string>();
		for (const e of edges) {
			if (e.a === id) peers.add(e.b);
			if (e.b === id) peers.add(e.a);
		}
		return [...peers];
	}

	function deviceCount(id: string): number {
		return devices.filter((d) => d.node_id === id).length;
	}
</script>

<ul class="list">
	{#each nodes as node (node.id)}
		{@const peers = connectedTo(node.id)}
		{@const count = deviceCount(node.id)}
		<li>
			<button type="button" class="row" class:selected={selectedId === node.id} onclick={() => onSelect(node.id)}>
				<span class="id">{node.id}</span>
				<span class="meta">
					{count} device{count === 1 ? "" : "s"}
					{#if peers.length > 0}
						· connected to {peers.join(", ")}
					{/if}
				</span>
			</button>
		</li>
	{/each}
</ul>

<style>
	.list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.row {
		width: 100%;
		text-align: left;
		border: none;
		background: var(--color-surface);
		border-radius: var(--radius-md);
		padding: 12px 16px;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.row.selected {
		background: var(--color-primary-soft);
	}

	.id {
		font-family: var(--font-mono);
		font-weight: 600;
		font-size: 0.9rem;
	}

	.row.selected .id {
		color: var(--color-primary-hover);
	}

	.meta {
		font-size: 0.78rem;
		color: var(--color-text-muted);
	}
</style>
