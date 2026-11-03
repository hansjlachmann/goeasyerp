<script lang="ts">
	import { untrack } from 'svelte';
	import Modal from '../Modal.svelte';
	import { goto } from '$app/navigation';
	import type { PageDefinition, Field, FieldState, Action } from '$lib/types/pages';
	import type { LookupData } from '$lib/types/api';
	import FieldRenderer from './FieldRenderer.svelte';
	import Button from '$lib/components/Button.svelte';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import Card from '$lib/components/Card.svelte';
	import CustomizeFieldsModal from './CustomizeFieldsModal.svelte';
	import PlusIcon from '$lib/components/icons/PlusIcon.svelte';
	import NavigationButtons from '$lib/components/NavigationButtons.svelte';
	import { shortcuts, createShortcutMap, normalizeShortcut } from '$lib/utils/shortcuts';
	import { currentUser } from '$lib/stores/user';
	import { lookupFor, getFieldCaption, isItemVisible, isBlobType, type ItemCustomization } from '$lib/utils/fieldHelpers';
	import { loadPageCustomizations, savePageCustomizations } from '$lib/utils/customizationStorage';
	import { getRecordId, isNewRecord, deepCopy, getPrimaryKeyField, getPrimaryKeyFields, stripInternalFields, hasRecordChanged, sameFieldValue } from '$lib/utils/recordHelpers';
	import { runPagePath, type RunPageAction } from '$lib/utils/runPage';
	import { withReturn } from '$lib/utils/returnUrl';
	import { toast } from '$lib/stores/toast';
	import { api } from '$lib/services/api';
	import { t, ERR, MSG, LIST, CARD } from '$lib/services/i18n.svelte';
	import PictureField from './PictureField.svelte';
	import PeriodGrid from './PeriodGrid.svelte';
	import MatrixGrid from './MatrixGrid.svelte';
	import { handleFieldEnterKey, getFieldControls, arrowLeavesField, fieldInDirection, focusField, type ArrowDirection } from '$lib/utils/fieldNavigation';
	import { getJson, setJson } from '$lib/utils/storage';
	import PageRenderer from './PageRenderer.svelte';
	import ActionButton from '../ActionButton.svelte';
	import ActionMenu from '../ActionMenu.svelte';
	import ObjectRunner from '../ObjectRunner.svelte';
	import type { Subpage } from '$lib/types/pages';
	import type { FixedFilter } from '$lib/types/api';

	interface Props {
		page: PageDefinition;
		record?: Record<string, any>;
		captions?: Record<string, string>;
		fieldTypes?: Record<string, string>; // Field type metadata (e.g., "bool", "code", "text")
		options?: Record<string, Record<string, string>>; // Option field values (enum lookups)
		lookups?: Record<string, LookupData>; // Table relation lookup values
		onaction?: (actionName: string) => void;
		// leavingKey: the user left the blank key field of a new auto_key record (insert it now)
		onsave?: (record: Record<string, any>, opts?: { leavingKey?: boolean }) => Promise<boolean> | boolean | void;
		onclose?: () => void; // Callback when closing the card
		// A card opened over its list (modal card): going back to the list closes it
		onbacktolist?: () => void;
		saveBlocked?: boolean; // Block editing due to save error (e.g., record already exists)
		saveBlockedMessage?: string; // Error message to display when blocked
		onclearerror?: () => void; // Callback to clear the error state
		navigationEnabled?: boolean;
		initialEditMode?: boolean;
		canNavigateFirst?: boolean;
		canNavigatePrevious?: boolean;
		canNavigateNext?: boolean;
		canNavigateLast?: boolean;
		onNavigateFirst?: () => void;
		onNavigatePrevious?: () => void;
		onNavigateNext?: () => void;
		onNavigateLast?: () => void;
		// A subpage's lines changed: the card refreshes its FlowFields (totals)
		onrefresh?: () => void;
	}

	let {
		page,
		record = $bindable({}),
		captions = {},
		fieldTypes = {},
		options = {},
		lookups = {},
		onaction,
		onsave,
		onclose,
		onbacktolist,
		saveBlocked = false,
		saveBlockedMessage = '',
		onclearerror,
		navigationEnabled = false,
		initialEditMode,
		canNavigateFirst = false,
		canNavigatePrevious = false,
		canNavigateNext = false,
		canNavigateLast = false,
		onNavigateFirst,
		onNavigatePrevious,
		onNavigateNext,
		onNavigateLast,
		onrefresh
	}: Props = $props();

	// Get primary key field name from page definition
	const primaryKeyField = $derived(getPrimaryKeyField(page));

	// Page triggers (OnAfterGetRecord): fields switched for the current record and the values
	// of the calculated fields (page variables), asked from the server when the record or a
	// field value changes
	let fieldStates = $state<Record<string, FieldState>>({});
	let calculatedValues = $state<Record<string, any>>({});
	let fieldStatesRequest = 0;
	$effect(() => {
		if (!page.page.has_record_triggers) return;
		const current = stripInternalFields($state.snapshot(record) ?? {});
		const request = ++fieldStatesRequest;
		const timer = setTimeout(async () => {
			try {
				const result = await api.getFieldStates(page.page.id, current);
				if (request === fieldStatesRequest) {
					fieldStates = result.states;
					calculatedValues = result.values;
				}
			} catch {
				// keep the last states: the page definition still applies
			}
		}, 150);
		return () => clearTimeout(timer);
	});

	// Customization state
	let customizeModalOpen = $state(false);
	let fieldCustomizations = $state<Record<string, ItemCustomization>>({});

	// Tab while the frame itself has the focus (no field yet): to the first field, Shift+Tab to
	// the last one, in the card's field order (as Enter moves)
	function handleFrameTab(event: KeyboardEvent) {
		if (event.key !== 'Tab' || event.target !== cardPageElement || !cardPageElement) return;
		const fields = getFieldControls(cardPageElement);
		const target = event.shiftKey ? fields.at(-1) : fields[0];
		if (target) {
			event.preventDefault();
			target.focus();
		}
	}

	// Reference to the card page element for focus management
	let cardPageElement = $state<HTMLDivElement | null>(null);


	// Check if record is empty (no data loaded)
	const isEmptyRecord = $derived(() => {
		if (!record) return true;
		const keys = Object.keys(record).filter(k => !k.startsWith('_'));
		return keys.length === 0;
	});

	// Check if this is a new record (no ID)
	function checkIsNewRecord(): boolean {
		// A setup record has a blank key and always exists. A composite key (FA No. + Depreciation
		// Book Code) is new while one of its fields is blank — a new record from a filtered list
		// already has the filter's field (Ship-to Address: Customer No.)
		if (page?.page?.setup_table) return false;
		const keyFields = getPrimaryKeyFields(page);
		if (keyFields.length > 1) return keyFields.some((f) => record?.[f] === undefined || record?.[f] === null || record?.[f] === '');
		return isNewRecord(record, keyFields[0]);
	}

	// Edit mode state - start in edit mode for new records or if initialEditMode is true
	// svelte-ignore state_referenced_locally - initialEditMode is intentionally only used for initialization
	let editMode = $state(initialEditMode ?? checkIsNewRecord());

	// Track if we've already focused the initial field (to avoid refocusing on every state change)
	let initialFocusDone = $state(false);

	// Track the last known record ID to detect when a truly new record is loaded
	let lastRecordId = $state<string | undefined>(undefined);

	// Auto-enable edit mode when a new record is loaded (not when field values change)
	$effect(() => {
		const id = checkIsNewRecord() ? undefined : getRecordId(record, primaryKeyField, getPrimaryKeyFields(page));
		// Only trigger when the record ID actually changes (not on every field change)
		if (id !== lastRecordId) {
			lastRecordId = id;
			if (!id && !page?.page?.setup_table) {
				editMode = true;
				// Reset focus flag only when loading a truly new record
				initialFocusDone = false;
			}
		}
	});

	// Focus the configured field when page opens in edit mode, or focus the card page for keyboard shortcuts
	$effect(() => {
		if (!page?.page) return; // Guard during cleanup
		if (editMode && page.page.focus_field && !initialFocusDone) {
			initialFocusDone = true;
			// Use longer timeout to ensure modal animation completes and DOM is ready
			setTimeout(() => {
				const input = document.getElementById(page.page.focus_field!);
				if (input) {
					input.focus();
					// Select all text if it's an input
					if (input instanceof HTMLInputElement) {
						input.select();
					}
				}
			}, 250);
		} else if (!initialFocusDone && cardPageElement) {
			// Otherwise (view mode, or edit mode without a focus field) focus the card page
			// element so keyboard shortcuts (Esc, Ctrl+E …) work
			initialFocusDone = true;
			setTimeout(() => {
				cardPageElement?.focus();
			}, 100);
		}
	});

	// Load customizations from localStorage on mount
	$effect(() => {
		if (!page?.page) return; // Guard during cleanup
		const userId = $currentUser?.user_id || 'anonymous';
		fieldCustomizations = loadPageCustomizations<Record<string, ItemCustomization>>(
			userId,
			page.page.id
		);
	});

	// Auto-save state
	let saveState = $state<'idle' | 'saving' | 'saved'>('idle');
	let saveTimeout: number | null = null;
	let savedTimeout: number | null = null;

	// Track last saved record state for reverting on validation error
	let lastSavedRecord = $state<Record<string, any>>({});

	// Update lastSavedRecord when record is loaded from parent
	$effect(() => {
		const id = getRecordId(record, primaryKeyField);
		// When a record is first loaded (has ID and we don't have it saved yet)
		if (id && (!lastSavedRecord || getRecordId(lastSavedRecord, primaryKeyField) !== id)) {
			lastSavedRecord = deepCopy(record);
		}
	});

	// Auto-save with debouncing
	function autoSave(opts?: { leavingKey?: boolean }) {
		// Skip if already saving or blocked due to error
		if (saveState === 'saving' || saveBlocked) {
			return;
		}
		// Tabbing through the fields of a saved record changes nothing: nothing to save
		if (!checkIsNewRecord() && lastSavedRecord && !hasRecordChanged($state.snapshot(record), lastSavedRecord)) {
			return;
		}

		// Clear any pending timeouts
		if (saveTimeout) {
			clearTimeout(saveTimeout);
		}
		if (savedTimeout) {
			clearTimeout(savedTimeout);
		}

		// Debounce: wait 300ms after last change before saving
		saveTimeout = setTimeout(async () => {
			// Double-check save state before proceeding
			if (saveState === 'saving') {
				return;
			}

			saveState = 'saving';
			// the fields with an OnAfterValidate whose value this save changes
			const before = lastSavedRecord;
			const afterValidate = [
				...new Set(
					(page.page.layout.sections ?? [])
						.flatMap((s) => s.fields ?? [])
						.filter((f) => f.on_after_validate && before && !sameFieldValue(record[f.source], before[f.source]))
						.map((f) => f.on_after_validate as string)
				)
			];
			try {
				// onsave returns true if a save actually happened
				const didSave = await onsave?.(record, opts);

				if (didSave) {
					// OnAfterValidate: after the save, quietly (its questions and pages show)
					for (const fn of afterValidate) void runner?.run(fn, $state.snapshot(record), { quiet: true });
					// Update lastSavedRecord after successful save
					lastSavedRecord = deepCopy(record);
					savedRecord = deepCopy($state.snapshot(record));
					// The header changed: the subpages reload their lines (the server may have
					// changed them, e.g. a new discount)
					linesRefresh++;
					saveState = 'saved';
					// Show "Saved" for 1.5 seconds then hide
					savedTimeout = setTimeout(() => {
						saveState = 'idle';
					}, 1500) as unknown as number;
				} else {
					// No actual save happened (no changes)
					saveState = 'idle';
				}
			} catch (err) {
				saveState = 'idle';
				// Validation error - revert to last saved state
				const errorMessage = err instanceof Error ? err.message : t(ERR.VALIDATION_FAILED);
				toast.error(errorMessage);

				// Revert record to last saved state
				if (lastSavedRecord && Object.keys(lastSavedRecord).length > 0) {
					// Copy each field back to revert changes
					for (const key of Object.keys(record)) {
						if (key in lastSavedRecord) {
							record[key] = lastSavedRecord[key];
						}
					}
				}
			}
		}, 300) as unknown as number;
	}

	// Handle field blur (when user leaves a field)
	function handleFieldBlur(field?: { source: string }, event?: FocusEvent) {
		// A new record of an auto-numbered table (auto_key: a number series) is inserted as soon
		// as the user leaves its blank No. field (Enter/Tab, classic) — it gets the next number;
		// only when the focus moves on to another field of the card — not to the field's "…"
		// (AssistEdit picks the series first) and not when the card is closed (Esc)
		const target = event?.relatedTarget as HTMLElement | null;
		const toNextField = !!target?.closest('.sections-container') && !target.classList.contains('assist-edit');
		const leavingKey =
			!!page.page.auto_key &&
			!!field &&
			checkIsNewRecord() &&
			getPrimaryKeyFields(page).includes(field.source) &&
			String(record?.[field.source] ?? '').trim() === '' &&
			toNextField;
		autoSave(leavingKey ? { leavingKey } : undefined);
	}

	// Handle action clicks
	function handleAction(actionName: string) {
		// Handle built-in actions locally first
		switch (actionName) {
			case 'Edit':
				toggleEditMode();
				return;
			case 'New':
				handleNew();
				return;
		}

		// Pass all other actions to parent (including Delete)
		onaction?.(actionName);
	}

	// DrillDown on a card field (No. of Ledger Entries, Balance): the page behind the value opens
	// in a modal over the card, filtered to this record (drilldown_filter_field = this record's
	// drilldown_filter_value); Esc or the close button returns to the card
	let drilldown = $state<{ pageId: number; filters: import('$lib/types/api').TableFilter[] } | null>(null);
	function openDrilldown(field: import('$lib/types/pages').Field) {
		const filters: import('$lib/types/api').TableFilter[] = [];
		if (field.drilldown_filter_field && field.drilldown_filter_value) {
			const value = record?.[field.drilldown_filter_value];
			if (value === undefined || value === null || value === '') return;
			filters.push({ field: field.drilldown_filter_field, expression: String(value) });
		}
		drilldown = { pageId: field.drilldown!, filters };
	}

	// AssistEdit (a field's "…", Shift+F2): its function works on the card's record — also a new
	// one — and may open a page to choose from; the record it returns is taken over and saved
	// like a field change
	async function handleAssistEdit(runObject: string) {
		if (!record) return;
		const data = await runner?.run(runObject, stripInternalFields($state.snapshot(record)), { quiet: true });
		const changed = data?.record as Record<string, unknown> | undefined;
		if (!changed) return;
		for (const [key, value] of Object.entries(changed)) {
			record[key] = value;
		}
		autoSave();
	}

	// Handle run_object actions (codeunits)
	// URL of an action's run_page, filtered to this record when the action names a filter
	// (run_page_filter_field = record[run_page_filter_value]), e.g. the customer's ledger entries
	function runPageUrl(action: RunPageAction): string {
		const url = runPagePath(action, record) ?? `/pages/${action.run_page}`;
		// Esc / close on the opened page comes back here (from a modal card: to its list,
		// on this record)
		// on this record; a modal card (over its list) opens again
		return withReturn(url, getRecordId(record, getPrimaryKeyField(page), getPrimaryKeyFields(page)), !!onbacktolist);
	}

	// Run a page object with the card's record: a codeunit, a report or a function
	// ("table:Job_Queue.ShowStatus", with arguments "…(0)") — its dialogs and message box
	// come from the ObjectRunner; afterwards the card and its subpages refresh when needed
	async function handleRunObject(runObject: string) {
		if (!record || Object.keys(record).length === 0 || checkIsNewRecord()) {
			toast.error(t(ERR.NO_RECORD_SELECTED));
			return;
		}
		await runner?.run(runObject, stripInternalFields($state.snapshot(record)));
	}

	function handleRunDone(refresh: boolean) {
		if (!refresh) return;
		onaction?.('Refresh');
		linesRefresh++;
	}

	// A card action or menu item: open a page, run an object, or a standard action
	// The action going back to the card's own list (Back to List)
	function isBackToList(action: Action): boolean {
		return !!action.run_page && action.run_page === page.page.list_page_id && !action.run_page_filter_field && !action.run_page_filters?.length;
	}

	function runCardAction(action: Action) {
		if (isBackToList(action)) {
			// Over the list (modal card): close the card; a card page: open the list on this
			// record — not as a page to return from (its Esc would come back here)
			if (onbacktolist) onbacktolist();
			else goto(`/pages/${action.run_page}?select=${encodeURIComponent(getRecordId(record, getPrimaryKeyField(page), getPrimaryKeyFields(page)) ?? '')}`);
		} else if (action.run_page) {
			goto(runPageUrl(action));
		} else if (action.run_object) {
			handleRunObject(action.run_object);
		} else if (!action.items) {
			handleAction(action.name);
		}
	}

	// The card's actions with the items of its menu buttons (names and shortcuts)
	function allActions(): Action[] {
		const flat = (actions: Action[] | undefined): Action[] => (actions ?? []).flatMap((a) => [a, ...flat(a.items)]);
		return flat(page.page.actions);
	}

	function handleNew() {
		record = {};
	}

	function toggleEditMode() {
		editMode = !editMode;
	}

	// Build keyboard shortcut map from actions
	const shortcutMap = $derived(() => {
		const map: Record<string, () => void> = {};

		// Guard against page being undefined during cleanup
		if (!page?.page) return map;

		allActions().forEach((action) => {
			if (action.shortcut && action.enabled !== false) {
				// Normalize shortcut (e.g., "Esc" -> "Escape") to match keyboard event key names
				const normalizedShortcut = normalizeShortcut(action.shortcut);
				// Over the list, Esc belongs to the modal (it closes the card once)
				if (onbacktolist && normalizedShortcut === 'Escape') return;
				map[normalizedShortcut] = () => runCardAction(action);
			}
		});

		// Add navigation shortcuts if enabled
		if (navigationEnabled) {
			if (canNavigateFirst) map['Ctrl+Home'] = () => onNavigateFirst?.();
			// On a card with subpages Ctrl+↑/↓ move between the parts (handlePartKeys);
			// Ctrl+Shift+↑/↓ go to the previous/next record on every card
			if (canNavigatePrevious && !hasSubpages) map['Ctrl+ArrowUp'] = () => onNavigatePrevious?.();
			if (canNavigateNext && !hasSubpages) map['Ctrl+ArrowDown'] = () => onNavigateNext?.();
			if (canNavigateLast) map['Ctrl+End'] = () => onNavigateLast?.();
		}

		// Add Escape to close
		if (onclose) {
			map['Escape'] = () => onclose();
		}

		// PgUp/PgDn scroll the sections a screen up/down — also from a field, unless an open
		// dropdown (lookup, option list) used the key
		const scrollPage = (direction: number) => ((event?: KeyboardEvent) => {
			if (event?.defaultPrevented) return false;
			if (!sectionsElement) return false;
			sectionsElement.scrollBy({ top: direction * sectionsElement.clientHeight * 0.85, behavior: 'smooth' });
		}) as () => void;
		map['PageDown'] = scrollPage(1);
		map['PageUp'] = scrollPage(-1);

		return map;
	});

	// Runs the card's page objects (codeunits, reports, functions) with their dialogs
	let runner = $state<ObjectRunner | undefined>(undefined);
	// Bumped after a save or a function asking for it: the subpages reload their lines
	let linesRefresh = $state(0);

	// The scrolling sections (PgUp/PgDn)
	let sectionsElement = $state<HTMLDivElement | null>(null);

	// ---- Subpages: list pages in the card, tied to its record (section.subpage.link) ----
	const hasSubpages = $derived((page.page.layout.sections ?? []).some((s) => !!s.subpage));

	// The record as it is in the database (loaded, navigated to, inserted or saved): the
	// subpages link to it, not to the values being typed — typing a new record's No. would
	// otherwise show lines of a record that does not exist and reload them per keystroke
	let savedRecord = $state<Record<string, any> | null>(null);
	$effect.pre(() => {
		// Only a new record object from the parent (typing changes fields of the same object)
		const current = record;
		untrack(() => {
			savedRecord = checkIsNewRecord() ? null : deepCopy($state.snapshot(current));
		});
	});

	// The link of a subpage for the saved record: subpage field → the card field's value
	function linkValues(sub: Subpage): Record<string, any> {
		return Object.fromEntries(Object.entries(sub.link).map(([field, cardField]) => [field, savedRecord?.[cardField]]));
	}
	function linkFilters(sub: Subpage): FixedFilter[] {
		return Object.entries(linkValues(sub)).map(([field, value]) => ({ field, value }));
	}
	// Lines need a saved card record with its key
	function canShowLines(sub: Subpage): boolean {
		return !!savedRecord && Object.values(linkValues(sub)).every((v) => v !== undefined && v !== null && v !== '');
	}

	// The subpages' renderers by section name: Tab into their lines
	// (a plain object: a $state proxy hides the components' exported functions)
	const subpageRenderers: Record<string, { enterLines?: () => void } | undefined> = {};

	// Arrow keys move between the fields (like the list's cells): ←/→ to the previous/next field
	// at the start/end of the text, ↑/↓ to the field above/below — ↓ into the lines when they
	// come first. A field that keeps the key (an open dropdown, a textarea) has it.
	const arrowDirections: Record<string, ArrowDirection> = { ArrowUp: 'up', ArrowDown: 'down', ArrowLeft: 'left', ArrowRight: 'right' };
	function arrowNavigate(e: KeyboardEvent, container: HTMLElement): boolean {
		const direction = arrowDirections[e.key];
		if (!direction || e.shiftKey || e.ctrlKey || e.altKey || e.metaKey || e.defaultPrevented || e.isComposing) return false;
		const target = e.target as HTMLElement;
		if (!target.matches('input:not([type="hidden"]), select, textarea, [role="combobox"]')) return false;
		if (target.getAttribute('aria-expanded') === 'true') return false;
		if (!arrowLeavesField(target, direction)) return false;
		const controls = getFieldControls(container).filter((c) => !c.closest('.subpage-part') && c.offsetParent !== null);
		const next = fieldInDirection(controls, target, direction);
		if (direction === 'down') {
			const bottom = target.getBoundingClientRect().bottom;
			const nextTop = next ? next.getBoundingClientRect().top : Infinity;
			const part = Array.from(container.querySelectorAll<HTMLElement>('[data-subpage-part]')).find((p) => {
				const top = p.getBoundingClientRect().top;
				return top >= bottom - 1 && top < nextTop;
			});
			const renderer = part ? subpageRenderers[part.dataset.subpageName ?? ''] : undefined;
			if (renderer?.enterLines) {
				e.preventDefault();
				renderer.enterLines();
				return true;
			}
		}
		// ↑/↓ never change the value (a date's day); at the edge they stay put
		if (direction === 'up' || direction === 'down') e.preventDefault();
		if (!next) return direction === 'up' || direction === 'down';
		e.preventDefault();
		focusField(next);
		return true;
	}

	// Tab (and Enter, which moves like Tab) on the last field before a subpage goes into its
	// lines — the last field the user can reach, whichever is visible (hidden fields and the
	// fields' buttons do not count); a field that keeps the key (an open dropdown) has it
	function tabIntoLines(e: KeyboardEvent, container: HTMLElement): boolean {
		if ((e.key !== 'Tab' && e.key !== 'Enter') || e.shiftKey || e.ctrlKey || e.altKey || e.metaKey || e.defaultPrevented) return false;
		const target = e.target as HTMLElement;
		if (e.key === 'Enter' && target.tagName === 'TEXTAREA') return false;
		if (!target.matches('input, select, textarea, [role="combobox"]')) return false;
		const following = (a: Node, b: Node) => (a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING) !== 0;
		const part = Array.from(container.querySelectorAll<HTMLElement>('[data-subpage-part]')).find((p) => following(target, p));
		if (!part) return false;
		const reachable = (el: HTMLElement) => el.offsetParent !== null && el.tabIndex >= 0 && !(el as HTMLInputElement).disabled;
		const fieldsBetween = Array.from(container.querySelectorAll<HTMLElement>('input, select, textarea')).filter(
			(el) => el !== target && !el.closest('.subpage-part') && reachable(el) && following(target, el) && following(el, part)
		);
		if (fieldsBetween.length > 0) return false;
		const renderer = subpageRenderers[part.dataset.subpageName ?? ''];
		if (!renderer?.enterLines) return false;
		e.preventDefault();
		renderer.enterLines();
		return true;
	}

	// Parts of the card: its fields, then each subpage. F6 / Shift+F6 and (cards with subpages)
	// Ctrl+↓ / Ctrl+↑ move the cursor to the next/previous part; Ctrl+Shift+↓/↑ go to the
	// next/previous record. Handled before the parts' own keys (capture).
	let lastCardField: HTMLElement | null = null;
	function handleFocusIn(e: FocusEvent) {
		const target = e.target as HTMLElement;
		if (!target.closest('.subpage-part') && target.matches('input, select, textarea, button')) lastCardField = target;
	}
	function currentPart(): number {
		const parts = subpageParts();
		const active = document.activeElement;
		return parts.findIndex((p) => p.contains(active)) + 1; // 0 = the card's fields
	}
	function subpageParts(): HTMLElement[] {
		return Array.from(cardPageElement?.querySelectorAll<HTMLElement>('[data-subpage-part]') ?? []);
	}
	function focusPart(index: number) {
		const parts = subpageParts();
		const count = parts.length + 1;
		const i = ((index % count) + count) % count;
		if (i === 0) {
			const field =
				lastCardField && cardPageElement?.contains(lastCardField)
					? lastCardField
					: sectionsElement?.querySelector<HTMLElement>('.section-fields input, .section-fields select, .section-fields button');
			(field ?? cardPageElement)?.focus();
			return;
		}
		const part = parts[i - 1];
		// Into the lines as Tab does: the first cell of the first line (the card in edit mode;
		// an empty subpage starts a line), else the first line selected
		const renderer = subpageRenderers[part.dataset.subpageName ?? ''];
		if (renderer?.enterLines && part.querySelector('.list-page')) {
			part.scrollIntoView({ block: 'nearest' });
			renderer.enterLines();
			return;
		}
		// The folded section's header
		const target =
			part.querySelector<HTMLElement>('.list-page') ??
			part.closest('.card')?.querySelector<HTMLElement>('.card-header') ??
			part;
		target.focus();
		target.scrollIntoView({ block: 'nearest' });
	}
	function handlePartKeys(e: KeyboardEvent) {
		const arrow = e.key === 'ArrowDown' ? 1 : e.key === 'ArrowUp' ? -1 : 0;
		if (e.ctrlKey && e.shiftKey && !e.altKey && arrow !== 0 && navigationEnabled) {
			e.preventDefault();
			e.stopPropagation();
			if (arrow > 0 && canNavigateNext) onNavigateNext?.();
			if (arrow < 0 && canNavigatePrevious) onNavigatePrevious?.();
			return;
		}
		if (!hasSubpages) return;
		const f6 = e.key === 'F6' && !e.ctrlKey && !e.altKey;
		const ctrlArrow = e.ctrlKey && !e.shiftKey && !e.altKey && arrow !== 0;
		if (!f6 && !ctrlArrow) return;
		e.preventDefault();
		e.stopPropagation();
		const step = f6 ? (e.shiftKey ? -1 : 1) : arrow;
		focusPart(currentPart() + step);
	}
	$effect(() => {
		const element = cardPageElement;
		if (!element) return;
		element.addEventListener('keydown', handlePartKeys, true);
		element.addEventListener('focusin', handleFocusIn);
		return () => {
			element.removeEventListener('keydown', handlePartKeys, true);
			element.removeEventListener('focusin', handleFocusIn);
		};
	});

	// Folded sections (FastTabs), remembered per page in the browser
	const collapsedKey = $derived(`card-collapsed-sections-${page.page.id}`);
	let collapsedSections = $state<Record<string, boolean>>({});
	$effect(() => {
		collapsedSections = getJson<Record<string, boolean>>(collapsedKey, {});
	});
	function setCollapsed(section: string, collapsed: boolean) {
		collapsedSections = { ...collapsedSections, [section]: collapsed };
		setJson(collapsedKey, collapsedSections);
	}

	// Get customized sections (fields reorganized by user preferences)
	const customizedSections = $derived(() => {
		if (!page?.page?.layout?.sections) return [];

		// Create a map of section names to fields
		const sectionMap = new Map<string, Field[]>();

		// Initialize with empty arrays for all sections
		page.page.layout.sections.forEach(section => {
			sectionMap.set(section.name, []);
		});

		// Distribute fields to sections based on customizations
		let globalIndex = 0;
		page.page.layout.sections.forEach(section => {
			(section.fields ?? []).forEach(field => {
				// Get the target section (customized or original)
				const targetSection =
					(field.source in fieldCustomizations && fieldCustomizations[field.source].section)
						? fieldCustomizations[field.source].section!
						: section.name;

				// Get order (customized or original index)
				const order = fieldCustomizations[field.source]?.order ?? globalIndex;
				globalIndex++;

				// Add field to target section if visible with order info (a page trigger may hide it)
				if (isItemVisible(field, fieldCustomizations) && fieldStates[field.source]?.visible !== false) {
					const fields = sectionMap.get(targetSection) || [];
					fields.push({ field, order } as any);
					sectionMap.set(targetSection, fields as any);
				}
			});
		});

		// Convert map back to section array (only include non-empty sections)
		// Sort fields within each section by order
		return page.page.layout.sections
			.map(section => ({
				...section,
				fields: ((sectionMap.get(section.name) || []) as any[])
					.sort((a: any, b: any) => a.order - b.order)
					.map((item: any) => item.field)
			}))
			.filter(section => section.fields.length > 0 || !!section.subpage || !!section.period_grid || !!section.matrix);
	});

	// Open customize modal
	function handleCustomize() {
		customizeModalOpen = true;
	}

	// Save customizations
	function handleSaveCustomizations(customizations: Record<string, ItemCustomization>) {
		fieldCustomizations = customizations;
		const userId = $currentUser?.user_id || 'anonymous';
		savePageCustomizations(userId, page.page.id, customizations);
	}
