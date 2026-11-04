<script lang="ts">
	import Wordmark from '$lib/components/brand/Wordmark.svelte';
	import { goto } from '$app/navigation';
	import { api } from '$lib/services/api';
	import { t, MSG, ERR, LOGIN, PLC, loadTranslations } from '$lib/services/i18n.svelte';
	import { currentUser } from '$lib/stores/user';
	import { session } from '$stores/session';
	import { toast } from '$lib/stores/toast';
	import { onMount } from 'svelte';
	import { companyLabel, type CompanyInfo } from '$lib/utils/company';

	let userID = $state('');
	let password = $state('');
	let company = $state('');
	let companies = $state<CompanyInfo[]>([]);
	let error = $state('');
	let loading = $state(false);
	let needsInitialSetup = $state(false);
	let setupMode = $state(false);
	let showNewCompanyForm = $state(false);
	let newCompanyName = $state('');

	// Setup form fields
	let setupUserID = $state('');
	let setupUserName = $state('');
	let setupEmail = $state('');
	let setupPassword = $state('');
	let setupPasswordConfirm = $state('');

	onMount(async () => {
		// Load translations (login page may load before layout's onMount)
		await loadTranslations();

		// Load companies
		try {
			const response = await api.listCompanies();
			if (response.success && response.data) {
				companies = response.data;
				// Set default company if available
				if (companies.length > 0) {
					company = companies[0].name;
				}
			}
		} catch (err) {
			console.error('Failed to load companies:', err);
		}

		// Check if we're already logged in
		try {
			const response = await api.getCurrentUser();
			if (response.success) {
				// Already logged in, redirect to home
				goto('/');
			}
		} catch (err) {
			// Not logged in, check if we need initial setup
			// Try to detect if no users exist by attempting to create initial user with invalid data
			// This will tell us if users already exist
			try {
				const testResponse = await fetch('/api/auth/init', {
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ user_id: '', user_name: '', password: '' })
				});

				if (testResponse.status === 403) {
					// Users already exist
					needsInitialSetup = false;
				} else {
					// No users exist, show setup
					needsInitialSetup = true;
					setupMode = true;
				}
			} catch (err) {
				// Assume users exist if we can't check
				needsInitialSetup = false;
			}
		}
	});

	async function handleLogin() {
		error = '';
		if (!userID || !password) {
			error = t(ERR.LOGIN_MISSING_CREDENTIALS);
			return;
		}

		if (!company) {
			error = t(ERR.LOGIN_NO_COMPANY);
			return;
		}

		loading = true;
		try {
			const response = await api.login(userID, password, company);
			if (response.success) {
				// Store user info in store (also saves to localStorage)
				currentUser.setUser(response.data);
				// Re-initialize session to get updated language from backend
				await session.initialize();
				// Full page reload to apply the new user's language and menu
				window.location.href = '/';
			} else {
				error = response.error || t(ERR.LOGIN_FAILED);
				toast.error(error);
				// Check if it's because no users exist
				if (error.includes('Invalid credentials')) {
					// Try to detect if this is the initial setup scenario
					needsInitialSetup = true;
				}
			}
		} catch (err: any) {
			if (err.status === 401) {
				error = t(ERR.INVALID_CREDENTIALS);
			} else if (err.status === 403) {
				// Valid user, but no User Member entry for this company
				const selected = companies.find((c) => c.name === company);
				error = t(ERR.LOGIN_NO_COMPANY_ACCESS, userID.toUpperCase(), selected ? companyLabel(selected) : company);
			} else {
				error = t(ERR.GENERIC_RETRY);
			}
			toast.error(error);
		} finally {
			loading = false;
		}
	}

	async function handleInitialSetup() {
		error = '';

		// Validation
		if (!setupUserID || !setupUserName || !setupPassword) {
			error = t(ERR.SETUP_MISSING_FIELDS);
			return;
		}

		if (setupPassword !== setupPasswordConfirm) {
			error = t(ERR.PASSWORD_MISMATCH);
			return;
		}

		if (setupPassword.length < 6) {
			error = t(ERR.PASSWORD_TOO_SHORT);
			return;
		}

		loading = true;
		try {
			const response = await api.createInitialUser({
				user_id: setupUserID,
				user_name: setupUserName,
				email: setupEmail,
				password: setupPassword
			});

			if (response.success) {
				toast.success(t(MSG.USER_CREATED));
				// User created, now log in
				userID = setupUserID;
				password = setupPassword;
				setupMode = false;
				needsInitialSetup = false;
				await handleLogin();
			} else {
				error = response.error || t(ERR.FAILED_CREATE_USER);
				toast.error(error);
			}
		} catch (err: any) {
			error = err.message || t(ERR.SETUP_ERROR);
			toast.error(error);
		} finally {
			loading = false;
		}
	}

	function handleKeyPress(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			if (setupMode) {
				handleInitialSetup();
			} else if (showNewCompanyForm) {
				handleCreateCompany();
			} else {
				handleLogin();
			}
		}
	}

	async function handleCreateCompany() {
		error = '';
		if (!newCompanyName.trim()) {
			error = t(ERR.COMPANY_NAME_REQUIRED);
			return;
		}

		loading = true;
		try {
			const response = await api.createCompany(newCompanyName.trim());
			if (response.success && response.data) {
				toast.success(t(MSG.COMPANY_CREATED, response.data.name));
				// Refresh companies list
				const listResponse = await api.listCompanies();
				if (listResponse.success && listResponse.data) {
					companies = listResponse.data;
					company = response.data.name;
				}
				showNewCompanyForm = false;
				newCompanyName = '';
			} else {
				error = response.error || t(ERR.FAILED_CREATE_COMPANY);
				toast.error(error);
			}
		} catch (err: any) {
			error = err.message || t(ERR.GENERIC);
			toast.error(error);
		} finally {
			loading = false;
		}
	}
