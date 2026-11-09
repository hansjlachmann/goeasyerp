<script lang="ts">
	import { onMount } from 'svelte';
	import Modal from '$components/Modal.svelte';
	import { api } from '$lib/services/api';
	import { t, APPEARANCE } from '$lib/services/i18n.svelte';
	import { toast } from '$lib/stores/toast';
	import { confirm } from '$lib/stores/confirm';
	import { appearance, chooseTheme, loadAppearance, saveAppearance } from '$lib/stores/theme';
	import { FONT_SIZES, FONTS, themeAssetUrl, type AppearanceSettings, type Theme, type ThemeList } from '$lib/theme/apply';

	/** Gallery of community themes (opens in a new tab; the user copies a repository URL from it) */
	const OMARCHY_THEMES_URL = 'https://omarchy.org/themes/';

	/** Caption keys of the fonts (messages.yaml) */
	const fontCaption: Record<string, string> = {
		default: APPEARANCE.FONT_DEFAULT,
		system: APPEARANCE.FONT_SYSTEM,
		inter: 'Inter',
		ibm_plex_sans: 'IBM Plex Sans',
		serif: APPEARANCE.FONT_SERIF,
		monospace: APPEARANCE.FONT_MONOSPACE
	};

	interface Props {
		onclose: () => void;
	}

	let { onclose }: Props = $props();

	let list = $state<ThemeList>({ themes: [], can_install: false, can_refresh: false, can_delete: false });
	let busy = $state(false);
	let installUrl = $state('');
	let installing = $state(false);

	const themes = $derived(list.themes);

	const lightThemes = $derived(themes.filter((th) => th.mode === 'light'));
	const darkThemes = $derived(themes.filter((th) => th.mode === 'dark'));
	const settings = $derived($appearance?.settings);
	/** The theme in use now (the slot of the current mode) — its wallpapers can be chosen */
	const activeTheme = $derived($appearance ? ($appearance.settings.mode === 'light' ? $appearance.light : $appearance.dark) : null);
	const wallpaperCount = $derived(activeTheme?.source === 'github' ? activeTheme.wallpapers : 0);

	async function save(patch: Partial<AppearanceSettings>) {
		try {
			await saveAppearance(patch);
		} catch (err: any) {
			toast.error(err?.message ?? String(err));
		}
	}

	// Sliders: the change shows at once (CSS variables), the save follows when the slider rests
	let sliderTimer: ReturnType<typeof setTimeout> | undefined;
	function slide(field: 'wallpaper_opacity' | 'wallpaper_blur' | 'panel_opacity', value: number) {
		const html = document.documentElement;
		if (field === 'wallpaper_opacity') html.style.setProperty('--wp-opacity', String(value / 100));
		if (field === 'wallpaper_blur') html.style.setProperty('--wp-blur', `${value}px`);
		if (field === 'panel_opacity') html.style.setProperty('--panel-alpha', String(value / 100));
		clearTimeout(sliderTimer);
		sliderTimer = setTimeout(() => save({ [field]: value }), 400);
	}

	async function reload() {
		try {
			list = await api.getThemes();
		} catch (err: any) {
			toast.error(err?.message ?? String(err));
		}
	}

	onMount(reload);

	async function install(event: SubmitEvent) {
		event.preventDefault();
		if (installing || !installUrl.trim()) return;
		installing = true;
		try {
			const res = await api.installTheme(installUrl.trim());
			toast.success(t(res.unchanged ? APPEARANCE.UP_TO_DATE : APPEARANCE.INSTALLED, res.name));
			installUrl = '';
			await reload();
		} catch (err: any) {
			toast.error(err?.message ?? String(err));
		} finally {
			installing = false;
		}
	}

	async function refresh(th: Theme) {
		if (busy) return;
		busy = true;
		try {
			const res = await api.refreshTheme(th.code);
			toast.success(t(res.unchanged ? APPEARANCE.UP_TO_DATE : APPEARANCE.UPDATED, res.name));
			await reload();
			await loadAppearance();
		} catch (err: any) {
			toast.error(err?.message ?? String(err));
		} finally {
			busy = false;
		}
	}

	function remove(th: Theme) {
		confirm.show(t(APPEARANCE.REMOVE), t(APPEARANCE.REMOVE_CONFIRM, th.name), async () => {
			try {
				await api.deleteTheme(th.code);
				await reload();
				await loadAppearance();
			} catch (err: any) {
				toast.error(err?.message ?? String(err));
			}
		}, { confirmText: t(APPEARANCE.REMOVE), variant: 'danger' });
	}

	function inUse(th: Theme): boolean {
		if (!settings) return false;
		return th.mode === 'light' ? settings.light_theme === th.code : settings.dark_theme === th.code;
	}

	async function pick(th: Theme) {
		if (busy) return;
		busy = true;
		try {
			await chooseTheme(th);
		} catch (err: any) {
			toast.error(err?.message ?? String(err));
		} finally {
			busy = false;
		}
	}

	/** Preview colors of a theme card: the theme's own tokens, never the app's variables */
	function preview(th: Theme) {
		const v = th.tokens.vars;
		const g = th.tokens.gray;
		const a = th.tokens.accent;
		const dark = th.mode === 'dark';
		return {
			page: v['page-bg'],
			panel: dark ? g[8] : v['surface'],
			border: dark ? g[7] : g[2],
			appbar: v['appbar'],
			appbarFg: v['appbar-fg'],
			text: v['fg'],
			secondary: dark ? g[4] : g[6],
			accent: dark ? a[4] : a[6],
			selected: v['row-selected']
		};
	}
