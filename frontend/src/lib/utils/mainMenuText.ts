/**
 * The company's own text at the top of the main menu (table 907 Main Menu Text, entered on page
 * 914 from Company Information): lines of plain text with a style. No lines: the main menu
 * shows its standard welcome text.
 */
export interface MainMenuTextLine {
	text: string;
	style: 'normal' | 'heading' | 'large' | 'small';
	bold: boolean;
	italic: boolean;
}

const styleClasses: Record<MainMenuTextLine['style'], string> = {
	heading: 'text-3xl font-bold text-heading dark:text-blue-400',
	large: 'text-xl text-fg',
	normal: 'text-gray-600 dark:text-gray-400',
	small: 'text-sm text-gray-600 dark:text-gray-400'
};

/** The classes of a line: its style, then bold and italic (an unknown style is normal) */
export function mainMenuTextClass(line: MainMenuTextLine): string {
	const classes = [styleClasses[line.style] ?? styleClasses.normal];
	if (line.bold) classes.push('font-bold');
	if (line.italic) classes.push('italic');
	return classes.join(' ');
}
