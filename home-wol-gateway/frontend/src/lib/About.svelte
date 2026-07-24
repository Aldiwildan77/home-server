<script lang="ts">
	let { onDone }: { onDone: () => void } = $props();
</script>

<div class="panel">
	<h2>About</h2>
	<p>
		A mesh of small Go nodes that gossip device inventory and route Wake-on-LAN requests to whichever
		node owns the target device — across as many subnets as you've got, with no fixed root or hierarchy.
	</p>
	<p>Every UDP message is signed and every request here is authenticated. This app is the control surface for that mesh.</p>

	<p class="author">
		Author: Muhammad Wildan Aldiansyah — <a href="https://aldiwildan.me" target="_blank" rel="noreferrer">aldiwildan.me</a>
	</p>

	<button type="button" class="back" onclick={onDone}>Back</button>
</div>

<style>
	.panel {
		background: var(--color-surface);
		border-radius: var(--radius-md);
		padding: 24px;
		max-width: 460px;
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	h2 {
		font-size: 1.05rem;
	}

	p {
		color: var(--color-text-muted);
		font-size: 0.88rem;
		line-height: 1.5;
	}

	.author {
		font-size: 0.8rem;
	}

	.author a {
		color: var(--color-primary);
		font-weight: 600;
		text-decoration: none;
	}

	.author a:hover {
		text-decoration: underline;
	}

	.back {
		align-self: flex-start;
		margin-top: 8px;
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
