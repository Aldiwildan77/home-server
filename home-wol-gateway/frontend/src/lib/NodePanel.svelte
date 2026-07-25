<script lang="ts">
	import type { Device, NodeInfo } from "./api";
	import { setAllowed, wake, ApiError } from "./api";
	import { gateway } from "./gateway.svelte";

	let {
		node,
		devices,
		onChange,
		onSwitchGateway,
		onWake,
	}: {
		node: NodeInfo;
		devices: Device[];
		onChange: () => void;
		onSwitchGateway: (url: string) => Promise<boolean>;
		onWake?: (nodeId: string) => void;
	} = $props();

	let switching = $state(false);
	let switchError = $state("");
	let broadcasting = $state(false);
	let feedback = $state("");
	let pending = $state<Record<string, boolean>>({});
	let pulsing = $state<Record<string, boolean>>({});

	const ownDevices = $derived(devices.filter((d) => d.node_id === node.id));
	const isCurrentGateway = $derived(node.http_addr === gateway.url);

	function pulse(mac: string) {
		pulsing = { ...pulsing, [mac]: true };
		setTimeout(() => {
			pulsing = { ...pulsing, [mac]: false };
		}, 1000);
	}

	async function toggleAllow(device: Device) {
		pending = { ...pending, [device.mac]: true };
		try {
			await setAllowed(device.mac, !device.wol_allowed);
			onChange();
		} finally {
			pending = { ...pending, [device.mac]: false };
		}
	}

	async function wakeOne(device: Device) {
		pending = { ...pending, [device.mac]: true };
		try {
			await wake(device.mac);
			pulse(device.mac);
			onWake?.(node.id);
		} finally {
			pending = { ...pending, [device.mac]: false };
		}
	}

	async function wakeAll() {
		const targets = ownDevices.filter((d) => d.wol_allowed);
		if (targets.length === 0) return;

		broadcasting = true;
		feedback = "";
		try {
			await Promise.all(targets.map((d) => wake(d.mac)));
			feedback = `Woken ${targets.length} device${targets.length === 1 ? "" : "s"}.`;
			for (const d of targets) pulse(d.mac);
			onWake?.(node.id);
		} catch (err) {
			feedback = err instanceof ApiError ? err.message : "Couldn't wake every device.";
		} finally {
			broadcasting = false;
			setTimeout(() => (feedback = ""), 4000);
		}
	}

	async function switchToNode() {
		if (!node.http_addr) return;

		switching = true;
		switchError = "";
		const ok = await onSwitchGateway(node.http_addr);
		switching = false;

		if (!ok) {
			switchError = "Can't reach this node directly.";
		}
	}
</script>

