<script lang="ts">
	import Wordmark from '$lib/components/brand/Wordmark.svelte';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { theme, loadAppearance, clearAppearance } from '$lib/stores/theme';
	import { regional, loadRegional, clearRegional } from '$lib/stores/regional';
	import MySettingsDialog from './MySettingsDialog.svelte';
	import { formatDate } from '$lib/utils/fieldHelpers';
	import { currentUser } from '$lib/stores/user';
	import { session, currentCompany } from '$stores/session';
	import { api } from '$lib/services/api';
	import { t, MSG, MENU } from '$lib/services/i18n.svelte';
	import { toast } from '$lib/stores/toast';
	import { companySwitchOpen } from '$lib/stores/companySwitch';
	import { getShortcutKey } from '$lib/utils/shortcuts';
	import CompanySwitchDialog from './CompanySwitchDialog.svelte';
	import AppearanceDialog from './AppearanceDialog.svelte';
	import { openShortcutHelp as openShortcutWindow } from '$lib/utils/shortcutHelp';
	import { companyLabel, companyLabelFor, type CompanyInfo } from '$lib/utils/company';

	// Ctrl+O opens the Switch Company dialog. Browsers let the page take
	// Ctrl+O (unlike Ctrl+N/T/W), so it never reaches the browser's Open File dialog.
	const SWITCH_COMPANY_SHORTCUT = 'Ctrl+O';

	let loading = $state(true);
	let currentTheme = $state<'light' | 'dark'>('light');
	let showUserMenu = $state(false);
	let showLanguageMenu = $state(false);
	let showCompanyMenu = $state(false);
	let languages = $state<{ code: string; name: string }[]>([]);
	let companies = $state<CompanyInfo[]>([]);
	let currentLanguage = $state('en-US');
	let currentCompanyName = $state('');
	let changingLanguage = $state(false);
	let changingCompany = $state(false);
	let version = $state('...');
	let showCompanyDialog = $state(false);
	let showAppearanceDialog = $state(false);
	let showMySettingsDialog = $state(false);
	let focusBeforeDialog: HTMLElement | null = null;

	$effect(() => {
		companySwitchOpen.set(showCompanyDialog);
	});

	theme.subscribe((value) => {
		currentTheme = value;
	});

	// Subscribe to session for language and company updates
	session.subscribe((sess) => {
		if (sess.language) {
			currentLanguage = sess.language;
		}
		if (sess.company) {
			currentCompanyName = sess.company;
		}
	});

	// Track previous user ID to detect login/logout changes
	let prevUserID: string | null = null;

	// Reload companies when user changes (login/logout/switch)
	$effect(() => {
		const user = $currentUser;
		const userID = user?.user_id ?? null;
		if (userID !== prevUserID) {
			prevUserID = userID;
			if (userID) {
				// User just logged in - reload companies (filtered by access) and the user's
				// appearance (themes, mode)
				api.getCompanies().then(comps => { companies = comps; }).catch(() => {});
				loadAppearance().catch(() => {});
				loadRegional().catch(() => {});
			} else {
				clearAppearance();
				clearRegional();
			}
		}
	});

	// The current company is not among the loaded companies (renamed while working in it):
	// reload them for its display name — once per name, it may also be one without access
	let companiesReloadedFor = '';
	$effect(() => {
		const name = $currentCompany;
		if (!name || companies.length === 0 || companiesReloadedFor === name) return;
		if (!companies.some((c) => c.name === name)) {
			companiesReloadedFor = name;
			api.getCompanies().then(comps => { companies = comps; }).catch(() => {});
		}
	});

	onMount(async () => {
		try {
			// Load current user info from storage
			currentUser.loadFromStorage();
			// Load available languages, companies and version in parallel
			const [langs, comps, ver] = await Promise.all([
				api.getLanguages(),
				api.getCompanies(),
				api.getVersion()
			]);
			languages = langs;
			companies = comps;
			version = ver;
		} catch (err) {
			console.error('Error loading data:', err);
		} finally {
			loading = false;
		}
	});

	function toggleTheme() {
		theme.toggle();
	}

	async function handleLogout() {
		showUserMenu = false;
		try {
			await api.logout();
			currentUser.clear();
			goto('/login');
		} catch (err) {
			console.error('Logout error:', err);
			// Even if API call fails, remove local data and redirect
			currentUser.clear();
			goto('/login');
		}
	}

	function toggleUserMenu() {
		showUserMenu = !showUserMenu;
		showLanguageMenu = false;
		showCompanyMenu = false;
	}

	function toggleLanguageMenu() {
		showLanguageMenu = !showLanguageMenu;
		showCompanyMenu = false;
	}

	function toggleCompanyMenu() {
		showCompanyMenu = !showCompanyMenu;
		showLanguageMenu = false;
	}

	async function handleLanguageChange(langCode: string) {
		if (langCode === currentLanguage || changingLanguage) return;

		changingLanguage = true;
		try {
			await api.setLanguage(langCode, true);
			currentLanguage = langCode;
			session.setLanguage(langCode);
			showLanguageMenu = false;
			showUserMenu = false;

			// Reload the page to apply new translations
			window.location.reload();
		} catch (err) {
			console.error('Error changing language:', err);
		} finally {
			changingLanguage = false;
		}
	}

	async function handleCompanyChange(companyName: string) {
		if (companyName === currentCompanyName || changingCompany) return;

		changingCompany = true;
		try {
			await api.setCompany(companyName);
			currentCompanyName = companyName;
			session.setCompany(companyName);
			showCompanyMenu = false;
			showUserMenu = false;

			// Land on the main menu of the new company (Classic closed all windows,
			// goes to the start page): the open record may not exist there. Full load,
			// so every page and store starts fresh in the new company.
			window.location.href = '/';
		} catch (err) {
			console.error('Error changing company:', err);
			toast.error(err instanceof Error ? err.message : String(err));
		} finally {
			changingCompany = false;
		}
	}

	// Keyboard Shortcuts help opens as a detached window (also linked from the home page)
	function openShortcutHelp() {
		showUserMenu = false;
		showLanguageMenu = false;
		showCompanyMenu = false;
		openShortcutWindow();
	}

	function openCompanyDialog() {
		showUserMenu = false;
		showLanguageMenu = false;
		showCompanyMenu = false;
		focusBeforeDialog = document.activeElement instanceof HTMLElement ? document.activeElement : null;
		showCompanyDialog = true;
	}

	function closeCompanyDialog() {
		showCompanyDialog = false;
		// Back to where the user was (e.g. the list cell), if it still exists
		if (focusBeforeDialog?.isConnected) focusBeforeDialog.focus({ preventScroll: true });
		focusBeforeDialog = null;
	}

	// Ctrl+O anywhere (menu, list, card, inside a cell) opens Switch Company. Capture
	// phase, so it runs before page handlers and the browser's Open File dialog.
	function handleGlobalKeydown(event: KeyboardEvent) {
		if (getShortcutKey(event) !== SWITCH_COMPANY_SHORTCUT || !$currentUser) return;
		event.preventDefault();
		event.stopPropagation();
		if (!showCompanyDialog && !changingCompany) openCompanyDialog();
	}

	onMount(() => {
		window.addEventListener('keydown', handleGlobalKeydown, true);
		return () => window.removeEventListener('keydown', handleGlobalKeydown, true);
	});

	// Close dropdown when clicking outside
	function handleClickOutside(event: MouseEvent) {
		const target = event.target as HTMLElement;
		const userMenuButton = target.closest('[data-user-menu]');
		const userMenuDropdown = target.closest('[data-user-dropdown]');

		if (!userMenuButton && !userMenuDropdown && showUserMenu) {
			showUserMenu = false;
			showLanguageMenu = false;
			showCompanyMenu = false;
		}
	}

	onMount(() => {
		document.addEventListener('click', handleClickOutside);
		return () => {
			document.removeEventListener('click', handleClickOutside);
		};
	});

	// Get language display name
	function getLanguageName(code: string): string {
		const lang = languages.find((l) => l.code === code);
		return lang?.name || code;
	}

	// Get short language code for display (e.g., "EN" from "en-US")
	function getShortCode(code: string): string {
		return code.split('-')[0].toUpperCase();
	}
