<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch, type CSSProperties } from "vue";
import { useI18n } from "vue-i18n";
import type { ColumnInfo } from "../types";
import { colWidthAt } from "../utils/resultTable";

const props = defineProps<{
  columns: ColumnInfo[];
  visibleColumnIndexes: number[];
  gridStyle: string;
  keyIndexes: number[];
  searchColumn: number;
  searchNoMatch: boolean;
  searchText: string;
  virtualEnabled: boolean;
  virtualHeight: number;
  rowHeight: number;
  visibleRows: Array<{ ri: number; row: unknown[] }>;
  virtualRowTop: (ri: number) => number;
  parseStyle: (s: string) => Record<string, string>;
  editingRow: number;
  editingCol: number;
  editingValue: string;
  isPendingEdited: (ri: number, ci: number) => boolean;
  pendingValue: (ri: number, ci: number) => unknown;
  colWidths: number[];
}>();

const emit = defineEmits<{
  (e: "scroll-header", ev: Event): void;
  (e: "scroll-body", ev: Event): void;
  (e: "open-header-menu", ev: MouseEvent, ci: number): void;
  (e: "open-header-value-filter", ci: number): void;
  (e: "open-cell-menu", ev: MouseEvent, ri: number, ci: number): void;
  (e: "start-edit", ri: number, ci: number): void;
  (e: "commit-edit"): void;
  (e: "cancel-edit"): void;
  (e: "update:editing-value", value: string): void;
  (e: "resize-columns", ci: number, width: number): void;
}>();

const { t } = useI18n();
const headerScrollEl = ref<HTMLElement | null>(null);
const scrollEl = ref<HTMLElement | null>(null);

function cellAt(row: unknown[], pos: number): unknown {
  const ci = props.visibleColumnIndexes[pos];
  return ci === undefined ? undefined : row[ci];
}

const editInputEl = ref<HTMLInputElement | null>(null);

const resizing = ref<{ ci: number; startX: number; startWidth: number } | null>(null);

function startColumnResize(ev: PointerEvent, ci: number): void {
  ev.preventDefault();
  ev.stopPropagation();
  const target = ev.currentTarget as HTMLElement | null;
  target?.setPointerCapture?.(ev.pointerId);
  resizing.value = { ci, startX: ev.clientX, startWidth: colWidthAt(props.colWidths, ci) };
  document.body.style.cursor = "col-resize";
  document.body.style.userSelect = "none";
}

function moveColumnResize(ev: PointerEvent): void {
  const state = resizing.value;
  if (!state) return;
  const width = state.startWidth + (ev.clientX - state.startX);
  emit("resize-columns", state.ci, width);
}

function endColumnResize(): void {
  if (!resizing.value) return;
  resizing.value = null;
  document.body.style.cursor = "";
  document.body.style.userSelect = "";
}

onBeforeUnmount(() => {
  endColumnResize();
});

function setEditInput(el: any) {
  editInputEl.value = (el as HTMLInputElement | null) ?? null;
}

function inputValue(e: Event): string {
  return (e.target as HTMLInputElement).value;
}

defineExpose({ headerScrollEl, scrollEl, editInputEl });

watch(
  () => [props.editingRow, props.editingCol],
  ([row, col]) => {
    if (row < 0 || col < 0) {
      editInputEl.value = null;
      return;
    }
    nextTick(() => {
      if (editInputEl.value) {
        editInputEl.value.focus();
        editInputEl.value.select();
      }
    });
  },
);
</script>

