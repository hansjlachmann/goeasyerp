<script lang="ts">
	// Page frames: the pages a page in a modal opens (the New of a lookup, a page run with
	// RunModal) open over it, each in its own modal on top — they are pages as in the browser,
	// whose navigation stays in the frames: a page they open goes on top, their record and
	// parameter changes stay in their frame, and closing one (Esc, the close button) goes back to
	// the page under it. Around the modal's own page:
	//
	//	<PageFrames><PageRenderer embedded … /></PageFrames>
	//
	// A card's subpages are inside page frames too: the pages a line opens (its order, its
	// reservation entries) open over the card, which stays as it is; onclosed tells when the
	// last of them is closed (the lines may have changed)
	import type { Snippet } from 'svelte';
	import Modal from '../Modal.svelte';
	import PageFrame from './PageFrame.svelte';
	import { pageNavigator, setPageNavigator, type PageNavigator } from '$lib/utils/pageNavigator';

	let { children, onclosed }: { children: Snippet; onclosed?: () => void } = $props();

	interface Frame {
		id: number;
		url: string;
	}
	let frames = $state<Frame[]>([]);
	let nextId = 0;

	function open(url: string) {
		frames.push({ id: nextId++, url });
	}

	// A frame's navigator: open on top, move and close in place (closing a frame twice — the
	// close button and the page's Esc — closes it once)
	function frameNavigator(id: number): PageNavigator {
		return {
			framed: true,
			open,
			move: (url) => {
				const frame = frames.find((f) => f.id === id);
				if (frame) frame.url = url;
			},
			close: () => {
				const i = frames.findIndex((f) => f.id === id);
				if (i < 0) return;
				frames.splice(i);
				if (frames.length === 0) onclosed?.();
			},
			replaceState: () => {},
			location: () => new URL(frames.find((f) => f.id === id)?.url ?? '/', window.location.origin)
		};
	}

	// The modal's own page opens its pages in the frames; the rest is as around the modal
	const outer = pageNavigator();
	setPageNavigator({ ...outer, open });
</script>

{@render children()}

{#each frames as frame (frame.id)}
	{@const nav = frameNavigator(frame.id)}
	<Modal open={true} size="fullscreen" closeOnEscape={false} onclose={() => nav.close()}>
		<div class="page-frame">
			<PageFrame url={frame.url} pageNav={nav} />
		</div>
	</Modal>
{/each}

<style>
	.page-frame {
		@apply flex min-h-0 flex-1 flex-col overflow-auto p-4;
		height: 85vh;
	}
</style>
