import { toast } from '$lib/stores/toast';

/** The header the API sends a request's notes in (the server's middleware.NotesHeader) */
export const NOTES_HEADER = 'X-App-Notes';

/**
 * The notes of an API answer: texts the server's code told the user without stopping (a
 * number series reaching its Warning No.) — a JSON array, URL-encoded (form encoding: `+` is a
 * space)
 */
export function notesOf(response: Response): string[] {
	const header = response.headers?.get(NOTES_HEADER);
	if (!header) return [];
	try {
		const notes = JSON.parse(decodeURIComponent(header.replace(/\+/g, ' ')));
		return Array.isArray(notes) ? notes.filter((n): n is string => typeof n === 'string' && n !== '') : [];
	} catch {
		return [];
	}
}

/** Shows an API answer's notes to the user */
export function showNotes(response: Response): void {
	for (const note of notesOf(response)) toast.info(note, 8000);
}