</script>

{#if loading}
	<div class="menu-bar bg-appbar text-appbar-fg">
		<div class="px-4 py-2 text-sm">{t(MSG.LOADING)}</div>
	</div>
{:else}
	<nav class="menu-bar bg-appbar text-appbar-fg">
		<div class="flex items-center gap-2 w-full">
			<!-- The wordmark: the way home (the main menu) -->
			<a
				href="/"
				class="flex items-center px-4 py-2 hover:bg-appbar-fg/10 rounded transition-colors border-r border-appbar-fg/20 mr-2"
				title={t(MENU.HOME)}
				aria-label={t(MENU.HOME)}
			>
				<Wordmark variant="offwhite" height="1.25rem" alt="" />
			</a>

			<!-- Session information -->
			<div class="flex items-center gap-4 text-sm text-appbar-fg/80 border-r border-appbar-fg/20 pr-4 mr-2">
				{#if $currentCompany}
					<span class="font-medium">{companyLabelFor(companies, $currentCompany)}</span>
				{/if}
				{#if $session.userName}
					<span>User: {$session.userName}</span>
				{/if}
				{#if $session.language}
					<span>Language: {$session.language}</span>
				{/if}
				<!-- the work date when it is not today (My Settings): new documents get it -->
				{#if $regional.work_date && $regional.work_date !== $regional.today}
					<button type="button" class="work-date" title={t(MENU.MY_SETTINGS)} onclick={() => (showMySettingsDialog = true)}>
						{t(MENU.WORK_DATE)}: {formatDate($regional.work_date, $session.language)}
					</button>
				{/if}
			</div>

			<!-- Spacer to push user menu to the right -->
			<div class="flex-1"></div>

			<!-- User menu -->
			{#if $currentUser}
				<div class="relative border-l border-appbar-fg/20 pl-3 ml-2">
					<button
						data-user-menu
						onclick={toggleUserMenu}
						class="flex items-center gap-2 px-3 py-2 hover:bg-appbar-fg/10 rounded transition-colors"
						title={t(MENU.USER_MENU)}
					>
						<svg
							xmlns="http://www.w3.org/2000/svg"
							class="h-5 w-5"
							fill="none"
							viewBox="0 0 24 24"
							stroke="currentColor"
						>
							<path
								stroke-linecap="round"
								stroke-linejoin="round"
								stroke-width="2"
								d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
							/>
						</svg>
						<span class="text-sm font-medium">{$currentUser.user_name || $currentUser.user_id}</span>
						<svg
							xmlns="http://www.w3.org/2000/svg"
							class="h-4 w-4"
							fill="none"
							viewBox="0 0 24 24"
							stroke="currentColor"
						>
							<path
								stroke-linecap="round"
								stroke-linejoin="round"
								stroke-width="2"
								d="M19 9l-7 7-7-7"
							/>
						</svg>
					</button>

					<!-- User dropdown menu -->
					{#if showUserMenu}
						<div data-user-dropdown class="absolute right-0 mt-2 w-56 bg-surface dark:bg-gray-800 rounded-md shadow-lg border border-gray-200 dark:border-gray-700 z-50">
							<div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
								<p class="text-sm font-medium text-gray-900 dark:text-white">{$currentUser.user_name}</p>
								<p class="text-xs text-gray-500 dark:text-gray-400">{$currentUser.email || $currentUser.user_id}</p>
							</div>
							<div class="py-1">
								<!-- Language selector -->
								<div class="relative">
									<button
										onclick={toggleLanguageMenu}
										class="w-full text-left px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center justify-between"
									>
										<span class="flex items-center gap-2">
											<svg
												xmlns="http://www.w3.org/2000/svg"
												class="h-4 w-4"
												fill="none"
												viewBox="0 0 24 24"
												stroke="currentColor"
											>
												<path
													stroke-linecap="round"
													stroke-linejoin="round"
													stroke-width="2"
													d="M3 5h12M9 3v2m1.048 9.5A18.022 18.022 0 016.412 9m6.088 9h7M11 21l5-10 5 10M12.751 5C11.783 10.77 8.07 15.61 3 18.129"
												/>
											</svg>
											{t(MENU.LANGUAGE)}
										</span>
										<span class="flex items-center gap-1 text-xs text-gray-500 dark:text-gray-400">
											{getShortCode(currentLanguage)}
											<svg
												xmlns="http://www.w3.org/2000/svg"
												class="h-3 w-3"
												fill="none"
												viewBox="0 0 24 24"
												stroke="currentColor"
											>
												<path
													stroke-linecap="round"
													stroke-linejoin="round"
													stroke-width="2"
													d="M9 5l7 7-7 7"
												/>
											</svg>
										</span>
									</button>

									<!-- Language submenu -->
									{#if showLanguageMenu}
										<div class="absolute right-full top-0 mr-1 w-48 bg-surface dark:bg-gray-800 rounded-md shadow-lg border border-gray-200 dark:border-gray-700">
											{#each languages as lang}
												<button
													onclick={() => handleLanguageChange(lang.code)}
													disabled={changingLanguage}
													class="w-full text-left px-4 py-2 text-sm hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center justify-between disabled:opacity-50"
													class:text-blue-600={lang.code === currentLanguage}
													class:dark:text-blue-400={lang.code === currentLanguage}
													class:font-medium={lang.code === currentLanguage}
													class:text-gray-700={lang.code !== currentLanguage}
													class:dark:text-gray-300={lang.code !== currentLanguage}
												>
													{lang.name}
													{#if lang.code === currentLanguage}
														<svg
															xmlns="http://www.w3.org/2000/svg"
															class="h-4 w-4"
															fill="none"
															viewBox="0 0 24 24"
															stroke="currentColor"
														>
															<path
																stroke-linecap="round"
																stroke-linejoin="round"
																stroke-width="2"
																d="M5 13l4 4L19 7"
															/>
														</svg>
													{/if}
												</button>
											{/each}
										</div>
									{/if}
								</div>

								<!-- Company selector -->
								{#if companies.length > 1}
									<div class="relative">
										<button
											onclick={toggleCompanyMenu}
											class="w-full text-left px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center justify-between"
										>
											<span class="flex items-center gap-2">
												<svg
													xmlns="http://www.w3.org/2000/svg"
													class="h-4 w-4"
													fill="none"
													viewBox="0 0 24 24"
													stroke="currentColor"
												>
													<path
														stroke-linecap="round"
														stroke-linejoin="round"
														stroke-width="2"
														d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
													/>
												</svg>
												{t(MENU.COMPANY)}
												<span class="text-xs text-gray-500 dark:text-gray-400">({SWITCH_COMPANY_SHORTCUT})</span>
											</span>
											<span class="flex items-center gap-1 text-xs text-gray-500 dark:text-gray-400">
												{currentCompanyName ? companyLabelFor(companies, currentCompanyName) : '—'}
												<svg
													xmlns="http://www.w3.org/2000/svg"
													class="h-3 w-3"
													fill="none"
													viewBox="0 0 24 24"
													stroke="currentColor"
												>
													<path
														stroke-linecap="round"
														stroke-linejoin="round"
														stroke-width="2"
														d="M9 5l7 7-7 7"
													/>
												</svg>
											</span>
										</button>

										<!-- Company submenu -->
										{#if showCompanyMenu}
											<div class="absolute right-full top-0 mr-1 w-48 bg-surface dark:bg-gray-800 rounded-md shadow-lg border border-gray-200 dark:border-gray-700 max-h-64 overflow-y-auto">
												{#each companies as company (company.name)}
													<button
														onclick={() => handleCompanyChange(company.name)}
														disabled={changingCompany}
														class="w-full text-left px-4 py-2 text-sm hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center justify-between disabled:opacity-50"
														class:text-blue-600={company.name === currentCompanyName}
														class:dark:text-blue-400={company.name === currentCompanyName}
														class:font-medium={company.name === currentCompanyName}
														class:text-gray-700={company.name !== currentCompanyName}
														class:dark:text-gray-300={company.name !== currentCompanyName}
													>
														{companyLabel(company)}
														{#if company.name === currentCompanyName}
															<svg
																xmlns="http://www.w3.org/2000/svg"
																class="h-4 w-4"
																fill="none"
																viewBox="0 0 24 24"
																stroke="currentColor"
															>
																<path
																	stroke-linecap="round"
																	stroke-linejoin="round"
																	stroke-width="2"
																	d="M5 13l4 4L19 7"
																/>
															</svg>
														{/if}
													</button>
												{/each}
											</div>
										{/if}
									</div>
								{/if}

								<!-- My Settings: date and number format, work date -->
								<button
									onclick={() => { showUserMenu = false; showMySettingsDialog = true; }}
									class="w-full text-left px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
									data-my-settings-menu
								>
									<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
									</svg>
									{t(MENU.MY_SETTINGS)}
								</button>

								<!-- Appearance: themes for light and dark mode -->
								<button
									onclick={() => { showUserMenu = false; showAppearanceDialog = true; }}
									class="w-full text-left px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
									data-appearance-menu
								>
									<svg
										xmlns="http://www.w3.org/2000/svg"
										class="h-4 w-4"
										fill="none"
										viewBox="0 0 24 24"
										stroke="currentColor"
									>
										<path
											stroke-linecap="round"
											stroke-linejoin="round"
											stroke-width="2"
											d="M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01"
										/>
									</svg>
									{t(MENU.APPEARANCE)}
								</button>

								<!-- Keyboard shortcuts help (detached window) -->
								<button
									onclick={openShortcutHelp}
									class="w-full text-left px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
								>
									<svg
										xmlns="http://www.w3.org/2000/svg"
										class="h-4 w-4"
										fill="none"
										viewBox="0 0 24 24"
										stroke="currentColor"
									>
										<rect x="2" y="6" width="20" height="12" rx="2" stroke-width="2" />
										<path
											stroke-linecap="round"
											stroke-linejoin="round"
											stroke-width="2"
											d="M6 10h.01M10 10h.01M14 10h.01M18 10h.01M8 14h8"
										/>
									</svg>
									{t(MENU.SHORTCUTS)}
								</button>

								<!-- Divider -->
								<div class="border-t border-gray-200 dark:border-gray-700 my-1"></div>

								<!-- Sign out -->
								<button
									onclick={handleLogout}
									class="w-full text-left px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
								>
									<svg
										xmlns="http://www.w3.org/2000/svg"
										class="h-4 w-4"
										fill="none"
										viewBox="0 0 24 24"
										stroke="currentColor"
									>
										<path
											stroke-linecap="round"
											stroke-linejoin="round"
											stroke-width="2"
											d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"
										/>
									</svg>
									{t(MENU.SIGN_OUT)}
								</button>
							</div>
						</div>
					{/if}
				</div>
			{/if}

			<!-- Theme toggle button -->
			<button
				onclick={toggleTheme}
				class="flex items-center gap-2 px-3 py-2 hover:bg-appbar-fg/10 rounded transition-colors"
				title={currentTheme === 'light' ? 'Switch to dark mode' : 'Switch to light mode'}
				aria-label="Toggle theme"
			>
				{#if currentTheme === 'light'}
					<!-- Moon icon for dark mode -->
					<svg
						xmlns="http://www.w3.org/2000/svg"
						class="h-5 w-5"
						fill="none"
						viewBox="0 0 24 24"
						stroke="currentColor"
					>
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"
						/>
					</svg>
				{:else}
					<!-- Sun icon for light mode -->
					<svg
						xmlns="http://www.w3.org/2000/svg"
						class="h-5 w-5"
						fill="none"
						viewBox="0 0 24 24"
						stroke="currentColor"
					>
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"
						/>
					</svg>
				{/if}
			</button>

			<!-- Version -->
			<span class="text-xs text-appbar-fg/70 pl-2 border-l border-appbar-fg/20">v{version}</span>
		</div>
	</nav>
{/if}

{#if showMySettingsDialog}
	<MySettingsDialog onclose={() => (showMySettingsDialog = false)} />
{/if}

{#if showAppearanceDialog}
	<AppearanceDialog onclose={() => (showAppearanceDialog = false)} />
{/if}

{#if showCompanyDialog}
	<CompanySwitchDialog
		{companies}
		current={currentCompanyName}
		switching={changingCompany}
		onselect={handleCompanyChange}
		onclose={closeCompanyDialog}
	/>
{/if}

<style>
	.menu-bar {
		@apply flex items-center px-4 py-2 shadow-md;
		min-height: 3rem;
	}
	/* the work date when it is not today: a reminder (opens My Settings) */
	.work-date {
		@apply px-2 py-0.5 rounded font-semibold;
		background: rgb(var(--accent-600));
		color: rgb(var(--surface));
	}
</style>