</script>

<div class="login-container">
	<div class="login-card">
		<div class="login-header">
			<h1>
				<Wordmark height="2.5rem" alt="" />
				<span class="sr-only">GoEasyERP</span>
			</h1>
			<p>{setupMode ? t(LOGIN.INITIAL_SETUP) : t(LOGIN.SIGN_IN)}</p>
		</div>

		{#if error}
			<div class="error-message">
				{error}
			</div>
		{/if}

		{#if setupMode}
			<!-- Initial Setup Form -->
			<div class="form-group">
				<label for="setup-userid">{t(LOGIN.USER_ID)} *</label>
				<input
					id="setup-userid"
					type="text"
					bind:value={setupUserID}
					placeholder="admin"
					disabled={loading}
					onkeypress={handleKeyPress}
				/>
			</div>

			<div class="form-group">
				<label for="setup-username">{t(LOGIN.FULL_NAME)} *</label>
				<input
					id="setup-username"
					type="text"
					bind:value={setupUserName}
					placeholder="Administrator"
					disabled={loading}
					onkeypress={handleKeyPress}
				/>
			</div>

			<div class="form-group">
				<label for="setup-email">{t(LOGIN.EMAIL)}</label>
				<input
					id="setup-email"
					type="email"
					bind:value={setupEmail}
					placeholder="admin@example.com"
					disabled={loading}
					onkeypress={handleKeyPress}
				/>
			</div>

			<div class="form-group">
				<label for="setup-password">{t(LOGIN.PASSWORD)} *</label>
				<input
					id="setup-password"
					type="password"
					bind:value={setupPassword}
					placeholder="••••••••"
					disabled={loading}
					onkeypress={handleKeyPress}
				/>
			</div>

			<div class="form-group">
				<label for="setup-password-confirm">{t(LOGIN.CONFIRM_PASSWORD)} *</label>
				<input
					id="setup-password-confirm"
					type="password"
					bind:value={setupPasswordConfirm}
					placeholder="••••••••"
					disabled={loading}
					onkeypress={handleKeyPress}
				/>
			</div>

			<button class="login-button" onclick={handleInitialSetup} disabled={loading}>
				{loading ? t(LOGIN.CREATING) : t(LOGIN.CREATE_INITIAL_USER)}
			</button>

			<button class="secondary-button" onclick={() => { setupMode = false; needsInitialSetup = false; }} disabled={loading}>
				{t(LOGIN.BACK_TO_LOGIN)}
			</button>
		{:else if showNewCompanyForm}
			<!-- New Company Form -->
			<div class="form-group">
				<label for="new-company">{t(LOGIN.COMPANY_NAME)}</label>
				<!-- svelte-ignore a11y_autofocus -->
				<input
					id="new-company"
					type="text"
					bind:value={newCompanyName}
					placeholder={t(PLC.COMPANY_NAME_EXAMPLE)}
					disabled={loading}
					onkeypress={handleKeyPress}
					autofocus
				/>
				<small class="help-text">{t(LOGIN.COMPANY_NAME_HELP)}</small>
			</div>

			<button class="login-button" onclick={handleCreateCompany} disabled={loading}>
				{loading ? t(LOGIN.CREATING) : t(LOGIN.CREATE_COMPANY)}
			</button>

			<button class="secondary-button" onclick={() => { showNewCompanyForm = false; error = ''; }} disabled={loading}>
				{t(LOGIN.BACK_TO_LOGIN)}
			</button>
		{:else}
			<!-- Login Form: User ID, Password, Company -->
			<div class="form-group">
				<label for="userid">{t(LOGIN.USER_ID)}</label>
				<!-- svelte-ignore a11y_autofocus -->
				<input
					id="userid"
					type="text"
					bind:value={userID}
					placeholder={t(PLC.ENTER_USER_ID)}
					disabled={loading}
					onkeypress={handleKeyPress}
					onblur={() => { userID = userID.toUpperCase(); }}
					autofocus
				/>
			</div>

			<div class="form-group">
				<label for="password">{t(LOGIN.PASSWORD)}</label>
				<input
					id="password"
					type="password"
					bind:value={password}
					placeholder={t(PLC.ENTER_PASSWORD)}
					disabled={loading}
					onkeypress={handleKeyPress}
				/>
			</div>

			<div class="form-group">
				<label for="company">{t(LOGIN.COMPANY)}</label>
				<div class="company-row">
					<select
						id="company"
						bind:value={company}
						disabled={loading || companies.length === 0}
						class="company-select"
					>
						{#if companies.length === 0}
							<option value="">{t(LOGIN.LOADING_COMPANIES)}</option>
						{:else}
							{#each companies as c (c.name)}
								<option value={c.name}>{companyLabel(c)}</option>
							{/each}
						{/if}
					</select>
				</div>
			</div>

			<button class="login-button" onclick={handleLogin} disabled={loading}>
				{loading ? t(LOGIN.SIGNING_IN) : t(LOGIN.SIGN_IN)}
			</button>

			{#if needsInitialSetup}
				<div class="setup-prompt">
					<p>{t(LOGIN.NO_USERS_FOUND)}</p>
					<button class="secondary-button" onclick={() => setupMode = true}>
						{t(LOGIN.CREATE_INITIAL_USER)}
					</button>
				</div>
			{/if}
		{/if}
	</div>
</div>

<style>
	.login-container {
		display: flex;
		align-items: center;
		justify-content: center;
		min-height: 100vh;
		background: linear-gradient(135deg, rgb(var(--accent-600)) 0%, rgb(var(--accent-900)) 100%);
		padding: 1rem;
	}

	:global(.dark) .login-container {
		background: linear-gradient(135deg, rgb(var(--gray-900)) 0%, rgb(var(--gray-800)) 100%);
	}

	.login-card {
		background: white;
		border-radius: 8px;
		box-shadow: 0 10px 40px rgba(0, 0, 0, 0.2);
		padding: 2rem;
		width: 100%;
		max-width: 420px;
	}

	:global(.dark) .login-card {
		background: rgb(var(--gray-800));
		box-shadow: 0 10px 40px rgba(0, 0, 0, 0.5);
	}

	.login-header {
		text-align: center;
		margin-bottom: 2rem;
	}

	.login-header h1 {
		display: flex;
		justify-content: center;
		margin: 0 0 1rem 0;
	}

	.login-header p {
		margin: 0;
		color: rgb(var(--gray-600));
		font-size: 1rem;
	}

	:global(.dark) .login-header p {
		color: rgb(var(--gray-400));
	}

	.error-message {
		background-color: #fee;
		border: 1px solid #fcc;
		border-radius: 4px;
		color: #c33;
		padding: 0.75rem;
		margin-bottom: 1rem;
		font-size: 0.875rem;
	}

	:global(.dark) .error-message {
		background-color: #450a0a;
		border-color: #7f1d1d;
		color: #fca5a5;
	}

	.form-group {
		margin-bottom: 1.25rem;
	}

	.form-group label {
		display: block;
		margin-bottom: 0.5rem;
		color: rgb(var(--fg));
		font-weight: 500;
		font-size: 0.875rem;
	}

	:global(.dark) .form-group label {
		color: rgb(var(--gray-200));
	}

	.form-group input {
		width: 100%;
		padding: 0.75rem;
		border: 1px solid rgb(var(--gray-300));
		border-radius: 4px;
		font-size: 1rem;
		transition: border-color 0.15s ease-in-out;
		box-sizing: border-box;
		background-color: white;
		color: rgb(var(--fg));
	}

	:global(.dark) .form-group input {
		background-color: rgb(var(--gray-700));
		border-color: rgb(var(--gray-600));
		color: rgb(var(--gray-100));
	}

	.form-group input:focus {
		outline: none;
		border-color: rgb(var(--accent-600));
		box-shadow: 0 0 0 3px rgba(0, 132, 137, 0.1);
	}

	:global(.dark) .form-group input:focus {
		border-color: rgb(var(--accent-400));
		box-shadow: 0 0 0 3px rgba(55, 161, 165, 0.2);
	}

	.form-group input:disabled {
		background-color: rgb(var(--gray-50));
		cursor: not-allowed;
	}

	:global(.dark) .form-group input:disabled {
		background-color: rgb(var(--gray-800));
	}

	.form-group input::placeholder {
		color: rgb(var(--gray-400));
	}

	:global(.dark) .form-group input::placeholder {
		color: rgb(var(--gray-500));
	}

	.company-select {
		width: 100%;
		padding: 0.75rem;
		border: 1px solid rgb(var(--gray-300));
		border-radius: 4px;
		font-size: 1rem;
		transition: border-color 0.15s ease-in-out;
		box-sizing: border-box;
		background-color: white;
		color: rgb(var(--fg));
		cursor: pointer;
	}

	:global(.dark) .company-select {
		background-color: rgb(var(--gray-700));
		border-color: rgb(var(--gray-600));
		color: rgb(var(--gray-100));
	}

	.company-select:focus {
		outline: none;
		border-color: rgb(var(--accent-600));
		box-shadow: 0 0 0 3px rgba(0, 132, 137, 0.1);
	}

	:global(.dark) .company-select:focus {
		border-color: rgb(var(--accent-400));
		box-shadow: 0 0 0 3px rgba(55, 161, 165, 0.2);
	}

	.company-select:disabled {
		background-color: rgb(var(--gray-50));
		cursor: not-allowed;
	}

	:global(.dark) .company-select:disabled {
		background-color: rgb(var(--gray-800));
	}

	.login-button {
		width: 100%;
		padding: 0.75rem;
		background: linear-gradient(135deg, rgb(var(--accent-600)) 0%, rgb(var(--accent-900)) 100%);
		color: white;
		border: none;
		border-radius: 4px;
		font-size: 1rem;
		font-weight: 500;
		cursor: pointer;
		transition: transform 0.1s ease-in-out, box-shadow 0.15s ease-in-out;
	}

	:global(.dark) .login-button {
		background: linear-gradient(135deg, rgb(var(--accent-500)) 0%, rgb(var(--accent-800)) 100%);
	}

	.login-button:hover:not(:disabled) {
		transform: translateY(-1px);
		box-shadow: 0 4px 12px rgba(0, 132, 137, 0.4);
	}

	:global(.dark) .login-button:hover:not(:disabled) {
		box-shadow: 0 4px 12px rgba(0, 131, 143, 0.4);
	}

	.login-button:active:not(:disabled) {
		transform: translateY(0);
	}

	.login-button:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.secondary-button {
		width: 100%;
		padding: 0.75rem;
		background: rgb(var(--gray-200));
		color: rgb(var(--fg));
		border: none;
		border-radius: 4px;
		font-size: 0.875rem;
		font-weight: 500;
		cursor: pointer;
		margin-top: 0.75rem;
		transition: background-color 0.15s ease-in-out;
	}

	:global(.dark) .secondary-button {
		background: rgb(var(--gray-700));
		color: rgb(var(--gray-200));
	}

	.secondary-button:hover:not(:disabled) {
		background: rgb(var(--gray-300));
	}

	:global(.dark) .secondary-button:hover:not(:disabled) {
		background: rgb(var(--gray-600));
	}

	.secondary-button:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.setup-prompt {
		margin-top: 1.5rem;
		padding: 1rem;
		background-color: rgb(var(--gray-50));
		border-radius: 4px;
		text-align: center;
	}

	:global(.dark) .setup-prompt {
		background-color: rgb(var(--gray-700));
	}

	.setup-prompt p {
		margin: 0 0 0.75rem 0;
		color: rgb(var(--gray-600));
		font-size: 0.875rem;
	}

	:global(.dark) .setup-prompt p {
		color: rgb(var(--gray-400));
	}

	.company-row {
		display: flex;
		gap: 0.5rem;
	}

	.company-row .company-select {
		flex: 1;
	}

	.new-company-btn {
		padding: 0.75rem 1rem;
		background: linear-gradient(135deg, rgb(var(--accent-600)) 0%, rgb(var(--accent-900)) 100%);
		color: white;
		border: none;
		border-radius: 4px;
		font-size: 1.25rem;
		font-weight: bold;
		cursor: pointer;
		transition: transform 0.1s ease-in-out, box-shadow 0.15s ease-in-out;
		line-height: 1;
	}

	:global(.dark) .new-company-btn {
		background: linear-gradient(135deg, rgb(var(--accent-500)) 0%, rgb(var(--accent-800)) 100%);
	}

	.new-company-btn:hover:not(:disabled) {
		transform: translateY(-1px);
		box-shadow: 0 4px 12px rgba(0, 132, 137, 0.4);
	}

	.new-company-btn:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.help-text {
		display: block;
		margin-top: 0.25rem;
		font-size: 0.75rem;
		color: rgb(var(--gray-600));
	}

	:global(.dark) .help-text {
		color: rgb(var(--gray-400));
	}
</style>
