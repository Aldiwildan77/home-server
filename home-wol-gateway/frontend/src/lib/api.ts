import { gateway } from "./gateway.svelte";

export interface Device {
	mac: string;
	ip: string;
	hostname?: string;
	online: boolean;
	node_id: string;
	wol_allowed: boolean;
}

export interface NodeInfo {
	id: string;
	http_addr?: string;
}

export interface Edge {
	a: string;
	b: string;
}

export interface Topology {
	nodes: NodeInfo[];
	edges: Edge[];
}

export class ApiError extends Error {}

async function request<T>(baseUrl: string, token: string, path: string, init?: RequestInit): Promise<T> {
	const res = await fetch(`${baseUrl}${path}`, {
		headers: {
			"Content-Type": "application/json",
			Authorization: `Bearer ${token}`,
		},
		...init,
	});

	if (!res.ok) {
		const text = await res.text();
		throw new ApiError(text || `request failed: ${res.status}`);
	}

	if (res.status === 204) {
		return undefined as T;
	}

	return res.json();
}

function requireGateway(): { url: string; token: string } {
	if (!gateway.url) {
		throw new ApiError("gateway not configured");
	}
	return { url: gateway.url, token: gateway.token };
}

export function getInventory(): Promise<Device[]> {
	const { url, token } = requireGateway();
	return request(url, token, "/inventory");
}

export function getTopology(): Promise<Topology> {
	const { url, token } = requireGateway();
	return request(url, token, "/topology");
}

export function setAllowed(mac: string, allow: boolean): Promise<void> {
	const { url, token } = requireGateway();
	return request(url, token, `/devices/${encodeURIComponent(mac)}/allow`, {
		method: "POST",
		body: JSON.stringify({ allow }),
	});
}

export function wake(mac: string): Promise<void> {
	const { url, token } = requireGateway();
	return request(url, token, "/wake", {
		method: "POST",
		body: JSON.stringify({ mac }),
	});
}

export async function checkHealth(url: string, token: string): Promise<boolean> {
	try {
		const res = await fetch(`${url}/healthz`, {
			headers: { Authorization: `Bearer ${token}` },
		});
		return res.ok;
	} catch {
		return false;
	}
}
