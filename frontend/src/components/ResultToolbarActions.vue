<script setup lang="ts">
import { useI18n } from "vue-i18n";

const props = defineProps<{
  querying: boolean;
  refreshEnabled: boolean;
  refreshSeconds: number;
  canExplain: boolean;
  explainMode: string;
  explainModes: Array<{ id: string; labelKey: string }>;
  hasColumns: boolean;
  exporting: boolean;
  filterEnabled: boolean;
  filterHasActive: boolean;
  activeFilterCount: number;
}>();

const emit = defineEmits<{
  (e: "manual-refresh"): void;
  (e: "toggle-refresh-menu", ev: MouseEvent): void;
  (e: "open-refresh-custom", ev: MouseEvent): void;
  (e: "explain"): void;
  (e: "update:explain-mode", value: string): void;
  (e: "toggle-export", ev: MouseEvent): void;
  (e: "open-filter", ev: MouseEvent): void;
}>();

const { t } = useI18n();

function onExplainMode(e: Event) {
  emit("update:explain-mode", (e.target as HTMLSelectElement).value);
}
</script>

<template>
  <div class="flex items-center gap-2 shrink-0">
    <div
      class="flex items-center rounded border border-ch-border overflow-hidden"
    >
      <button
        @click="emit('manual-refresh')"
        :disabled="props.querying"
        class="flex items-center gap-1 px-2 py-0.5 text-ch-muted hover:text-ch-text hover:bg-ch-hover disabled:opacity-40 disabled:hover:bg-transparent"
        :title="t('editor.refreshHint')"
      >
        <svg
          width="11"
          height="11"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        >
          <polyline points="23 4 23 10 17 10" />
          <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10" />
        </svg>
        <span>{{ t("editor.refresh") }}</span>
      </button>
      <div class="relative">
        <button
          @click.stop="emit('toggle-refresh-menu', $event)"
          class="flex items-center gap-1 px-2 py-0.5 border-l border-ch-border text-ch-muted hover:text-ch-text hover:bg-ch-hover"
          :title="t('editor.refreshInterval')"
        >
          <span>{{
            props.refreshEnabled
              ? t("editor.refreshSeconds", { seconds: props.refreshSeconds })
              : t("editor.refreshOff")
          }}</span>
          <svg
            width="9"
            height="9"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <polyline points="6 9 12 15 18 9" />
          </svg>
        </button>
      </div>
      <button
        @click.stop="emit('open-refresh-custom', $event)"
        class="px-1.5 py-0.5 border-l border-ch-border text-ch-muted hover:text-ch-text hover:bg-ch-hover"
        :title="t('editor.refreshCustom')"
      >
        <svg
          width="11"
          height="11"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        >
          <circle cx="12" cy="12" r="3" />
          <path
            d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"
          />
        </svg>
      </button>
    </div>

    <div
      class="flex items-center rounded border border-ch-border overflow-hidden"
    >
      <button
        @click="emit('explain')"
        :disabled="!props.canExplain || props.querying"
        class="flex items-center gap-1 px-2 py-0.5 text-ch-muted hover:text-ch-text hover:bg-ch-hover disabled:opacity-40 disabled:hover:bg-transparent"
        :title="t('editor.explainHint')"
      >
        <svg
          width="11"
          height="11"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        >
          <polyline points="22 12 18 12 15 21 9 3 6 12 2 12" />
        </svg>
        <span>{{ t("editor.explain") }}</span>
      </button>
      <select
        :value="props.explainMode"
        @change="onExplainMode"
        :disabled="!props.canExplain || props.querying"
        class="border-l border-ch-border bg-transparent text-ch-muted text-xs px-1 py-0.5 outline-none disabled:opacity-40"
      >
        <option v-for="m in props.explainModes" :key="m.id" :value="m.id">
          {{ t(m.labelKey) }}
        </option>
      </select>
    </div>

    <button
      @click.stop="emit('toggle-export', $event)"
      :disabled="!props.hasColumns || props.exporting"
      class="flex items-center gap-1 px-2 py-0.5 rounded border border-ch-border text-ch-muted hover:text-ch-text hover:bg-ch-hover disabled:opacity-40 disabled:hover:bg-transparent"
      :title="t('editor.exportHint')"
    >
      <svg
        width="11"
        height="11"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
      >
        <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
        <polyline points="7 10 12 15 17 10" />
        <line x1="12" y1="15" x2="12" y2="3" />
      </svg>
      <span>{{ t("editor.export") }}</span>
      <svg
        width="9"
        height="9"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
      >
        <polyline points="6 9 12 15 18 9" />
      </svg>
    </button>

    <button
      @click.stop="emit('open-filter', $event)"
      :disabled="!props.filterEnabled || props.querying"
      class="flex items-center gap-1 px-2 py-0.5 rounded border disabled:opacity-40 disabled:hover:bg-transparent"
      :class="
        props.filterHasActive
          ? 'border-ch-accent text-ch-accent'
          : 'border-ch-border text-ch-muted hover:text-ch-text hover:bg-ch-hover'
      "
      :title="
        props.filterEnabled
          ? t('editor.filterHint')
          : t('editor.filterDisabled')
      "
    >
      <svg
        width="11"
        height="11"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
      >
        <polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3" />
      </svg>
      <span>{{ t("editor.filter") }}</span>
      <span v-if="props.activeFilterCount > 0" class="text-[10px] opacity-80">{{
        props.activeFilterCount
      }}</span>
    </button>
  </div>
</template>
