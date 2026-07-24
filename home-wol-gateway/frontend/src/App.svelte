<script lang="ts">
	import Setup from "./lib/Setup.svelte";
	import DeviceList from "./lib/DeviceList.svelte";
	import GraphView from "./lib/GraphView.svelte";
	import Settings from "./lib/Settings.svelte";
	import About from "./lib/About.svelte";
	import { gateway } from "./lib/gateway.svelte";
	import { getInventory, getTopology, checkHealth, type Device, type NodeInfo, type Edge } from "./lib/api";

	const POLL_MS = 5000;

	let connected = $state(gateway.url !== "");
	let tab = $state<"devices" | "map">("devices");
	let view = $state<"main" | "settings" | "about">("main");
	let menuOpen = $state(false);
	let menuWrapEl: HTMLDivElement | undefined = $state();
	let devices = $state<Device[]>([]);
	let nodes = $state<NodeInfo[]>([]);
	let edges = $state<Edge[]>([]);
	let unreachable = $state(false);
	let pollHandle: ReturnType<typeof setInterval> | undefined;

	async function refresh() {
		try {
			const [inv, topo] = await Promise.all([getInventory(), getTopology()]);
			devices = inv;
			nodes = topo.nodes;
			edges = topo.edges;
			unreachable = false;
		} catch {
			unreachable = true;
		}
	}

	function stopPolling() {
		if (pollHandle) {
			clearInterval(pollHandle);
			pollHandle = undefined;
		}
	}

	function onConnected() {
		connected = true;
	}

	async function switchGateway(url: string): Promise<boolean> {
		const reachable = await checkHealth(url, gateway.token);
		if (!reachable) return false;

		gateway.set(url, gateway.token);
		await refresh();
		return true;
	}

	function logout() {
		menuOpen = false;
		stopPolling();
		gateway.clear();
		connected = false;
		view = "main";
		devices = [];
		nodes = [];
		edges = [];
	}

	function openSettings() {
		menuOpen = false;
		view = "settings";
	}

	function openAbout() {
		menuOpen = false;
		view = "about";
	}

	function backToMain() {
		view = "main";
		refresh();
	}

	$effect(() => {
		if (!connected) return;
		refresh();
		pollHandle = setInterval(refresh, POLL_MS);
		return stopPolling;
	});

	$effect(() => {
		if (!menuOpen) return;

		function handleClick(event: MouseEvent) {
			if (menuWrapEl && !menuWrapEl.contains(event.target as Node)) {
				menuOpen = false;
			}
		}

		document.addEventListener("click", handleClick, true);
		return () => document.removeEventListener("click", handleClick, true);
	});
</script>

{#if !connected}
	<Setup {onConnected} />
{:else}
	<div class="page">
		<header>
			<div class="brand">
				<svg class="signal" width="22" height="22" viewBox="0 0 24 24" fill="none" aria-hidden="true">
					<circle cx="12" cy="19" r="1.6" fill="var(--color-accent)" />
					<path d="M7 15a7 7 0 0 1 10 0" stroke="var(--color-primary)" stroke-width="2" stroke-linecap="round" class="arc arc-1" />
					<path d="M3.5 11.5a12 12 0 0 1 17 0" stroke="var(--color-primary)" stroke-width="2" stroke-linecap="round" class="arc arc-2" />
				</svg>
				<h1>Wake on Lan</h1>
			</div>
			<div class="header-right">
				{#if unreachable}
					<span class="warning">Can't reach gateway</span>
				{/if}
				<span class="address mono">{gateway.url}</span>

				<div class="menu-wrap" bind:this={menuWrapEl}>
					<button type="button" class="menu-trigger" onclick={() => (menuOpen = !menuOpen)} aria-label="Menu">
						<svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
							<circle cx="5" cy="12" r="1.7" fill="currentColor" />
							<circle cx="12" cy="12" r="1.7" fill="currentColor" />
							<circle cx="19" cy="12" r="1.7" fill="currentColor" />
						</svg>
					</button>

					{#if menuOpen}
						<div class="menu">
							<button type="button" onclick={openSettings}>Settings</button>
							<button type="button" onclick={openAbout}>About</button>
							<button type="button" class="logout" onclick={logout}>Logout</button>
						</div>
					{/if}
				</div>
			</div>
		</header>

		{#if view === "settings"}
			<Settings onDone={backToMain} />
		{:else if view === "about"}
			<About onDone={backToMain} />
		{:else}
			<nav>
				<button type="button" class:active={tab === "devices"} onclick={() => (tab = "devices")}>Devices</button>
				<button type="button" class:active={tab === "map"} onclick={() => (tab = "map")}>Map</button>
			</nav>

			<main>
				{#if tab === "devices"}
					<DeviceList {devices} onChange={refresh} />
				{:else}
					<GraphView {nodes} {edges} {devices} onChange={refresh} onSwitchGateway={switchGateway} />
				{/if}
			</main>
		{/if}
	</div>
{/if}

<style>
	.page {
		max-width: 1240px;
		margin: 0 auto;
		padding: 32px 32px 60px;
	}

	header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
		flex-wrap: wrap;
		margin-bottom: 24px;
	}

	.brand {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	h1 {
		font-size: 1.4rem;
	}

	.signal .arc {
		transform-origin: 12px 19px;
		animation: signal-breathe 2.4s ease-in-out infinite;
	}

	.signal .arc-2 {
		animation-delay: 0.3s;
	}

	@keyframes signal-breathe {
		0%,
		100% {
			opacity: 0.3;
		}
		50% {
			opacity: 1;
		}
	}

	.header-right {
		display: flex;
		align-items: center;
		gap: 12px;
	}

	.warning {
		font-size: 0.8rem;
		color: var(--color-danger);
	}

	.address {
		font-size: 0.8rem;
		color: var(--color-text-muted);
	}

	.menu-wrap {
		position: relative;
	}

	.menu-trigger {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		border: none;
		border-radius: var(--radius-sm);
		background: var(--color-surface);
		color: var(--color-text-muted);
		transition: background-color 0.15s ease, color 0.15s ease;
	}

	.menu-trigger:hover {
		background: var(--color-surface-strong);
		color: var(--color-text);
	}

	.menu {
		position: absolute;
		top: calc(100% + 8px);
		right: 0;
		background: var(--color-bg);
		border-radius: var(--radius-md);
		box-shadow: var(--shadow-md);
		padding: 6px;
		display: flex;
		flex-direction: column;
		min-width: 140px;
		z-index: 10;
	}

	.menu button {
		text-align: left;
		border: none;
		background: none;
		padding: 9px 12px;
		border-radius: var(--radius-sm);
		font-size: 0.85rem;
		font-weight: 600;
		color: var(--color-text);
	}

	.menu button:hover {
		background: var(--color-surface);
	}

	.menu button.logout {
		color: var(--color-danger);
	}

	nav {
		display: flex;
		gap: 4px;
		background: var(--color-surface);
		border-radius: var(--radius-md);
		padding: 4px;
		margin-bottom: 20px;
		width: fit-content;
	}

	nav button {
		border: none;
		background: none;
		padding: 8px 20px;
		border-radius: var(--radius-sm);
		font-size: 0.88rem;
		font-weight: 600;
		color: var(--color-text-muted);
		transition: background-color 0.15s ease, color 0.15s ease;
	}

	nav button.active {
		background: var(--color-bg);
		color: var(--color-text);
		box-shadow: var(--shadow-sm);
	}
</style>