<div class="panel">
	<div class="head">
		<h3 class="mono">{node.id}</h3>
		{#if node.http_addr}
			{#if isCurrentGateway}
				<span class="current">Connected</span>
			{:else}
				<button type="button" class="switch" disabled={switching} onclick={switchToNode}>
					{switching ? "Connecting…" : "Switch to this node"}
				</button>
			{/if}
		{:else}
			<span class="unreachable">No direct address configured</span>
		{/if}
	</div>

	{#if switchError}
		<p class="error">{switchError}</p>
	{/if}

	<div class="actions">
		<button type="button" class="broadcast" disabled={broadcasting} onclick={wakeAll}>
			{broadcasting ? "Waking…" : "Wake all allowed"}
		</button>
		{#if feedback}
			<span class="feedback">{feedback}</span>
		{/if}
	</div>

	{#if ownDevices.length === 0}
		<p class="empty">No devices on this node.</p>
	{:else}
		<ul class="devices">
			{#each ownDevices as device (device.mac)}
				<li class="device">
					<span class="dot-wrap">
						<span class="dot" class:online={device.online}></span>
						{#if pulsing[device.mac]}
							<span class="ripple"></span>
							<span class="ripple ripple-delay"></span>
						{/if}
					</span>
					<span class="name">
						{device.alias || device.hostname || device.mac}
						<span class="ip mono">{device.ip} · {device.mac}</span>
					</span>

					<label class="switch-toggle">
						<input
							type="checkbox"
							checked={device.wol_allowed}
							disabled={pending[device.mac]}
							onchange={() => toggleAllow(device)}
						/>
						<span class="track"><span class="thumb"></span></span>
					</label>

					<button
						type="button"
						class="wake"
						disabled={!device.wol_allowed || pending[device.mac]}
						onclick={() => wakeOne(device)}
					>
						Wake
					</button>
				</li>
			{/each}
		</ul>
	{/if}
</div>

<style>
	.panel {
		background: var(--color-surface);
		border-radius: var(--radius-md);
		padding: 20px;
		display: flex;
		flex-direction: column;
		gap: 14px;
	}

	.head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
	}

	h3 {
		font-size: 1rem;
	}

	.current {
		font-size: 0.75rem;
		font-weight: 600;
		color: var(--color-online);
	}

	.unreachable {
		font-size: 0.75rem;
		color: var(--color-text-muted);
	}

	.switch {
		border: none;
		background: var(--color-primary-soft);
		color: var(--color-primary-hover);
		font-size: 0.78rem;
		font-weight: 600;
		padding: 6px 12px;
		border-radius: var(--radius-sm);
	}

	.error {
		font-size: 0.8rem;
		color: var(--color-danger);
		margin-top: -6px;
	}

	.actions {
		display: flex;
		align-items: center;
		gap: 12px;
	}

	.broadcast {
		border: none;
		background: var(--color-accent);
		color: #574a05;
		font-size: 0.82rem;
		font-weight: 600;
		padding: 8px 16px;
		border-radius: var(--radius-sm);
		transition: background-color 0.15s ease, box-shadow 0.15s ease;
	}

	.broadcast:hover:not(:disabled) {
		background: var(--color-accent-strong);
		box-shadow: var(--glow-accent);
	}

	.feedback {
		font-size: 0.78rem;
		color: var(--color-primary);
	}

	.empty {
		font-size: 0.85rem;
		color: var(--color-text-muted);
	}

	.devices {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.device {
		display: flex;
		align-items: center;
		gap: 10px;
		background: var(--color-bg);
		border-radius: var(--radius-sm);
		padding: 10px 12px;
	}

	.dot-wrap {
		position: relative;
		width: 7px;
		height: 7px;
		flex-shrink: 0;
	}

	.dot {
		display: block;
		width: 7px;
		height: 7px;
		border-radius: 50%;
		background: #cbd5e1;
	}

	.dot.online {
		background: var(--color-online);
	}

	.ripple {
		position: absolute;
		inset: 0;
		border-radius: 50%;
		background: var(--color-accent);
		animation: pulse-ring 1s ease-out;
		pointer-events: none;
	}

	.ripple-delay {
		animation-delay: 0.25s;
	}

	.name {
		flex: 1;
		font-size: 0.85rem;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.ip {
		margin-left: 8px;
		font-size: 0.75rem;
		font-weight: 400;
		color: var(--color-text-muted);
	}

	.switch-toggle {
		cursor: pointer;
		flex-shrink: 0;
	}

	.switch-toggle input {
		position: absolute;
		width: 1px;
		height: 1px;
		overflow: hidden;
		opacity: 0;
	}

	.track {
		width: 34px;
		height: 19px;
		border-radius: 999px;
		background: #dbe2ee;
		position: relative;
		display: block;
		transition: background-color 0.15s ease;
	}

	.switch-toggle input:checked + .track {
		background: var(--color-accent);
	}

	.thumb {
		position: absolute;
		top: 2px;
		left: 2px;
		width: 15px;
		height: 15px;
		border-radius: 50%;
		background: #ffffff;
		transition: transform 0.15s ease;
	}

	.switch-toggle input:checked + .track .thumb {
		transform: translateX(15px);
	}

	.wake {
		flex-shrink: 0;
		border: none;
		background: var(--color-primary);
		color: #ffffff;
		font-size: 0.78rem;
		font-weight: 600;
		padding: 6px 14px;
		border-radius: var(--radius-sm);
		transition: background-color 0.15s ease, box-shadow 0.15s ease;
	}

	.wake:hover:not(:disabled) {
		background: var(--color-primary-hover);
		box-shadow: var(--glow-primary);
	}
</style>
