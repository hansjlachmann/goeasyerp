<script lang="ts">
	import type { Field } from '$lib/types/pages';
	import type { LookupData } from '$lib/types/api';
	import { cn } from '$lib/utils/cn';
	import { getFieldStyleClasses, formatValue, formatOptionValue, formatLookupValue, isAdvancedLookup as isAdvancedLookupData, isDateType, isDateTimeType, isDecimalType, formatDate, formatDateTime, formatDecimal, toDateTimeInput, fromDateTimeInput } from '$lib/utils/fieldHelpers';
	import { toDecimalInput, fromDecimalInput } from '$lib/utils/regional';
	import DateInput from '../DateInput.svelte';
	import { currentLanguage } from '$lib/stores/session';
	import { t, MSG } from '$lib/services/i18n.svelte';
	import LookupDropdown from './LookupDropdown.svelte';
	import OptionDropdown from './OptionDropdown.svelte';

	interface Props {
		field: Field;
		value: any;
		caption?: string;
		editable?: boolean;
		required?: boolean;
		error?: string;
		fieldType?: string; // Field type from backend metadata (e.g., "bool", "code", "text")
		options?: Record<string, string>; // Option field values (key = stored value, value = display text)
		lookups?: LookupData; // Lookup values (advanced or simple mode)
		fieldCaptions?: Record<string, string>; // Field captions for lookup column headers
		tabindex?: number;
		onchange?: (value: any) => void;
		onblur?: (event?: FocusEvent) => void;
		// AssistEdit: the field's "…" button (also Shift+F2 in the field)
		onassistedit?: () => void;
		// DrillDown: the read-only value is a link (e.g. No. of Ledger Entries → the entries)
		ondrilldown?: () => void;
	}

	let {
		field,
		value = $bindable(),
		caption,
		editable = false,
		required = false,
		error,
		fieldType,
		options,
		lookups,
		fieldCaptions = {},
		tabindex,
		onchange,
		onblur,
		onassistedit,
		ondrilldown
	}: Props = $props();

	// Determine if field is editable
	const isEditable = $derived(editable);

	// Locale for date formatting
	let locale = $state('en-US');
	currentLanguage.subscribe((lang) => { locale = lang; });

	// Check if this is a boolean field (detected from actual value type or backend metadata)
	const isBooleanField = $derived(typeof value === 'boolean' || fieldType === 'bool');

	// Check if this is a date or datetime field
	const isDateField = $derived(isDateType(fieldType));
	const isDateTimeField = $derived(isDateTimeType(fieldType));
	const isDecimalField = $derived(isDecimalType(fieldType));

	// Check if this is an option/enum field (has options provided)
	const isOptionField = $derived(options && Object.keys(options).length > 0);

	// Check if this is a lookup field with advanced columns (table-style dropdown)
	// (rows sent along, or loaded on demand from lazy_url for large related tables)
	const isAdvancedLookup = $derived(isAdvancedLookupData(lookups));

	// Check if this is a simple lookup field (basic dropdown)
	const isSimpleLookup = $derived(
		lookups && lookups.simple && Object.keys(lookups.simple).length > 0 && !isAdvancedLookup
	);

	// Check if this is any lookup field
	const isLookupField = $derived(isAdvancedLookup || isSimpleLookup);

	// Get field caption (from props, field definition, or field source)
	const fieldCaption = $derived(caption || field.caption || field.source);

	// Determine field style classes based on metadata
	const fieldStyle = $derived(getFieldStyleClasses(field));

	// Handle value change for checkbox (boolean fields)
	function handleCheckboxChange(e: Event) {
		const target = e.target as HTMLInputElement;
		const newValue = target.checked;
		value = newValue;
		onchange?.(newValue);
		onblur?.();
	}

	// Handle value change for text inputs
	function handleChange(e: Event) {
		const target = e.target as HTMLInputElement;
		// datetime-local holds local time without a zone; the API takes RFC3339
		// decimals are typed with the user's decimal separator (My Settings), sent with a dot
		const newValue = isDateTimeField ? fromDateTimeInput(target.value) : isDecimalField ? fromDecimalInput(target.value, locale) : target.value;
		value = newValue;
		onchange?.(newValue);
	}

	// Handle value change for select (option fields)
	function handleSelectChange(e: Event) {
		const target = e.target as HTMLSelectElement;
		// Convert back to number since options are stored as integers
		const newValue = parseInt(target.value, 10);
		value = newValue;
		onchange?.(newValue);
		// Trigger blur to save immediately after selection
		onblur?.();
	}

	// Handle value change for lookup select (table relation fields)
	function handleLookupSelectChange(e: Event) {
		const target = e.target as HTMLSelectElement;
		// Lookups store the code as a string
		const newValue = target.value;
		value = newValue;
		onchange?.(newValue);
		// Trigger blur to save immediately after selection
		onblur?.();
	}

	// Get display value for option field (convert stored integer to display text)
	const optionDisplayValue = $derived(() => formatOptionValue(value, options));

	// Get display value for lookup field (show "code - description" or just description)
	const lookupDisplayValue = $derived(() => formatLookupValue(value, lookups, !isAdvancedLookup));

	// Handle lookup selection from LookupDropdown
	function handleLookupSelect(key: string) {
		value = key;
		onchange?.(key);
	}

	// Determine input type based on field
	const inputType = $derived(() => {
		// Masked fields (table/page YAML masked: true) are secrets: never shown in clear text
		if (field.masked) {
			return 'password';
		}
		if (field.source.includes('email')) {
			return 'email';
		}
		if (isDateTimeField) {
			return 'datetime-local';
		}
		return 'text';
	});
