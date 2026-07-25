<script lang="ts">
	import type { Device } from "./api";
	import { setAllowed, setAlias, wake, ApiError } from "./api";

	let { devices, onChange }: { devices: Device[]; onChange: () => void } = $props();

	let pending = $state<Record<string, boolean>>({});
	let feedback = $state<Record<string, string>>({});
	let pulsing = $state<Record<string, boolean>>({});
	let editingMac = $state<string | null>(null);
	let editValue = $state("");

	function deviceName(device: Device): string {
		return device.alias || device.hostname || device.mac;
	}

	function startEdit(device: Device) {
		editingMac = device.mac;
		editValue = device.alias || "";
	}

	function cancelEdit() {
		editingMac = null;
		editValue = "";
	}

	async function saveAlias(device: Device) {
		const alias = editValue.trim();
		editingMac = null;
		if (alias === (device.alias || "")) return;

		try {
			await setAlias(device.mac, alias);
			onChange();
		} catch (err) {
			feedback = { ...feedback, [device.mac]: err instanceof ApiError ? err.message : "Couldn't rename this device." };
		}
	}

	function autofocus(node: HTMLInputElement) {
		node.focus();
	}

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
		} catch (err) {
			feedback = { ...feedback, [device.mac]: err instanceof ApiError ? err.message : "Couldn't update this device." };
		} finally {
			pending = { ...pending, [device.mac]: false };
		}
	}

	async function wakeDevice(device: Device) {
		pending = { ...pending, [device.mac]: true };
		feedback = { ...feedback, [device.mac]: "" };
		try {
			await wake(device.mac);
			feedback = { ...feedback, [device.mac]: "Woken." };
			pulse(device.mac);
		} catch (err) {
			feedback = { ...feedback, [device.mac]: err instanceof ApiError ? err.message : "Couldn't send wake." };
		} finally {
			pending = { ...pending, [device.mac]: false };
			setTimeout(() => {
				feedback = { ...feedback, [device.mac]: "" };
			}, 4000);
		}
	}
</script>

{#if devices.length === 0}
	<p class="empty">No devices reported yet. They'll show up here once a gateway or agent starts sending in reports.</p>
{:else}
	<ul class="list">
		{#each devices as device (device.mac)}
			<li class="row">
				<span class="dot-wrap">
					<span class="dot" class:online={device.online} aria-label={device.online ? "Online" : "Offline"}></span>
					{#if pulsing[device.mac]}
						<span class="ripple"></span>
						<span class="ripple ripple-delay"></span>
					{/if}
				</span>

				<div class="info">
					{#if editingMac === device.mac}
						<input
							class="name-input"
							type="text"
							bind:value={editValue}
							placeholder={device.hostname || device.mac}
							use:autofocus
							onblur={() => saveAlias(device)}
							onkeydown={(e) => {
								if (e.key === "Enter") saveAlias(device);
								if (e.key === "Escape") cancelEdit();
							}}
						/>
					{:else}
						<button type="button" class="name" onclick={() => startEdit(device)} title="Click to rename">
							{deviceName(device)}
						</button>
					{/if}
					<span class="meta"><span class="mono">{device.ip} · {device.mac}</span> · via {device.node_id}</span>
					{#if feedback[device.mac]}
						<span class="feedback">{feedback[device.mac]}</span>
					{/if}
				</div>

				<label class="switch">
					<input
						type="checkbox"
						checked={device.wol_allowed}
						disabled={pending[device.mac]}
						onchange={() => toggleAllow(device)}
					/>
					<span class="track"><span class="thumb"></span></span>
					<span class="switch-label">Allow wake</span>
				</label>

				<button
					type="button"
					class="wake"
					disabled={!device.wol_allowed || pending[device.mac]}
					onclick={() => wakeDevice(device)}
				>
					Wake
				</button>
			</li>
		{/each}
	</ul>
{/if}

<style>
	.empty {
		color: var(--color-text-muted);
		padding: 32px 4px;
	}

	.list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.row {
		display: flex;
		align-items: center;
		gap: 16px;
		padding: 16px 20px;
		background: var(--color-surface);
		border-radius: var(--radius-md);
	}

	.dot-wrap {
		position: relative;
		width: 10px;
		height: 10px;
		flex-shrink: 0;
	}

	.dot {
		display: block;
		width: 10px;
		height: 10px;
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

	.info {
		display: flex;
		flex-direction: column;
		gap: 2px;
		flex: 1;
		min-width: 0;
	}

	.name {
		font-weight: 600;
		font-size: 0.95rem;
		background: none;
		border: none;
		padding: 0;
		text-align: left;
		color: var(--color-text);
		cursor: text;
		width: fit-content;
		max-width: 100%;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.name:hover {
		text-decoration: underline dotted;
	}

	.name-input {
		font-weight: 600;
		font-size: 0.95rem;
		font-family: inherit;
		color: var(--color-text);
		background: var(--color-bg);
		border: none;
		border-radius: var(--radius-sm);
		padding: 2px 6px;
		margin: -2px -6px;
		width: 100%;
		max-width: 240px;
	}

	.name-input:focus {
		outline: 2px solid var(--color-primary);
		outline-offset: 1px;
	}

	.meta {
		font-size: 0.8rem;
		color: var(--color-text-muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.feedback {
		font-size: 0.8rem;
		color: var(--color-primary);
	}

	.switch {
		display: flex;
		align-items: center;
		gap: 8px;
		cursor: pointer;
		flex-shrink: 0;
	}

	.switch input {
		position: absolute;
		width: 1px;
		height: 1px;
		overflow: hidden;
		opacity: 0;
	}

	.switch-label {
		font-size: 0.8rem;
		color: var(--color-text-muted);
	}

	.track {
		width: 40px;
		height: 22px;
		border-radius: 999px;
		background: #dbe2ee;
		position: relative;
		transition: background-color 0.15s ease;
		flex-shrink: 0;
	}

	.switch input:checked + .track {
		background: var(--color-accent);
	}

	.thumb {
		position: absolute;
		top: 2px;
		left: 2px;
		width: 18px;
		height: 18px;
		border-radius: 50%;
		background: #ffffff;
		box-shadow: var(--shadow-sm);
		transition: transform 0.15s ease;
	}

	.switch input:checked + .track .thumb {
		transform: translateX(18px);
	}

	.switch input:focus-visible + .track {
		outline: 2px solid var(--color-primary);
		outline-offset: 2px;
	}

	.wake {
		flex-shrink: 0;
		padding: 9px 18px;
		border: none;
		border-radius: var(--radius-sm);
		background: var(--color-primary);
		color: #ffffff;
		font-size: 0.85rem;
		font-weight: 600;
		transition: background-color 0.15s ease, box-shadow 0.15s ease;
	}

	.wake:hover:not(:disabled) {
		background: var(--color-primary-hover);
		box-shadow: var(--glow-primary);
	}

	@media (max-width: 640px) {
		.row {
			flex-wrap: wrap;
		}

		.info {
			flex-basis: 100%;
			order: -1;
		}
	}
</style>