</script>

{#snippet themeCard(th: Theme)}
	{@const c = preview(th)}
	<button
		type="button"
		class="theme-card"
		class:in-use={inUse(th)}
		aria-pressed={inUse(th)}
		disabled={busy}
		onclick={() => pick(th)}
		data-theme-code={th.code}
	>
		<div class="theme-preview" style:background-color={c.page} aria-hidden="true">
			<div class="pv-appbar" style:background-color={c.appbar}>
				<span style:background-color={c.appbarFg}></span>
			</div>
			<div class="pv-panel" style:background-color={c.panel} style:border-color={c.border}>
				<div class="pv-title" style:background-color={c.accent}></div>
				<div class="pv-row" style:background-color={c.text}></div>
				<div class="pv-row pv-selected" style:background-color={c.selected}>
					<span style:background-color={c.text}></span>
				</div>
				<div class="pv-row short" style:background-color={c.secondary}></div>
			</div>
		</div>
		<div class="theme-name">
			<span>{th.name}</span>
			{#if inUse(th)}
				<span class="in-use-badge">✓ {t(APPEARANCE.IN_USE)}</span>
			{/if}
		</div>
		{#if th.source === 'github'}
			<div class="theme-source">{t(APPEARANCE.FROM, th.author ?? '')}</div>
		{/if}
	</button>
	{#if th.source === 'github' && (list.can_refresh || list.can_delete)}
		<div class="theme-actions">
			{#if list.can_refresh}
				<button type="button" class="link-btn" disabled={busy} onclick={() => refresh(th)}>{t(APPEARANCE.REFRESH)}</button>
			{/if}
			{#if list.can_delete}
				<button type="button" class="link-btn danger" disabled={busy} onclick={() => remove(th)}>{t(APPEARANCE.REMOVE)}</button>
			{/if}
		</div>
	{/if}
{/snippet}

<Modal open={true} {onclose} size="expanded" title={t(APPEARANCE.TITLE)}>
	<div class="appearance-dialog">
		<div class="dialog-header">
			<h2 id="modal-title" class="dialog-title">{t(APPEARANCE.TITLE)}</h2>
			<button type="button" class="close-btn" onclick={onclose} aria-label={t(APPEARANCE.CLOSE)}>×</button>
		</div>
		<p class="hint">{t(APPEARANCE.HINT)}</p>

		{#if list.can_install}
			<form class="install-box" onsubmit={install}>
				<label for="theme-install-url" class="section-title">{t(APPEARANCE.INSTALL_TITLE)}</label>
				<div class="install-row">
					<input
						id="theme-install-url"
						type="url"
						class="install-input"
						placeholder="https://github.com/owner/omarchy-…-theme"
						autocomplete="off"
						spellcheck="false"
						maxlength="300"
						bind:value={installUrl}
						disabled={installing}
					/>
					<button type="submit" class="install-btn" disabled={installing || !installUrl.trim()}>
						{installing ? t(APPEARANCE.INSTALLING) : t(APPEARANCE.INSTALL)}
					</button>
				</div>
				<p class="install-hint">{t(APPEARANCE.INSTALL_HINT)}</p>
				<a class="install-link" href={OMARCHY_THEMES_URL} target="_blank" rel="noopener noreferrer">{t(APPEARANCE.BROWSE_THEMES)} ↗</a>
			</form>
		{/if}

		{#if settings}
			<section class="theme-section text-section">
				<h3 class="section-title">{t(APPEARANCE.TEXT)}</h3>
				<div class="text-controls">
					<label>
						<span>{t(APPEARANCE.FONT)}</span>
						<select value={settings.font} onchange={(e) => save({ font: e.currentTarget.value })} data-appearance-font>
							{#each FONTS as f (f)}
								<option value={f}>{fontCaption[f].startsWith('APPEARANCE_') ? t(fontCaption[f]) : fontCaption[f]}</option>
							{/each}
						</select>
					</label>
					<label>
						<span>{t(APPEARANCE.FONT_SIZE)}</span>
						<select value={String(settings.font_size)} onchange={(e) => save({ font_size: +e.currentTarget.value })} data-appearance-size>
							{#each FONT_SIZES as size (size)}
								<option value={String(size)}>{size === 0 ? t(APPEARANCE.FONT_DEFAULT) : `${size} %`}</option>
							{/each}
						</select>
					</label>
					<label>
						<span>{t(APPEARANCE.DENSITY)}</span>
						<select value={settings.density} onchange={(e) => save({ density: e.currentTarget.value as AppearanceSettings['density'] })} data-appearance-density>
							<option value="comfortable">{t(APPEARANCE.DENSITY_COMFORTABLE)}</option>
							<option value="compact">{t(APPEARANCE.DENSITY_COMPACT)}</option>
						</select>
					</label>
					<label>
						<span>{t(APPEARANCE.AMOUNT_FONT)}</span>
						<select value={settings.amount_font ?? 'text'} onchange={(e) => save({ amount_font: e.currentTarget.value as AppearanceSettings['amount_font'] })} data-appearance-amount-font>
							<option value="text">{t(APPEARANCE.AMOUNT_FONT_TEXT)}</option>
							<option value="monospace">{t(APPEARANCE.AMOUNT_FONT_MONOSPACE)}</option>
						</select>
					</label>
				</div>
			</section>
		{/if}

		{#if settings && activeTheme}
			<section class="theme-section wallpaper-section">
				<h3 class="section-title">{t(APPEARANCE.WALLPAPER)} <span class="current-mode">— {activeTheme.name}</span></h3>
				{#if wallpaperCount === 0}
					<p class="hint">{t(APPEARANCE.NO_WALLPAPERS)}</p>
				{:else}
					<div class="wallpaper-grid" role="radiogroup" aria-label={t(APPEARANCE.WALLPAPER)}>
						<button
							type="button"
							role="radio"
							aria-checked={!settings.wallpaper}
							class="wallpaper-option none"
							class:selected={!settings.wallpaper}
							onclick={() => save({ wallpaper: '' })}
						>
							{t(APPEARANCE.WALLPAPER_NONE)}
						</button>
						{#each Array.from({ length: wallpaperCount }, (_, i) => i + 1) as n (n)}
							<button
								type="button"
								role="radio"
								aria-checked={settings.wallpaper === `bg-${n}`}
								aria-label={`${t(APPEARANCE.WALLPAPER)} ${n}`}
								class="wallpaper-option"
								class:selected={settings.wallpaper === `bg-${n}`}
								onclick={() => save({ wallpaper: `bg-${n}` })}
								data-wallpaper={`bg-${n}`}
							>
								<img src={themeAssetUrl(activeTheme, `tb-${n}`)} alt="" loading="lazy" />
							</button>
						{/each}
					</div>
					{#if settings.wallpaper}
						<div class="sliders">
							<label>
								<span>{t(APPEARANCE.WALLPAPER_OPACITY)}</span>
								<input type="range" min="0" max="100" step="5" value={settings.wallpaper_opacity}
									oninput={(e) => slide('wallpaper_opacity', +e.currentTarget.value)} />
							</label>
							<label>
								<span>{t(APPEARANCE.WALLPAPER_BLUR)}</span>
								<input type="range" min="0" max="20" step="1" value={settings.wallpaper_blur}
									oninput={(e) => slide('wallpaper_blur', +e.currentTarget.value)} />
							</label>
							<label>
								<span>{t(APPEARANCE.PANEL_OPACITY)}</span>
								<input type="range" min="75" max="100" step="1" value={settings.panel_opacity}
									oninput={(e) => slide('panel_opacity', +e.currentTarget.value)} />
							</label>
						</div>
					{/if}
				{/if}
			</section>
		{/if}

		{#each [{ list: lightThemes, mode: 'light', title: APPEARANCE.LIGHT_THEMES }, { list: darkThemes, mode: 'dark', title: APPEARANCE.DARK_THEMES }] as section (section.mode)}
			<section class="theme-section" data-mode={section.mode}>
				<h3 class="section-title">
					{t(section.title)}
					{#if settings?.mode === section.mode}
						<span class="current-mode">({t(APPEARANCE.CURRENT_MODE)})</span>
					{/if}
				</h3>
				<div class="theme-grid">
					{#each section.list as th (th.code)}
						<div class="theme-cell">{@render themeCard(th)}</div>
					{/each}
				</div>
			</section>
		{/each}
	</div>
</Modal>

<style>
	.appearance-dialog {
		@apply p-6 max-h-[85vh] overflow-y-auto;
	}

	.dialog-header {
		@apply flex items-center justify-between mb-2;
	}

	.dialog-title {
		@apply text-xl font-bold text-heading dark:text-blue-400;
	}

	.close-btn {
		@apply text-2xl leading-none px-2 text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-50;
	}

	.hint {
		@apply text-sm text-gray-600 dark:text-gray-400 mb-4;
	}

	.section-title {
		@apply text-base font-semibold mb-2 text-fg;
	}

	.current-mode {
		@apply text-sm font-normal text-gray-500 dark:text-gray-400;
	}

	.theme-section {
		@apply mb-6;
	}

	.theme-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
		gap: 0.75rem;
	}

	.theme-card {
		@apply rounded-lg border-2 border-gray-200 dark:border-gray-700 p-2 text-left transition-colors;
		@apply hover:border-primary-500 focus-visible:border-primary-500;
	}

	.theme-card.in-use {
		@apply border-primary-600 dark:border-primary-400;
	}

	.theme-card:disabled {
		@apply cursor-wait;
	}

	.theme-preview {
		@apply rounded overflow-hidden h-24 flex flex-col;
	}

	.pv-appbar {
		@apply h-4 flex items-center px-2;
	}

	.pv-appbar span {
		@apply block h-1 w-8 rounded-sm opacity-80;
	}

	.pv-panel {
		@apply m-2 flex-1 rounded border p-2 flex flex-col gap-1.5;
	}

	.pv-title {
		@apply h-1.5 w-12 rounded-sm;
	}

	.pv-row {
		@apply h-1 w-full rounded-sm opacity-70;
	}

	.pv-row.short {
		@apply w-2/3;
	}

	.pv-selected {
		@apply h-2.5 opacity-100 flex items-center px-1;
	}

	.pv-selected span {
		@apply block h-1 w-3/4 rounded-sm opacity-70;
	}

	.theme-name {
		@apply mt-2 flex items-center justify-between text-sm text-fg;
	}

	.text-controls {
		@apply grid gap-3;
		grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
	}

	.text-controls label {
		@apply flex flex-col gap-1 text-sm text-gray-600 dark:text-gray-400;
	}

	.text-controls select {
		@apply px-2 py-1.5 text-sm rounded border border-gray-300 dark:border-gray-600 bg-surface dark:bg-gray-700 text-fg;
	}

	.wallpaper-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
		gap: 0.5rem;
	}

	.wallpaper-option {
		@apply h-20 rounded border-2 border-gray-200 dark:border-gray-700 overflow-hidden text-sm text-fg;
		@apply hover:border-primary-500 focus-visible:border-primary-500;
	}

	.wallpaper-option.selected {
		@apply border-primary-600 dark:border-primary-400;
	}

	.wallpaper-option img {
		@apply w-full h-full object-cover;
	}

	.sliders {
		@apply mt-3 grid gap-2;
		grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
	}

	.sliders label {
		@apply flex flex-col text-sm text-gray-600 dark:text-gray-400 gap-1;
	}

	.sliders input[type='range'] {
		accent-color: rgb(var(--accent-600));
	}

	.theme-source {
		@apply text-xs text-gray-500 dark:text-gray-400 truncate;
	}

	.theme-cell {
		@apply flex flex-col;
	}

	.theme-actions {
		@apply flex gap-3 px-1 pt-1;
	}

	.link-btn {
		@apply text-xs text-primary-600 dark:text-primary-400 hover:underline disabled:opacity-50;
	}

	.link-btn.danger {
		@apply text-red-700 dark:text-red-400;
	}

	.install-box {
		@apply mb-6 p-3 rounded-lg border border-gray-200 dark:border-gray-700;
	}

	.install-row {
		@apply flex gap-2;
	}

	.install-input {
		@apply flex-1 px-3 py-1.5 text-sm rounded border border-gray-300 dark:border-gray-600 bg-surface dark:bg-gray-700 text-fg;
	}

	.install-btn {
		@apply px-4 py-1.5 text-sm rounded bg-primary-600 text-white hover:bg-primary-700 disabled:opacity-50 dark:bg-primary-500 dark:hover:bg-primary-600;
	}

	.install-hint {
		@apply mt-2 text-xs text-gray-500 dark:text-gray-400;
	}

	.install-link {
		@apply inline-block mt-1 text-xs text-primary-600 dark:text-primary-400 hover:underline;
	}

	.in-use-badge {
		@apply text-xs font-medium text-primary-600 dark:text-primary-400;
	}
</style>