</script>

{#if isEditable}
	<!-- Editable field -->
	<div class="field-group" class:has-error={!!error}>
		<label for={field.source} class="field-label">
			{fieldCaption}
			{#if required}
				<span class="required-indicator">*</span>
			{/if}
		</label>
		<div class="input-wrapper">
			{#if isBooleanField}
				<!-- Boolean field - render as checkbox -->
				<div class="checkbox-wrapper">
					<input
						id={field.source}
						type="checkbox"
						checked={value}
						{tabindex}
						onchange={handleCheckboxChange}
					/>
				</div>
			{:else if isOptionField && options}
				<!-- Option/Enum field - render as OptionDropdown -->
				<OptionDropdown
					arrowsMoveFields
					{options}
					{value}
					{tabindex}
					onselect={(newValue) => {
						value = newValue;
						onchange?.(newValue);
						onblur?.();
					}}
					onblur={() => onblur?.()}
				/>
			{:else if isAdvancedLookup && lookups?.columns}
				<!-- Advanced lookup with columns - render as table-style dropdown -->
				<LookupDropdown
					columns={[...lookups.columns]}
					rows={[...(lookups.rows ?? [])]}
					lazyUrl={lookups.lazy_url}
					freeText={lookups.free_text}
					newPageId={lookups.new_page_id}
					newPath={lookups.new_path}
					newKeyField={lookups.key_field}
					arrowsMoveFields
					value={value || ''}
					fieldName={fieldCaption}
					captions={fieldCaptions}
					searchTimeout={lookups.search_timeout}
					{tabindex}
					error={!!error}
					onselect={handleLookupSelect}
					onblur={() => onblur?.()}
				/>
			{:else if isSimpleLookup && lookups?.simple}
				<!-- Simple lookup - render as basic dropdown -->
				<select
					id={field.source}
					class={cn('select', fieldStyle, error ? 'input-error' : '')}
					value={value ?? ''}
					{tabindex}
					onchange={handleLookupSelectChange}
					onblur={() => onblur?.()}
					aria-invalid={!!error}
					aria-describedby={error ? `${field.source}-error` : undefined}
				>
					<option value=""></option>
					{#each Object.entries(lookups!.simple!) as [lookupCode, lookupDisplay]}
						<option value={lookupCode} title={lookupDisplay}>{lookupCode}</option>
					{/each}
				</select>
			{:else if isDateField}
				<!-- Date field: the user's format, short entries, a calendar (DateInput) -->
				<DateInput
					id={field.source}
					class={cn('input', fieldStyle, error ? 'input-error' : '')}
					{value}
					{tabindex}
					onchange={(iso) => {
						value = iso;
						onchange?.(iso);
					}}
					onblur={(e) => onblur?.(e)}
					aria-invalid={!!error}
					aria-describedby={error ? `${field.source}-error` : undefined}
				/>
			{:else}
				<!-- Regular text field -->
				<!-- autocomplete: browsers must not fill saved logins/addresses into record fields
				     ("new-password" is the value Chrome/Edge respect for password inputs) -->
				<input
					id={field.source}
					type={inputType()}
					autocomplete={field.masked ? 'new-password' : 'off'}
					class={cn('input', fieldStyle, isDecimalField && 'amount', error ? 'input-error' : '', onassistedit ? 'with-assist' : '')}
					value={isDateTimeField ? toDateTimeInput(value) : isDecimalField ? toDecimalInput(value, locale) : (value ?? '')}
					{tabindex}
					oninput={handleChange}
					onblur={(e) => onblur?.(e)}
					onkeydown={(e) => {
						if (onassistedit && e.key === 'F2' && e.shiftKey) {
							e.preventDefault();
							onassistedit();
						}
					}}
					aria-invalid={!!error}
					aria-describedby={error ? `${field.source}-error` : undefined}
				/>
				{#if onassistedit}
					<button
						type="button"
						class="assist-edit"
						tabindex="-1"
						title={t(MSG.ASSIST_EDIT)}
						aria-label={t(MSG.ASSIST_EDIT)}
						onclick={() => onassistedit()}>…</button
					>
				{/if}
			{/if}
		</div>
		{#if error}
			<p id="{field.source}-error" class="error-message">{error}</p>
		{/if}
	</div>
{:else}
	<!-- Read-only field -->
	<div class="field-group readonly">
		<div class="field-label">
			{fieldCaption}
		</div>
		{#if isBooleanField}
			<!-- Boolean field - read-only checkbox -->
			<div class="checkbox-wrapper readonly">
				<input
					type="checkbox"
					checked={value}
					disabled
				/>
			</div>
		{:else if isAdvancedLookup && lookups?.columns}
			<!-- Advanced lookup - show dropdown even in non-edit mode -->
			<div class="input-wrapper">
				<LookupDropdown
					columns={[...lookups.columns]}
					rows={[...(lookups.rows ?? [])]}
					lazyUrl={lookups.lazy_url}
					freeText={lookups.free_text}
					newPageId={lookups.new_page_id}
					newPath={lookups.new_path}
					newKeyField={lookups.key_field}
					arrowsMoveFields
					value={value || ''}
					fieldName={fieldCaption}
					captions={fieldCaptions}
					searchTimeout={lookups.search_timeout}
					{tabindex}
					error={false}
					onselect={handleLookupSelect}
					onblur={() => onblur?.()}
				/>
			</div>
		{:else if isSimpleLookup && lookups?.simple}
			<!-- Simple lookup - show dropdown even in non-edit mode -->
			<div class="input-wrapper">
				<select
					id={field.source}
					class={cn('select', fieldStyle)}
					value={value ?? ''}
					{tabindex}
					onchange={handleLookupSelectChange}
					onblur={() => onblur?.()}
				>
					<option value=""></option>
					{#each Object.entries(lookups!.simple!) as [lookupCode, lookupDisplay]}
						<option value={lookupCode} title={lookupDisplay}>{lookupCode}</option>
					{/each}
				</select>
			</div>
		{:else}
			<div class={cn('field-value', fieldStyle, isDecimalField && 'amount')}>
				{#if ondrilldown}
					<button type="button" class="drilldown-link" {tabindex} onclick={() => ondrilldown()}>
						{formatDecimal(value, field.decimal_places)}
					</button>
				{:else if isOptionField}
					{optionDisplayValue()}
				{:else if isDateField}
					{formatDate(String(value ?? ''), locale)}
				{:else if isDateTimeField}
					{formatDateTime(String(value ?? ''), locale)}
				{:else}
					{formatDecimal(value, field.decimal_places)}
				{/if}
			</div>
		{/if}
	</div>
{/if}

<style>
	/* DrillDown: the value opens the entries behind it */
	.drilldown-link {
		color: rgb(var(--accent-600));
		text-decoration: underline;
		text-underline-offset: 2px;
	}
	:global(.dark) .drilldown-link {
		color: rgb(var(--accent-400));
	}

	/* AssistEdit "…" at the right end of the field */
	.input-wrapper:has(.assist-edit) {
		position: relative;
	}
	:global(.input.with-assist) {
		padding-right: 2rem;
	}
	.assist-edit {
		position: absolute;
		right: 0.25rem;
		top: 50%;
		transform: translateY(-50%);
		padding: 0 0.4rem;
		line-height: 1.4;
		border-radius: 0.25rem;
		color: rgb(var(--gray-600));
	}
	.assist-edit:hover {
		background: rgb(var(--gray-200));
		color: rgb(var(--fg));
	}
	:global(.dark) .assist-edit:hover {
		background: rgb(var(--gray-700));
	}

	.field-group {
		@apply flex flex-col gap-1;
	}

	.field-label {
		@apply text-sm font-medium text-gray-700;
		@apply flex items-center gap-1;
	}

	:global(.dark) .field-label {
		color: var(--color-text-secondary);
	}

	.required-indicator {
		@apply text-red-500 font-bold;
	}

	.input-wrapper {
		@apply relative;
	}

	.field-group :global(input.input),
	.field-group :global(select.select) {
		@apply w-full py-2 px-3;
		@apply bg-surface border border-gray-300 rounded-md;
		@apply text-gray-900 text-base;
		@apply transition-all duration-150;
		@apply outline-none;
	}

	.field-group :global(select.select) {
		@apply cursor-pointer;
		@apply appearance-none;
		background-image: url("data:image/svg+xml,%3csvg xmlns='http://www.w3.org/2000/svg' fill='none' viewBox='0 0 20 20'%3e%3cpath stroke='%236b7280' stroke-linecap='round' stroke-linejoin='round' stroke-width='1.5' d='M6 8l4 4 4-4'/%3e%3c/svg%3e");
		background-position: right 0.5rem center;
		background-repeat: no-repeat;
		background-size: 1.5em 1.5em;
		padding-right: 2.5rem;
	}

	.field-group :global(input.input:hover:not(:focus)),
	.field-group :global(select.select:hover:not(:focus)) {
		@apply border-gray-400;
	}

	.field-group :global(input.input:focus),
	.field-group :global(select.select:focus) {
		@apply border-blue-500 ring-2 ring-blue-500/20;
	}

	.field-group.has-error :global(input.input),
	.field-group.has-error :global(select.select) {
		@apply border-red-500;
	}

	.field-group.has-error :global(input.input:focus),
	.field-group.has-error :global(select.select:focus) {
		@apply border-red-500 ring-2 ring-red-500/20;
	}

	/* Dark mode input styles */
	:global(.dark) .field-group :global(input.input),
	:global(.dark) .field-group :global(select.select) {
		background-color: var(--color-bg-input);
		border-color: var(--color-border-secondary);
		color: var(--color-text-primary);
	}

	:global(.dark) .field-group :global(select.select) {
		background-image: url("data:image/svg+xml,%3csvg xmlns='http://www.w3.org/2000/svg' fill='none' viewBox='0 0 20 20'%3e%3cpath stroke='%239ca3af' stroke-linecap='round' stroke-linejoin='round' stroke-width='1.5' d='M6 8l4 4 4-4'/%3e%3c/svg%3e");
	}

	:global(.dark) .field-group :global(select.select) option {
		background-color: var(--color-bg-primary);
		color: var(--color-text-primary);
	}

	:global(.dark) .field-group :global(input.input:hover:not(:focus)),
	:global(.dark) .field-group :global(select.select:hover:not(:focus)) {
		border-color: var(--color-text-muted);
	}

	:global(.dark) .field-group :global(input.input:focus),
	:global(.dark) .field-group :global(select.select:focus) {
		border-color: rgb(var(--accent-500)); /* blue-500 */
		box-shadow: 0 0 0 2px rgba(0, 131, 143, 0.3);
	}

	:global(.dark) .field-group :global(input.input::placeholder) {
		color: var(--color-text-muted);
	}

	.checkbox-wrapper {
		@apply flex items-center py-2 px-3;
		min-height: 2.5rem;
	}

	.checkbox-wrapper input[type="checkbox"] {
		@apply w-5 h-5 cursor-pointer;
		accent-color: rgb(var(--accent-500));
	}

	.checkbox-wrapper.readonly input[type="checkbox"] {
		@apply cursor-not-allowed opacity-70;
	}

	.error-message {
		@apply text-xs text-red-600 mt-1;
	}

	:global(.dark) .error-message {
		color: #fca5a5; /* red-300 */
	}

	.field-value {
		@apply text-base py-2 px-3;
		@apply bg-gray-50 border border-gray-200 rounded-md;
		@apply text-gray-900;
		min-height: 2.5rem;
		@apply flex items-center;
	}

	:global(.dark) .field-value {
		background-color: var(--color-bg-input);
		border-color: var(--color-border-secondary);
		color: var(--color-text-primary);
	}

	/* Strong style for important values */
	.field-value :global(.strong) {
		@apply font-semibold;
	}

	/* Readonly visual distinction */
	.field-group.readonly .field-value {
		@apply bg-gray-100;
	}

	:global(.dark) .field-group.readonly .field-value {
		background-color: var(--color-bg-primary);
	}
</style>