</script>

<!-- The card's frame takes the focus from code only (its shortcuts work while no field has the
	focus): not a Tab stop, never outlined -->
<div class="card-page" role="group" use:shortcuts={shortcutMap()} tabindex="-1" bind:this={cardPageElement} onkeydown={handleFrameTab}>

	<!-- Edge Navigation Buttons (classic ERP style) -->
	{#if navigationEnabled}
		<NavigationButtons
			onPrevious={onNavigatePrevious}
			onNext={onNavigateNext}
			canNavigatePrevious={canNavigatePrevious}
			canNavigateNext={canNavigateNext}
		/>
	{/if}

	<!-- No title here: the modal's title bar or the breadcrumb shows the card title -->
	<PageHeader title="" {onclose}>
		{#snippet leftActions()}
			<!-- Save state indicator - fixed width container to prevent layout shift -->
			<div class="save-state-container">
				<div class="saving-indicator" class:visible={saveState === 'saving'}>
					<svg class="animate-spin h-4 w-4 text-blue-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
					</svg>
					<span class="text-sm text-gray-600 dark:text-gray-400">{t(MSG.SAVING)}</span>
				</div>
				<div class="saved-indicator" class:visible={saveState === 'saved'}>
					<svg class="h-4 w-4 text-green-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
					</svg>
					<span class="text-sm text-green-600 dark:text-green-400 font-medium">{t(MSG.SAVED)}</span>
				</div>
			</div>

			{#each page.page.actions?.filter((a) => a.promoted) || [] as action}
				{#if action.items}
					<ActionMenu {action} itemDisabled={(item) => !!item.run_object && checkIsNewRecord()} onselect={runCardAction} />
				{:else}
					<ActionButton
						caption={action.caption}
						image={action.image}
						shortcut={action.shortcut}
						tabindex={-1}
						onclick={() => runCardAction(action)}
						disabled={action.enabled === false}
					/>
				{/if}
			{/each}
		{/snippet}

		{#snippet rightActions()}
			<!-- Customize button -->
			<ActionButton caption={t(LIST.CUSTOMIZE)} image="Customize" tabindex={-1} onclick={handleCustomize} title={t(LIST.CUSTOMIZE_COLUMNS)} />

		{/snippet}
	</PageHeader>

	<!-- Error banner when save is blocked -->
	{#if saveBlocked}
		<div class="error-banner">
			<div class="error-content">
				<svg class="error-icon" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
				</svg>
				<span class="error-message">{saveBlockedMessage || t(ERR.SAVE_BLOCKED)}</span>
			</div>
			<div class="error-actions">
				<button class="error-clear-btn" onclick={() => onclearerror?.()}>
					{t(CARD.CLEAR_FORM)}
				</button>
			</div>
		</div>
	{/if}

	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="sections-container"
		bind:this={sectionsElement}
		onkeydown={(e) => {
			// Enter moves between the card's fields — not in a subpage (its list handles Enter)
			if ((e.target as HTMLElement).closest('.subpage-part')) return;
			if (tabIntoLines(e, e.currentTarget as HTMLElement)) return;
			if (arrowNavigate(e, e.currentTarget as HTMLElement)) return;
			handleFieldEnterKey(e, e.currentTarget as HTMLElement);
		}}
	>
		{#if isEmptyRecord() && !editMode}
			<!-- Empty state -->
			<div class="empty-state">
				<div class="empty-state-icon">
					<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
					</svg>
				</div>
				<h3 class="empty-state-title">{t(CARD.NO_RECORD)}</h3>
				<p class="empty-state-text">{t(CARD.SELECT_OR_CREATE)}</p>
				<Button variant="primary" onclick={() => handleAction('New')}>
					{#snippet icon()}
						<PlusIcon size={16} color="currentColor" />
					{/snippet}
					{t(CARD.CREATE_NEW)}
				</Button>
			</div>
		{:else}
			{@const allFields = customizedSections().flatMap(s => s.fields)}
			{#each customizedSections() as section}
				<Card
					collapsible
					bind:collapsed={
						() => collapsedSections[section.name] ?? false,
						(v) => setCollapsed(section.name, v)
					}
				>
					{#snippet header()}
						<h3 class="section-title text-heading dark:text-blue-400">
							{section.caption}
						</h3>
					{/snippet}

					{#if section.period_grid}
						<!-- The record's FlowFields per period (or per record of the grid's rows table) -->
						{#if !checkIsNewRecord()}
							<PeriodGrid
								pageId={page.page.id}
								section={section.name}
								grid={section.period_grid}
								{record}
								{captions}
								ondrilldown={(pageId, filters) => { drilldown = { pageId, filters }; }}
							/>
						{:else}
							<p class="subpage-empty">{t(CARD.SAVE_FIRST)}</p>
						{/if}
					{:else if section.matrix}
						<!-- A table's records against periods (the card record filters the cells) -->
						{#if !checkIsNewRecord()}
							<MatrixGrid
								pageId={page.page.id}
								section={section.name}
								matrix={section.matrix}
								{record}
								ondrilldown={(pageId, filters) => { drilldown = { pageId, filters }; }}
							/>
						{:else}
							<p class="subpage-empty">{t(CARD.SAVE_FIRST)}</p>
						{/if}
					{:else if section.subpage}
						{@const sub = section.subpage}
						<!-- A subpage: the list page with the lines of this record (its own keys) -->
						<div class="subpage-part" data-subpage-part data-subpage-name={section.name} data-own-keys>
							{#if canShowLines(sub)}
								{#key JSON.stringify(linkValues(sub))}
									<PageRenderer
										bind:this={subpageRenderers[section.name]}
										pageid={sub.page_id}
										subpage={true}
										parentEditMode={editMode}
										fixedFilters={linkFilters(sub)}
										newRecordDefaults={linkValues(sub)}
										onexit={() => focusPart(0)}
										onlineschange={onrefresh}
										refreshToken={linesRefresh}
									/>
								{/key}
							{:else}
								<p class="subpage-empty">{t(CARD.SAVE_FIRST)}</p>
							{/if}
						</div>
					{:else}
					<div class="section-fields">
						{#each section.fields as field (field.source)}
							{@const fieldEditable = field.editable !== false && fieldStates[field.source]?.editable !== false && editMode && !saveBlocked}
							{@const fieldTabIndex = allFields.findIndex(f => f.source === field.source) + 1}
							{#if field.calculated}
								<!-- A calculated field: the page trigger's value, read-only -->
								<FieldRenderer
									{field}
									value={calculatedValues[field.source]}
									caption={getFieldCaption(field.source, captions, field.caption)}
									editable={false}
									fieldType={fieldTypes[field.source]}
									fieldCaptions={captions}
									tabindex={fieldTabIndex}
									ondrilldown={field.drilldown && !checkIsNewRecord() ? () => openDrilldown(field) : undefined}
								/>
							{:else if isBlobType(fieldTypes[field.source])}
								<!-- A picture (BLOB): its own endpoint; a new record must be saved first -->
								{#if !checkIsNewRecord()}
									<PictureField
										table={page.page.source_table}
										field={field.source}
										recordId={getRecordId(record, primaryKeyField, getPrimaryKeyFields(page)) ?? ''}
										caption={getFieldCaption(field.source, captions, field.caption)}
										hasPicture={!!record[field.source]}
										editable={field.editable !== false && !saveBlocked}
										onchange={(has) => (record[field.source] = has)}
									/>
								{/if}
							{:else}
							<FieldRenderer
								{field}
								bind:value={record[field.source]}
								caption={getFieldCaption(field.source, captions, field.caption)}
								editable={fieldEditable}
								fieldType={fieldTypes[field.source]}
								options={options[field.source]}
								lookups={lookupFor(lookups, field.source, record)}
								fieldCaptions={captions}
								tabindex={fieldTabIndex}
								onblur={(e) => handleFieldBlur(field, e)}
								onassistedit={field.assist_edit && fieldEditable ? () => handleAssistEdit(field.assist_edit!) : undefined}
								ondrilldown={field.drilldown && !fieldEditable && !checkIsNewRecord() ? () => openDrilldown(field) : undefined}
							/>
							{/if}
						{/each}
					</div>
					{/if}
				</Card>
			{/each}
		{/if}
	</div>
</div>

<!-- A field's drilldown: the entries behind the value, over the card -->
{#if drilldown}
	{@const d = drilldown}
	<Modal open={true} size="expanded" onclose={() => (drilldown = null)}>
		<div class="drilldown-page">
			<PageRenderer pageid={d.pageId} embedded={true} initialFilters={d.filters} onclose={() => (drilldown = null)} />
		</div>
	</Modal>
{/if}

<!-- The card's page objects: progress, questions, input, message boxes -->
<ObjectRunner bind:this={runner} ondone={handleRunDone} />

<!-- Customize Page Modal -->
{#if customizeModalOpen}
	<CustomizeFieldsModal
		open={customizeModalOpen}
		{page}
		customizations={fieldCustomizations}
		mode="card"
		onclose={() => customizeModalOpen = false}
		onsave={handleSaveCustomizations}
	/>
{/if}

<style>
	.drilldown-page {
		height: 75vh;
	}

	.card-page {
		@apply flex flex-col flex-1 min-h-0;
		outline: none;
		@apply relative;
		@apply bg-surface;
		/* The sections scroll inside the window (as the list page) */
		max-height: calc(100vh - 180px);
	}

	:global(.dark) .card-page {
		background-color: rgb(var(--gray-900)); /* gray-900 */
	}



	.sections-container {
		@apply flex flex-col gap-2 overflow-y-auto flex-1 min-h-0;
		@apply px-1; /* Small padding for scrollbar spacing */
	}

	/* A subpage inside its section */
	.subpage-part {
		@apply -mx-2;
	}

	.subpage-empty {
		@apply text-sm text-gray-500 dark:text-gray-400 py-2;
	}

	.section-fields {
		@apply grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-1.5;
	}

	/* Compact card: section headers, inputs and values take less vertical space */
	.section-title {
		@apply text-base font-semibold;
	}

	.card-page :global(.card-header) {
		@apply py-1.5;
	}

	.card-page :global(.card-body) {
		@apply px-4 py-2;
	}

	.card-page :global(.field-group) {
		gap: 0.125rem;
	}

	.card-page :global(.field-group .field-label) {
		@apply text-xs;
	}

	.card-page :global(.field-group input.input),
	.card-page :global(.field-group select.select),
	.card-page :global(.field-group .field-value),
	.card-page :global(.option-dropdown .option-trigger) {
		@apply py-1 px-2.5 text-sm;
		min-height: 2rem;
	}

	.card-page :global(.field-group .checkbox-wrapper) {
		@apply py-1;
		min-height: 2rem;
	}

	/* Save state container - fixed width to prevent layout shift */
	.save-state-container {
		@apply relative;
		width: 80px; /* Fixed width to accommodate "Saving..." text */
		height: 32px;
	}

	.saving-indicator,
	.saved-indicator {
		@apply flex items-center gap-2;
		@apply absolute inset-0;
		@apply opacity-0 transition-opacity duration-200;
		pointer-events: none;
	}

	.saving-indicator.visible,
	.saved-indicator.visible {
		@apply opacity-100;
		pointer-events: auto;
	}

	/* Empty state styles */
	.empty-state {
		@apply flex flex-col items-center justify-center;
		@apply py-16 px-8;
		@apply text-center;
	}

	.empty-state-icon {
		@apply w-16 h-16 mb-4;
		@apply text-gray-300;
	}

	:global(.dark) .empty-state-icon {
		color: rgb(var(--gray-600)); /* gray-600 */
	}

	.empty-state-icon svg {
		@apply w-full h-full;
	}

	.empty-state-title {
		@apply text-xl font-semibold text-gray-700 mb-2;
	}

	:global(.dark) .empty-state-title {
		color: rgb(var(--gray-300)); /* gray-300 */
	}

	.empty-state-text {
		@apply text-gray-500 mb-6;
	}

	:global(.dark) .empty-state-text {
		color: rgb(var(--gray-400)); /* gray-400 */
	}

	.card-page :global(.edge-nav-btn:not(:disabled):hover) {
		@apply shadow-xl;
	}

	/* Ensure PageHeader stays at top */
	.card-page :global(.page-header) {
		@apply sticky top-0 z-10 bg-surface;
		@apply mb-4;
	}

	:global(.dark) .card-page :global(.page-header) {
		background-color: rgb(var(--gray-900)); /* gray-900 */
	}

	/* Error banner styles */
	.error-banner {
		@apply flex items-center justify-between;
		@apply mx-4 mb-4 px-4 py-3;
		@apply bg-red-50 border border-red-200 rounded-lg;
	}

	:global(.dark) .error-banner {
		background-color: rgba(127, 29, 29, 0.3); /* red-900 with opacity */
		border-color: #991b1b; /* red-800 */
	}

	.error-content {
		@apply flex items-center gap-3;
	}

	.error-icon {
		@apply w-5 h-5 text-red-600 flex-shrink-0;
	}

	:global(.dark) .error-icon {
		color: #f87171; /* red-400 */
	}

	.error-message {
		@apply text-sm text-red-700 font-medium;
	}

	:global(.dark) .error-message {
		color: #fca5a5; /* red-300 */
	}

	.error-actions {
		@apply flex items-center gap-2;
	}

	.error-clear-btn {
		@apply px-3 py-1.5 text-sm font-medium;
		@apply bg-red-600 text-white rounded;
		@apply hover:bg-red-700 transition-colors;
	}

	:global(.dark) .error-clear-btn {
		background-color: #dc2626; /* red-600 */
	}

	:global(.dark) .error-clear-btn:hover {
		background-color: #b91c1c; /* red-700 */
	}
</style>
