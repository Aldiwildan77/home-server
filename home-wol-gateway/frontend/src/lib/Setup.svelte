<script lang="ts">
	import { checkHealth } from "./api";
	import { gateway } from "./gateway.svelte";

	let { onConnected }: { onConnected: () => void } = $props();

	let address = $state("");
	let token = $state("");
	let checking = $state(false);
	let error = $state("");

	async function connect(event: SubmitEvent) {
		event.preventDefault();

		const url = address.trim().replace(/\/+$/, "");
		if (!url) {
			error = "Enter an address first.";
			return;
		}
		if (!token.trim()) {
			error = "Enter the API token too.";
			return;
		}

		checking = true;
		error = "";

		const reachable = await checkHealth(url, token.trim());

		checking = false;

		if (!reachable) {
			error = "Can't reach that address, or the token's wrong.";
			return;
		}

		gateway.set(url, token);
		onConnected();
	}
</script>

<div class="wrap">
	<div class="card">
		<h1>Wake on Lan</h1>
		<p class="lead">Point this at your raspi's gateway to get started.</p>

		<form onsubmit={connect}>
			<label for="address">Gateway address</label>
			<input
				id="address"
				type="text"
				placeholder="http://192.168.1.10:8080"
				bind:value={address}
				disabled={checking}
				autocomplete="off"
			/>

			<label for="token" class="token-label">API token</label>
			<input
				id="token"
				type="password"
				placeholder="security.api_token from its config"
				bind:value={token}
				disabled={checking}
				autocomplete="off"
			/>

			{#if error}
				<p class="error">{error}</p>
			{/if}

			<button type="submit" disabled={checking}>
				{checking ? "Connecting…" : "Connect"}
			</button>
		</form>
	</div>
</div>

<style>
	.wrap {
		min-height: 100vh;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 24px;
	}

	.card {
		width: 100%;
		max-width: 380px;
		background: var(--color-bg);
		border-radius: var(--radius-lg);
		box-shadow: var(--shadow-md);
		padding: 40px 32px;
	}

	h1 {
		font-size: 1.5rem;
		color: var(--color-text);
	}

	.lead {
		margin-top: 8px;
		color: var(--color-text-muted);
		font-size: 0.95rem;
	}

	form {
		margin-top: 28px;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	label {
		font-size: 0.85rem;
		font-weight: 600;
		color: var(--color-text);
	}

	.token-label {
		margin-top: 8px;
	}

	input {
		padding: 12px 14px;
		border: none;
		border-radius: var(--radius-sm);
		background: var(--color-surface);
		font-size: 0.95rem;
		color: var(--color-text);
	}

	input::placeholder {
		color: var(--color-text-muted);
	}

	.error {
		color: var(--color-danger);
		font-size: 0.85rem;
	}

	button {
		margin-top: 12px;
		padding: 12px 16px;
		border: none;
		border-radius: var(--radius-sm);
		background: var(--color-primary);
		color: #ffffff;
		font-size: 0.95rem;
		font-weight: 600;
		transition: background-color 0.15s ease, box-shadow 0.15s ease;
	}

	button:hover:not(:disabled) {
		background: var(--color-primary-hover);
		box-shadow: var(--glow-primary);
	}
</style>
