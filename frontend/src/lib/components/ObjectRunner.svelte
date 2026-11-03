<script lang="ts">
	// Runs a page object — a codeunit, a report or a registered function (run_object) — with a
	// record, through the job API, and shows its dialogs: progress, questions, input / StrMenu /
	// request pages, a page it opens (RunModal) and its message box. Used by card pages; list
	// pages have the same dialogs inline.
	import Modal from './Modal.svelte';
	import ProgressModal from './ProgressModal.svelte';
	import PageRenderer from './pages/PageRenderer.svelte';
	import { startJob, type ErrorDetails } from '$lib/services/jobs';
	import { printoutOf } from '$lib/utils/printout';
	import { goto } from '$app/navigation';
	import { openPagePath } from '$lib/utils/runPage';
	import { toast } from '$lib/stores/toast';
	import { t, MSG, ERR, BTN } from '$lib/services/i18n.svelte';
	import type { DialogResult } from '$lib/types/api';
	import type { RequestDataItem, RequestInputField } from '$lib/types/pages';

	interface Props {
		// After a run: refresh = the result asked for it (or an async job ran)
		ondone?: (refresh: boolean) => void;
	}
	let { ondone }: Props = $props();

	let progressOpen = $state(false);
	let progressTitle = $state('');
	let progressValue = $state(0);
	let progressMessage = $state('');
	let progressError = $state('');
	let progressErrorDetails = $state<ErrorDetails | null>(null); // what lies behind the error (Show Entries)
	let confirmMode = $state(false);
	let confirmMessage = $state('');
	let confirmCallback = $state<((r: boolean) => void) | undefined>(undefined);
	let inputMode = $state(false);
	let inputFields = $state<RequestInputField[]>([]);
	let inputDataItems = $state<RequestDataItem[]>([]);
	let optionsCaption = $state('');
	let printed = $state(false); // a printed report's request page
	let defaultOutput = $state('preview'); // its first output (Printer Selection)
	let inputCallback = $state<((v: Record<string, string> | null) => void) | undefined>(undefined);
	let runPage = $state<{
		pageId: number;
		filters: import('$lib/types/api').TableFilter[];
		select?: string;
		lookupMode: boolean;
		params?: Record<string, string>; // the page's parameters (Page.SetParam)
		respond: (values: Record<string, string> | null) => void;
	} | null>(null);
	let dialog = $state<DialogResult | null>(null);
	// The page the result opens afterwards (open_page: the order a quote became), after its
	// message box
	let pendingOpen: string | null = null;

	function closeDialog() {
		dialog = null;
		openPending();
	}
	function openPending() {
		const path = pendingOpen;
		pendingOpen = null;
		if (path) goto(path);
	}

	/**
	 * Run a run_object: "codeunit:455", "report:93", a function "table:X.F(0)" — with the
	 * record (a card's record). quiet (AssistEdit): no progress window and no "completed"
	 * message, only the pages it opens and its errors. Returns the result's data (an AssistEdit
	 * function's changed record is data.record).
	 */
	export async function run(
		runObject: string,
		record: Record<string, unknown>,
		opts: { quiet?: boolean } = {}
	): Promise<Record<string, unknown> | undefined> {
		const [objectType, rest] = runObject.split(':');
		const isFunction = /^[a-z]+:[^.(]+\.[A-Za-z_]\w*(\(.*\))?$/.test(runObject);
		const isReport = objectType === 'report';
		const id = isFunction ? 1 : parseInt(rest ?? '', 10);
		if (!isFunction && (!['codeunit', 'report'].includes(objectType) || isNaN(id) || id <= 0)) {
			toast.error(t(ERR.UNKNOWN_OBJECT_TYPE, objectType));
			return;
		}

		const quiet = opts.quiet === true;
		let resultData: Record<string, unknown> | undefined;
		progressOpen = !quiet;
		progressTitle = t(MSG.PROCESSING);
		progressValue = 0;
		progressMessage = '';
		progressError = '';
		progressErrorDetails = null;
		confirmMode = false;
		inputMode = false;
		let refresh = false;
		try {
			const result = await startJob(
				id,
				record,
				{
					onProgress: (event) => {
						confirmMode = false;
						inputMode = false;
						progressValue = event.value;
						if (event.message) progressMessage = event.message;
					},
					onComplete: (event) => {
						confirmMode = false;
						inputMode = false;
						progressValue = 100;
						if (event.message) progressMessage = event.message;
						if (event.data?.refresh) refresh = true;
						resultData = event.data as Record<string, unknown> | undefined;
					},
					onError: (err, details) => {
						confirmMode = false;
						inputMode = false;
						progressError = err;
						progressErrorDetails = details ?? null;
					},
					onConfirm: (event, respond) => {
						progressOpen = true; // also a quiet run shows its questions
						confirmMode = true;
						confirmMessage = event.message;
						confirmCallback = (response: boolean) => {
							confirmMode = false;
							respond(response);
						};
					},
					onRunPage: (event, respond) => {
						const data = event.data ?? {};
						runPage = {
							pageId: Number(data.page_id),
							filters: (data.filters as import('$lib/types/api').TableFilter[]) ?? [],
							select: (data.select as string) || undefined,
							lookupMode: data.lookup_mode === true,
							params: (data.params as Record<string, string>) ?? undefined,
							respond
						};
					},
					onInputRequest: (event, respond) => {
						progressOpen = true;
						inputMode = true;
						inputFields = (event.data?.fields as typeof inputFields) || [];
						inputDataItems = (event.data?.data_items as RequestDataItem[]) || [];
						optionsCaption = (event.data?.options_caption as string) || '';
						printed = event.data?.printed === true;
						defaultOutput = (event.data?.default_output as string) || 'preview';
						if (event.message) progressTitle = event.message;
						inputCallback = (values: Record<string, string> | null) => {
							inputMode = false;
							inputFields = [];
							respond(values);
						};
					},
					onSyncResult: (syncResult) => {
						progressOpen = false;
						refresh = !!syncResult.refresh;
						resultData = syncResult.data;
						if (syncResult.success && quiet) {
							// quiet: no message for a plain success
						} else if (syncResult.success) {
							if (syncResult.dialog) dialog = syncResult.dialog;
							else toast.success(syncResult.message || t(MSG.CODEUNIT_SUCCESS));
						} else {
							toast.error(syncResult.message || t(ERR.CODEUNIT_FAILED));
						}
					}
				},
				undefined,
				isFunction ? 'function' : isReport ? 'report' : 'codeunit',
				isFunction ? runObject : undefined
			);
			if (result && 'job_id' in result) {
				if (!quiet) {
					await new Promise((resolve) => setTimeout(resolve, 500));
					// The codeunit's closing message box (Message), else a plain notice
					const finalDialog = (result as { data?: Record<string, unknown> }).data?.dialog as DialogResult | undefined;
					if (!progressError && finalDialog) dialog = finalDialog;
					else if (!progressError && !printoutOf((result as { data?: Record<string, unknown> }).data)) toast.success(t(MSG.JOB_COMPLETED));
					refresh = true; // an async job may have changed anything
				}
				progressOpen = false;
			}
			const opens = openPagePath(resultData?.open_page);
			pendingOpen = opens;
			if (opens && !dialog) openPending();
			// a page that opens another record leaves this one (it may be gone: the quote); a quiet
			// run reports only a refresh it asked for (a field's OnAfterValidate changed the record)
			if (quiet && resultData?.refresh === true) refresh = true;
			if (!opens && (!quiet || refresh)) ondone?.(refresh);
			return resultData;
		} catch (err) {
			// an error with details (the G/L entries of a posting that does not balance) stays in
			// the dialog with Show Entries; others are a toast
			if (progressErrorDetails) return;
			progressOpen = false;
			toast.error(err instanceof Error ? err.message : t(ERR.CODEUNIT_FAILED));
		}
	}

	// closes the dialog of an error with details
	function closeError() {
		progressOpen = false;
		progressError = '';
		progressErrorDetails = null;
	}
</script>

<ProgressModal
	open={progressOpen}
	title={progressTitle}
	message={progressMessage}
	progress={progressValue}
	error={progressError}
	errorDetails={progressErrorDetails}
	onCloseError={progressErrorDetails ? closeError : undefined}
	confirmMode={confirmMode}
	confirmMessage={confirmMessage}
	onConfirmResponse={confirmCallback}
	inputMode={inputMode}
	{inputFields}
	{inputDataItems}
	{optionsCaption}
	{printed}
	{defaultOutput}
	onInputResponse={inputCallback}
/>

<!-- A page the codeunit opened (RunModal) -->
{#if runPage}
	{@const request = runPage}
	<Modal open={true} size="expanded">
		<div class="run-page">
			<PageRenderer
				pageid={request.pageId}
				embedded={true}
				initialFilters={request.filters}
				initialSelect={request.select}
				lookupMode={request.lookupMode}
				openParams={request.params}
				onclose={(action, key) => {
					// respond first: {@const request} follows runPage, which is cleared next
					const respond = request.respond;
					respond(action === 'Cancel' ? null : { action, key: key ?? '' });
					runPage = null;
				}}
			/>
		</div>
	</Modal>
{/if}

<!-- The result's message box (Esc closes it) -->
{#if dialog}
	{@const d = dialog}
	<Modal open={true} onclose={closeDialog}>
		<div class="p-6">
			<h3 class="text-lg font-semibold text-gray-900 dark:text-white">{d.title}</h3>
			<p class="mt-2 text-gray-600 dark:text-gray-300" class:text-red-600={d.type === 'error'}>{d.message}</p>
			<div class="mt-6 flex justify-end">
				<button
					class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500"
					onclick={closeDialog}
				>
					{t(BTN.OK)}
				</button>
			</div>
		</div>
	</Modal>
{/if}

<style>
	.run-page {
		height: 75vh;
		display: flex;
		flex-direction: column;
	}
</style>
