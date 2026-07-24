import type { Edge, NodeInfo } from "./api";

export interface PositionedNode {
	id: string;
	x: number;
	y: number;
}

const REPULSION = 16000;
const SPRING = 0.02;
const REST_LENGTH = 180;
const CENTER_PULL = 0.01;
const ITERATIONS = 300;
const STEP = 0.02;

function hashAngle(id: string): number {
	let h = 0;
	for (let i = 0; i < id.length; i++) {
		h = (h * 31 + id.charCodeAt(i)) | 0;
	}
	return ((h % 360) + 360) % 360 * (Math.PI / 180);
}

// Deterministic force-directed layout: seeded from each id's hash (not
// Math.random) so the same node set settles into roughly the same shape
// across repeated calls, instead of jumping around on every poll refresh.
export function layoutForce(nodes: NodeInfo[], edges: Edge[], width: number, height: number, padding: number): PositionedNode[] {
	const cx = width / 2;
	const cy = height / 2;
	const radius = Math.min(width, height) / 3;

	const positions: PositionedNode[] = nodes.map((n) => {
		const angle = hashAngle(n.id);
		return { id: n.id, x: cx + radius * Math.cos(angle), y: cy + radius * Math.sin(angle) };
	});

	const index = new Map(positions.map((p, i) => [p.id, i]));
	const edgePairs: [number, number][] = [];
	for (const e of edges) {
		const a = index.get(e.a);
		const b = index.get(e.b);
		if (a !== undefined && b !== undefined && a !== b) {
			edgePairs.push([a, b]);
		}
	}

	for (let iter = 0; iter < ITERATIONS; iter++) {
		const fx = new Array(positions.length).fill(0);
		const fy = new Array(positions.length).fill(0);

		for (let i = 0; i < positions.length; i++) {
			for (let j = i + 1; j < positions.length; j++) {
				const dx = positions[i].x - positions[j].x;
				const dy = positions[i].y - positions[j].y;
				const distSq = Math.max(dx * dx + dy * dy, 1);
				const dist = Math.sqrt(distSq);
				const force = REPULSION / distSq;
				const ux = dx / dist;
				const uy = dy / dist;
				fx[i] += ux * force;
				fy[i] += uy * force;
				fx[j] -= ux * force;
				fy[j] -= uy * force;
			}
		}

		for (const [a, b] of edgePairs) {
			const dx = positions[b].x - positions[a].x;
			const dy = positions[b].y - positions[a].y;
			const dist = Math.max(Math.sqrt(dx * dx + dy * dy), 1);
			const force = (dist - REST_LENGTH) * SPRING;
			const ux = dx / dist;
			const uy = dy / dist;
			fx[a] += ux * force;
			fy[a] += uy * force;
			fx[b] -= ux * force;
			fy[b] -= uy * force;
		}

		for (let i = 0; i < positions.length; i++) {
			fx[i] += (cx - positions[i].x) * CENTER_PULL;
			fy[i] += (cy - positions[i].y) * CENTER_PULL;
		}

		for (let i = 0; i < positions.length; i++) {
			positions[i].x += fx[i] * STEP;
			positions[i].y += fy[i] * STEP;
		}
	}

	for (const p of positions) {
		p.x = Math.min(Math.max(p.x, padding), width - padding);
		p.y = Math.min(Math.max(p.y, padding), height - padding);
	}

	return positions;
}