<template>
  <div
    ref="headerScrollEl"
    @scroll="emit('scroll-header', $event)"
    class="overflow-hidden shrink-0 border-b border-ch-border bg-ch-panel"
  >
    <div class="grid" :style="gridStyle">
      <div
        v-for="(ci, pos) in visibleColumnIndexes"
        :key="'h-' + ci"
        class="group relative px-3 py-1.5 text-xs font-semibold text-ch-text border-r border-ch-border/40 last:border-r-0 truncate select-none"
        :title="columns[ci]?.name"
        @contextmenu.prevent="emit('open-header-menu', $event, ci)"
      >
        <div class="flex items-center gap-1.5">
          <span class="truncate">{{ columns[ci]?.name }}</span>
          <span
            v-if="keyIndexes.includes(ci)"
            class="text-[10px] text-ch-accent shrink-0"
            :title="t('editor.useAsEditKey')"
            >KEY</span
          >
          <span
            v-if="searchColumn === ci"
            class="text-[10px] text-ch-accent shrink-0"
            >🔍</span
          >
        </div>
        <div class="text-[10px] text-ch-muted font-normal truncate">
          {{ columns[ci]?.type }}
        </div>
        <button
          @click.stop="emit('open-header-value-filter', ci)"
          class="absolute right-1 top-1 opacity-0 group-hover:opacity-100 text-ch-muted hover:text-ch-accent bg-ch-panel rounded p-0.5"
          :title="t('editor.filterByValue')"
        >
          <svg
            width="10"
            height="10"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3" />
          </svg>
        </button>
        <div
          class="absolute top-0 right-0 h-full w-1.5 cursor-col-resize select-none"
          :class="
            resizing?.ci === ci
              ? 'bg-ch-accent/40'
              : 'bg-transparent hover:bg-ch-accent/30'
          "
          :title="t('editor.resizeColumnHint')"
          @pointerdown="startColumnResize($event, ci)"
          @pointermove="moveColumnResize"
          @pointerup="endColumnResize"
          @pointercancel="endColumnResize"
          @mousedown.stop
          @click.stop
        ></div>
      </div>
    </div>
  </div>

  <div
    ref="scrollEl"
    @scroll="emit('scroll-body', $event)"
    class="flex-1 min-h-0 overflow-auto"
  >
    <div
      v-if="searchNoMatch"
      class="flex items-center justify-center h-full text-xs text-ch-muted"
    >
      {{ t("editor.searchNoMatch", { query: searchText }) }}
    </div>
    <div
      v-else-if="virtualEnabled"
      class="relative"
      :style="{ height: virtualHeight + 'px' }"
    >
      <div
        v-for="item in visibleRows"
        :key="'r-' + item.ri"
        class="grid absolute left-0 w-full border-b border-ch-border/40 hover:bg-ch-hover"
        :style="{
          ...parseStyle(gridStyle),
          transform: `translateY(${virtualRowTop(item.ri)}px)`,
          height: rowHeight + 'px',
        }"
      >
        <div
          v-for="(ci, pos) in visibleColumnIndexes"
          :key="'vc-' + ci"
          class="px-3 text-xs text-ch-text border-r border-ch-border/40 last:border-r-0 truncate font-mono leading-none flex items-center"
          :title="cellAt(item.row, pos) !== null && cellAt(item.row, pos) !== undefined ? String(cellAt(item.row, pos)) : 'NULL'"
          @contextmenu.prevent="emit('open-cell-menu', $event, item.ri, ci)"
          @dblclick="emit('start-edit', item.ri, ci)"
        >
          <input
            v-if="editingRow === item.ri && editingCol === ci"
            :ref="setEditInput"
            :value="editingValue"
            @input="emit('update:editing-value', inputValue($event))"
            class="w-full h-5 box-border bg-ch-bg border border-ch-accent rounded px-1 text-xs leading-5 text-ch-text font-mono focus:outline-none"
            @keydown.enter.prevent="emit('commit-edit')"
            @keydown.esc.prevent="emit('cancel-edit')"
            @blur="emit('commit-edit')"
          />
          <template v-else>
            <span v-if="isPendingEdited(item.ri, ci)" class="text-amber-300">{{
              String(pendingValue(item.ri, ci))
            }}</span>
            <span
              v-else-if="cellAt(item.row, pos) === null || cellAt(item.row, pos) === undefined"
              class="text-ch-muted italic"
              >NULL</span
            >
            <template v-else>
              <span
                v-if="
                  typeof cellAt(item.row, pos) === 'string' &&
                  (String(cellAt(item.row, pos)).startsWith('{') || String(cellAt(item.row, pos)).startsWith('['))
                "
                class="text-blue-400"
                >{{ String(cellAt(item.row, pos)).slice(0, 100) }}</span
              >
              <span v-else>{{ String(cellAt(item.row, pos)) }}</span>
            </template>
          </template>
        </div>
      </div>
    </div>

    <template v-else>
      <div
        v-for="item in visibleRows"
        :key="'r-' + item.ri"
        class="grid border-b border-ch-border/40 hover:bg-ch-hover"
        :style="gridStyle"
      >
        <div
          v-for="(ci, pos) in visibleColumnIndexes"
          :key="'vc-' + ci"
          class="px-3 py-1.5 text-xs text-ch-text border-r border-ch-border/40 last:border-r-0 truncate font-mono"
          :title="cellAt(item.row, pos) !== null && cellAt(item.row, pos) !== undefined ? String(cellAt(item.row, pos)) : 'NULL'"
          @contextmenu.prevent="emit('open-cell-menu', $event, item.ri, ci)"
          @dblclick="emit('start-edit', item.ri, ci)"
        >
          <input
            v-if="editingRow === item.ri && editingCol === ci"
            :ref="setEditInput"
            :value="editingValue"
            @input="emit('update:editing-value', inputValue($event))"
            class="w-full h-5 box-border bg-ch-bg border border-ch-accent rounded px-1 text-xs leading-5 text-ch-text font-mono focus:outline-none"
            @keydown.enter.prevent="emit('commit-edit')"
            @keydown.esc.prevent="emit('cancel-edit')"
            @blur="emit('commit-edit')"
          />
          <template v-else>
            <span v-if="isPendingEdited(item.ri, ci)" class="text-amber-300">{{
              String(pendingValue(item.ri, ci))
            }}</span>
            <span
              v-else-if="cellAt(item.row, pos) === null || cellAt(item.row, pos) === undefined"
              class="text-ch-muted italic"
              >NULL</span
            >
            <template v-else>
              <span
                v-if="
                  typeof cellAt(item.row, pos) === 'string' &&
                  (String(cellAt(item.row, pos)).startsWith('{') || String(cellAt(item.row, pos)).startsWith('['))
                "
                class="text-blue-400"
                >{{ String(cellAt(item.row, pos)).slice(0, 100) }}</span
              >
              <span v-else>{{ String(cellAt(item.row, pos)) }}</span>
            </template>
          </template>
        </div>
      </div>
    </template>
  </div>
</template>