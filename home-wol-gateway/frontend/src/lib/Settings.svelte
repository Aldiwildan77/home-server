<script lang="ts">
	import { checkHealth } from "./api";
	import { gateway } from "./gateway.svelte";

	let { onDone }: { onDone: () => void } = $props();

	let address = $state(gateway.url);
	let token = $state(gateway.token);
	let checking = $state(false);
	let error = $state("");
	let saved = $state(false);

	async function save(event: SubmitEvent) {
		event.preventDefault();

		const url = address.trim().replace(/\/+$/, "");
		if (!url || !token.trim()) {
			error = "Both fields are required.";
			return;
		}

		checking = true;
		error = "";
		saved = false;

		const reachable = await checkHealth(url, token.trim());

		checking = false;

		if (!reachable) {
			error = "Can't reach that address, or the token's wrong.";
			return;
		}

		gateway.set(url, token);
		saved = true;
		setTimeout(() => (saved = false), 2500);
	}
</script>

<div class="panel">
	<h2>Settings</h2>
	<p class="lead">Change which gateway this app talks to.</p>

	<form onsubmit={save}>
		<label for="s-address">Gateway address</label>
		<input id="s-address" type="text" bind:value={address} disabled={checking} autocomplete="off" class="mono" />

		<label for="s-token" class="token-label">API token</label>
		<input id="s-token" type="password" bind:value={token} disabled={checking} autocomplete="off" />

		{#if error}
			<p class="error">{error}</p>
		{/if}
		{#if saved}
			<p class="success">Saved.</p>
		{/if}

		<div class="actions">
			<button type="submit" class="save" disabled={checking}>
				{checking ? "Checking…" : "Save"}
			</button>
			<button type="button" class="back" onclick={onDone}>Back</button>
		</div>
	</form>
</div>

<style>
	.panel {
		background: var(--color-surface);
		border-radius: var(--radius-md);
		padding: 24px;
		max-width: 420px;
	}

	h2 {
		font-size: 1.05rem;
	}

	.lead {
		margin-top: 6px;
		color: var(--color-text-muted);
		font-size: 0.88rem;
	}

	form {
		margin-top: 20px;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	label {
		font-size: 0.82rem;
		font-weight: 600;
		color: var(--color-text);
	}

	.token-label {
		margin-top: 8px;
	}

	input {
		padding: 11px 14px;
		border: none;
		border-radius: var(--radius-sm);
		background: var(--color-bg);
		font-size: 0.9rem;
		color: var(--color-text);
	}

	.error {
		color: var(--color-danger);
		font-size: 0.82rem;
	}

	.success {
		color: var(--color-online);
		font-size: 0.82rem;
	}

	.actions {
		display: flex;
		gap: 10px;
		margin-top: 8px;
	}

	.save {
		padding: 10px 18px;
		border: none;
		border-radius: var(--radius-sm);
		background: var(--color-primary);
		color: #ffffff;
		font-size: 0.88rem;
		font-weight: 600;
		transition: background-color 0.15s ease, box-shadow 0.15s ease;
	}

	.save:hover:not(:disabled) {
		background: var(--color-primary-hover);
		box-shadow: var(--glow-primary);
	}

	.back {
		padding: 10px 18px;
		border: none;
		border-radius: var(--radius-sm);
		background: none;
		color: var(--color-text-muted);
		font-size: 0.88rem;
		font-weight: 600;
	}

	.back:hover {
		color: var(--color-text);
	}
</style>
