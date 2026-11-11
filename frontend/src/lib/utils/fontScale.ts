/**
 * The interface's font scale: the root font size against the 16px the page definitions' widths
 * are made for (Appearance's font size sets --font-scale on <html>, the browser's own default
 * size counts too); 1 when it cannot be read
 */
export function rootFontScale(root: HTMLElement | undefined = globalThis.document?.documentElement): number {
	if (!root) return 1;
	const px = parseFloat(getComputedStyle(root).fontSize);
	return px > 0 ? px / 16 : 1;
}

/** A width of a page definition (px at 100 %) at the font scale, whole pixels */
export function scaledWidth(width: number, scale: number): number {
	return Math.round(width * (scale > 0 ? scale : 1));
}

/**
 * Calls onchange when the appearance changes the root element (its font scale and font
 * classes); returns the function that stops watching
 */
export function watchRootLayout(onchange: () => void, root: HTMLElement | undefined = globalThis.document?.documentElement): () => void {
	if (!root || typeof MutationObserver === 'undefined') return () => {};
	const observer = new MutationObserver(onchange);
	observer.observe(root, { attributes: true, attributeFilter: ['style', 'class'] });
	return () => observer.disconnect();
}
