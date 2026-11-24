/**
 * Drag and drop utility for reordering items in customization modals
 * Provides reactive state and handlers for drag-and-drop reordering
 */

import type { Field } from '#lib/types/pages.js';

export interface DragItem<T = any> {
	field: Field;
	data: T;
}

export interface DragState<T = any> {
	draggedItem: DragItem<T> | null;
	dragOverIndex: number | null;
}

/**
 * Create drag-and-drop state and handlers for reordering items
 * @returns Reactive drag state and handler functions
 */
export function createDragAndDrop<TCustomization extends { visible: boolean; order?: number }>() {
	// Drag state - using $state for reactivity
	let draggedItem = $state<DragItem | null>(null);
	let dragOverIndex = $state<number | null>(null);

	/**
	 * Handle drag start event
	 */
	function handleDragStart<T>(e: DragEvent, item: DragItem<T>) {
		e.dataTransfer!.effectAllowed = 'move';
		e.dataTransfer!.setData('text/html', ''); // Required for Firefox
		draggedItem = item;
	}

	/**
	 * Handle drag over event
	 */
	function handleDragOver(e: DragEvent, targetIndex: number) {
		e.preventDefault();
		e.dataTransfer!.dropEffect = 'move';
		dragOverIndex = targetIndex;
	}

	/**
	 * Handle drag leave event
	 */
	function handleDragLeave() {
		dragOverIndex = null;
	}

	/**
	 * Handle drop event - reorders items and updates customizations
	 * @param e - Drop event
	 * @param targetIndex - Index where the item should be dropped
	 * @param items - Current list of items (with field and data)
	 * @param customizations - Current customizations object
	 * @returns Updated customizations object with new order
	 */
	function handleDrop<T>(
		e: DragEvent,
		targetIndex: number,
		items: Array<DragItem<T>>,
		customizations: Record<string, TCustomization>
	): Record<string, TCustomization> {
		e.preventDefault();

		if (!draggedItem) return customizations;

		// Find current position of dragged item
		const fromIndex = items.findIndex(item => item.field.source === draggedItem!.field.source);

		// Same position - no change needed
		if (fromIndex === targetIndex) {
			draggedItem = null;
			dragOverIndex = null;
			return customizations;
		}

		// Create a new customizations object to trigger reactivity
		const newCustomizations: Record<string, TCustomization> = {};

		// Copy existing customizations
		Object.keys(customizations).forEach(key => {
			newCustomizations[key] = { ...customizations[key] };
		});

		// Reorder: remove from old position, insert at new position
		const reorderedItems = [...items];
		const [movedItem] = reorderedItems.splice(fromIndex, 1);
		reorderedItems.splice(targetIndex, 0, movedItem);

		// Assign new order to all items
		reorderedItems.forEach((item, newIndex) => {
			if (!newCustomizations[item.field.source]) {
				newCustomizations[item.field.source] = { visible: true } as TCustomization;
			}
			newCustomizations[item.field.source].order = newIndex;
		});

		// Reset drag state
		draggedItem = null;
		dragOverIndex = null;

		return newCustomizations;
	}

	/**
	 * Handle drag end event - cleanup
	 */
	function handleDragEnd() {
		draggedItem = null;
		dragOverIndex = null;
	}

	/**
	 * Check if an item is currently being dragged
	 */
	function isDragging(fieldSource: string): boolean {
		return draggedItem?.field.source === fieldSource;
	}

	/**
	 * Check if an index is currently being dragged over
	 */
	function isDragOver(index: number): boolean {
		return dragOverIndex === index;
	}

	return {
		// Reactive state getters
		get draggedItem() { return draggedItem; },
		get dragOverIndex() { return dragOverIndex; },

		// Handler functions
		handleDragStart,
		handleDragOver,
		handleDragLeave,
		handleDrop,
		handleDragEnd,

		// Helper functions
		isDragging,
		isDragOver
	};
}
