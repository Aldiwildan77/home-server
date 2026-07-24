<script lang="ts">
	import type { Device, Edge, NodeInfo } from "./api";
	import NodeList from "./NodeList.svelte";
	import GraphCanvas from "./GraphCanvas.svelte";
	import NodePanel from "./NodePanel.svelte";

	let {
		nodes,
		edges,
		devices,
		onChange,
		onSwitchGateway,
	}: {
		nodes: NodeInfo[];
		edges: Edge[];
		devices: Device[];
		onChange: () => void;
		onSwitchGateway: (url: string) => Promise<boolean>;
	} = $props();

	let mode = $state<"list" | "graph">("graph");
	let selectedId = $state<string | null>(null);
	let pulsingNodeId = $state<string | null>(null);
	const selectedNode = $derived(nodes.find((n) => n.id === selectedId) ?? null);

	function select(id: string) {
		selectedId = selectedId === id ? null : id;
	}

	function pulseNode(id: string) {
		pulsingNodeId = id;
		setTimeout(() => {
			if (pulsingNodeId === id) pulsingNodeId = null;
		}, 1000);
	}
</script>

{#if nodes.length === 0}
	<p class="empty">No nodes reporting yet.</p>
{:else}
	<div class="layout">
		<div class="main">
			<div class="mode-switch">
				<button type="button" class:active={mode === "graph"} onclick={() => (mode = "graph")}>Graph</button>
				<button type="button" class:active={mode === "list"} onclick={() => (mode = "list")}>List</button>
			</div>

			{#if mode === "graph"}
				<GraphCanvas {nodes} {edges} {devices} {selectedId} {pulsingNodeId} onSelect={select} />
			{:else}
				<NodeList {nodes} {edges} {devices} {selectedId} onSelect={select} />
			{/if}
		</div>

		<div class="sidebar">
			{#if selectedNode}
				<NodePanel node={selectedNode} {devices} {onChange} {onSwitchGateway} onWake={pulseNode} />
			{:else}
				<p class="hint">Tap a node to see its devices and controls.</p>
			{/if}
		</div>
	</div>
{/if}

<style>
	.empty {
		color: var(--color-text-muted);
		padding: 32px 4px;
	}

	.layout {
		display: flex;
		flex-direction: column;
		gap: 20px;
	}

	.main {
		display: flex;
		flex-direction: column;
		gap: 16px;
		min-width: 0;
	}

	.mode-switch {
		display: flex;
		gap: 4px;
		background: var(--color-surface);
		border-radius: var(--radius-md);
		padding: 4px;
		width: fit-content;
	}

	.mode-switch button {
		border: none;
		background: none;
		padding: 6px 16px;
		border-radius: var(--radius-sm);
		font-size: 0.8rem;
		font-weight: 600;
		color: var(--color-text-muted);
		transition: background-color 0.15s ease, color 0.15s ease;
	}

	.mode-switch button.active {
		background: var(--color-bg);
		color: var(--color-text);
		box-shadow: var(--shadow-sm);
	}

	.hint {
		font-size: 0.85rem;
		color: var(--color-text-muted);
		padding: 4px;
	}

	@media (min-width: 900px) {
		.layout {
			flex-direction: row;
			align-items: flex-start;
		}

		.main {
			flex: 1;
			min-width: 0;
		}

		.sidebar {
			width: 360px;
			flex-shrink: 0;
			position: sticky;
			top: 24px;
		}
	}
</style>
