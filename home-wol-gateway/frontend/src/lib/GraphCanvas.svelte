<script lang="ts">
	import type { Device, Edge, NodeInfo } from "./api";
	import { layoutForce } from "./force-layout";

	let {
		nodes,
		edges,
		devices,
		selectedId,
		pulsingNodeId,
		onSelect,
	}: {
		nodes: NodeInfo[];
		edges: Edge[];
		devices: Device[];
		selectedId: string | null;
		pulsingNodeId?: string | null;
		onSelect: (id: string) => void;
	} = $props();

	const WIDTH = 780;
	const HEIGHT = 480;
	const RADIUS = 24;
	const PADDING = 70;

	const layout = $derived.by(() => {
		const positions = layoutForce(nodes, edges, WIDTH, HEIGHT, PADDING);
		const byId = new Map(positions.map((p) => [p.id, p]));

		const lines = edges
			.map((e) => {
				const a = byId.get(e.a);
				const b = byId.get(e.b);
				return a && b ? { x1: a.x, y1: a.y, x2: b.x, y2: b.y } : null;
			})
			.filter((l) => l !== null);

		const drawn = positions.map((p) => {
			const own = devices.filter((d) => d.node_id === p.id);
			return {
				id: p.id,
				x: p.x,
				y: p.y,
				deviceCount: own.length,
				onlineCount: own.filter((d) => d.online).length,
			};
		});

		return { drawn, lines };
	});

	function handleKey(event: KeyboardEvent, id: string) {
		if (event.key === "Enter" || event.key === " ") {
			event.preventDefault();
			onSelect(id);
		}
	}
</script>

{#if nodes.length === 0}
	<p class="empty">No nodes reporting yet.</p>
{:else}
	<div class="scroll dot-grid">
		<svg viewBox={`0 0 ${WIDTH} ${HEIGHT}`} width={WIDTH} height={HEIGHT} role="img" aria-label="Gateway mesh graph">
			{#each layout.lines as line, i (i)}
				<line x1={line.x1} y1={line.y1} x2={line.x2} y2={line.y2} class="edge" />
			{/each}

			{#each layout.drawn as p (p.id)}
				<g
					class="node"
					class:selected={selectedId === p.id}
					transform={`translate(${p.x}, ${p.y})`}
					role="button"
					tabindex="0"
					onclick={() => onSelect(p.id)}
					onkeydown={(e) => handleKey(e, p.id)}
				>
					{#if pulsingNodeId === p.id}
						<circle r={RADIUS} class="wake-ripple" />
						<circle r={RADIUS} class="wake-ripple wake-ripple-delay" />
					{/if}
					<circle r={RADIUS} class="circle" />
					{#if p.onlineCount > 0}
						<circle r={4} cx={RADIUS - 6} cy={-RADIUS + 6} class="badge" />
					{/if}
					<text class="label" y={RADIUS + 16}>{p.id}</text>
					<text class="sub" y={RADIUS + 30}>{p.deviceCount} device{p.deviceCount === 1 ? "" : "s"}</text>
				</g>
			{/each}
		</svg>
	</div>
{/if}

<style>
	.empty {
		color: var(--color-text-muted);
		padding: 32px 4px;
	}

	.scroll {
		overflow-x: auto;
		padding: 8px 0 4px;
		border-radius: var(--radius-lg);
	}

	.edge {
		stroke: var(--color-surface-strong);
		stroke-width: 2;
		stroke-dasharray: 1 7;
		stroke-linecap: round;
		animation: edge-flow 1.2s linear infinite;
	}

	@keyframes edge-flow {
		to {
			stroke-dashoffset: -16;
		}
	}

	.node {
		cursor: pointer;
	}

	.node:focus-visible {
		outline: 2px solid var(--color-primary);
		outline-offset: 4px;
	}

	.circle {
		fill: var(--color-primary-soft);
		transition: fill 0.15s ease;
	}

	.wake-ripple {
		fill: var(--color-accent);
		transform-box: fill-box;
		transform-origin: center;
		animation: pulse-ring 1s ease-out;
		pointer-events: none;
	}

	.wake-ripple-delay {
		animation-delay: 0.25s;
	}

	.node:hover .circle {
		fill: var(--color-surface-strong);
	}

	.node.selected .circle {
		fill: var(--color-primary);
		filter: drop-shadow(0 0 6px rgba(21, 128, 61, 0.55));
	}

	.node.selected .label {
		fill: var(--color-primary-hover);
		font-weight: 700;
	}

	.badge {
		fill: var(--color-online);
	}

	.label {
		font-family: var(--font-mono);
		font-size: 12px;
		font-weight: 600;
		fill: var(--color-text);
		text-anchor: middle;
	}

	.sub {
		font-size: 10px;
		fill: var(--color-text-muted);
		text-anchor: middle;
	}
</style>
