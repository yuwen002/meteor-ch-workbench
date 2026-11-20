<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ColumnInfo } from '../types'
import {
  type FilterCondition,
  type FilterJoin,
  type FilterOp,
  type FilterRule,
  FILTER_OP_ORDER,
  FILTER_JOIN_ORDER,
  FILTER_JOIN_AND,
  FILTER_EQ,
  FILTER_LIKE,
  FILTER_NOT_LIKE,
  isNumericType,
  opSymbol,
  buildFilterWhere,
} from '../utils/resultFilter'

const props = defineProps<{
  visible: boolean
  columns: ColumnInfo[]
  conditions: FilterCondition[]
  error: string | null
  applyDisabled: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'update:conditions', value: FilterCondition[]): void
  (e: 'update:error', value: string | null): void
  (e: 'apply'): void
  (e: 'clear'): void
}>()

const { t } = useI18n()

const columnValues = computed(() => props.columns.map((c) => c.name))

const wherePreview = computed(() => buildFilterWhere(props.conditions))

function columnType(name: string): string | undefined {
  return props.columns.find((c) => c.name === name)?.type
}

function columnIsNumeric(name: string): boolean {
  return isNumericType(columnType(name))
}

function ruleIsLike(op: FilterOp): boolean {
  return op === FILTER_LIKE || op === FILTER_NOT_LIKE
}

function rulePlaceholder(op: FilterOp, numeric: boolean): string {
  if (ruleIsLike(op)) return t('editor.filterLikePlaceholder')
  return numeric ? t('editor.filterNumericPlaceholder') : t('editor.filterValuePlaceholder')
}

function emitConditions(next: FilterCondition[]) {
  emit('update:conditions', next)
}

function createRule(): FilterRule {
  return { op: FILTER_EQ, value: '' }
}

function createCondition(): FilterCondition {
  const column = columnValues.value[0] ?? ''
  const rule = createRule()
  rule.numeric = columnIsNumeric(column)
  return {
    column,
    join: FILTER_JOIN_AND,
    rules: [rule],
  }
}

function addCondition() {
  emitConditions([...props.conditions, createCondition()])
}

function removeCondition(index: number) {
  const next = props.conditions.filter((_, i) => i !== index)
  emitConditions(next.length === 0 ? [createCondition()] : next)
}

function addRule(ci: number) {
  const cond = props.conditions[ci]
  if (!cond) return
  const rule = createRule()
  rule.numeric = columnIsNumeric(cond.column)
  const next = props.conditions.map((c, i) =>
    i === ci ? { ...c, rules: [...c.rules, rule] } : c
  )
  emitConditions(next)
}

function removeRule(ci: number, ri: number) {
  const cond = props.conditions[ci]
  if (!cond) return
  const rules = cond.rules.filter((_, i) => i !== ri)
  const next = props.conditions.map((c, i) =>
    i === ci ? { ...c, rules: rules.length === 0 ? [createRule()] : rules } : c
  )
  emitConditions(next)
}

function setConditionJoin(ci: number, join: FilterJoin) {
  emitConditions(props.conditions.map((c, i) => (i === ci ? { ...c, join } : c)))
}

function setConditionColumn(ci: number, column: string) {
  const numeric = columnIsNumeric(column)
  emitConditions(
    props.conditions.map((c, i) =>
      i === ci
        ? { ...c, column, rules: c.rules.map((r) => ({ ...r, numeric })) }
        : c
    )
  )
}

function setRuleOp(ci: number, ri: number, op: FilterOp) {
  emitConditions(
    props.conditions.map((c, i) =>
      i === ci ? { ...c, rules: c.rules.map((r, j) => (j === ri ? { ...r, op } : r)) } : c
    )
  )
}

function setRuleValue(ci: number, ri: number, value: string) {
  emitConditions(
    props.conditions.map((c, i) =>
      i === ci ? { ...c, rules: c.rules.map((r, j) => (j === ri ? { ...r, value } : r)) } : c
    )
  )
}

function toggleRuleNumeric(ci: number, ri: number, numeric: boolean) {
  emitConditions(
    props.conditions.map((c, i) =>
      i === ci ? { ...c, rules: c.rules.map((r, j) => (j === ri ? { ...r, numeric } : r)) } : c
    )
  )
}

function clear() {
  emit('clear')
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="visible"
      class="fixed inset-0 z-[2000] bg-black/50 flex items-center justify-center"
      @click.self="emit('close')"
    >
      <div class="bg-ch-panel border border-ch-border rounded-lg shadow-2xl w-[680px] max-w-[94vw] max-h-[86vh] flex flex-col overflow-hidden">
        <div class="flex items-center justify-between px-4 py-3 border-b border-ch-border shrink-0">
          <span class="text-ch-text text-sm font-medium">{{ t('editor.filterTitle') }}</span>
          <button
            class="text-ch-muted hover:text-ch-text"
            :title="t('common.cancel')"
            @click="emit('close')"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
          </button>
        </div>
        <div class="flex-1 overflow-y-auto px-4 py-3 space-y-2">
          <p class="text-[11px] text-ch-muted leading-relaxed">{{ t('editor.filterTip') }}</p>
          <div
            v-if="wherePreview"
            class="border border-ch-border rounded-md bg-ch-bg/60 px-2 py-1.5"
          >
            <div class="text-[10px] text-ch-muted">{{ t('editor.filterActiveTitle') }}</div>
            <div class="text-[11px] text-ch-accent font-mono break-all leading-relaxed">{{ wherePreview }}</div>
          </div>
          <div
            v-for="(cond, ci) in conditions"
            :key="'cond-' + ci"
            class="border border-ch-border rounded-md p-2 space-y-1.5 bg-ch-bg/40"
          >
            <div class="flex items-center gap-2">
              <select
                :value="cond.join"
                class="bg-ch-bg border border-ch-border rounded px-1 py-0.5 text-[11px] text-ch-text outline-none focus:border-ch-accent disabled:opacity-50"
                :disabled="ci === 0"
                :title="t('editor.filterJoinHint')"
                @change="setConditionJoin(ci, ($event.target as HTMLSelectElement).value as FilterJoin)"
              >
                <option v-for="j in FILTER_JOIN_ORDER" :key="j" :value="j">{{ j }}</option>
              </select>
              <select
                :value="cond.column"
                class="bg-ch-bg border border-ch-border rounded px-1 py-0.5 text-[11px] text-ch-text outline-none focus:border-ch-accent min-w-[120px]"
                @change="setConditionColumn(ci, ($event.target as HTMLSelectElement).value)"
              >
                <option v-for="name in columnValues" :key="name" :value="name">{{ name }}</option>
              </select>
              <button
                class="ml-auto text-ch-muted hover:text-ch-text"
                :title="t('editor.filterRemoveGroup')"
                @click="removeCondition(ci)"
              >
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="5" y1="12" x2="19" y2="12"/></svg>
              </button>
            </div>
            <div
              v-for="(rule, ri) in cond.rules"
              :key="'rule-' + ci + '-' + ri"
              class="flex items-center gap-2 pl-4"
            >
              <span class="text-[10px] text-ch-muted shrink-0 w-6">{{ ri === 0 ? t('editor.filterWhere') : 'OR' }}</span>
              <select
                :value="rule.op"
                class="bg-ch-bg border border-ch-border rounded px-1 py-0.5 text-[11px] text-ch-text outline-none focus:border-ch-accent"
                @change="setRuleOp(ci, ri, ($event.target as HTMLSelectElement).value as FilterOp)"
              >
                <option v-for="op in FILTER_OP_ORDER" :key="op" :value="op">{{ opSymbol(op) }}</option>
              </select>
              <input
                :value="rule.value"
                type="text"
                class="flex-1 min-w-0 bg-ch-bg border border-ch-border rounded px-1.5 py-0.5 text-[11px] text-ch-text outline-none focus:border-ch-accent font-mono"
                :placeholder="rulePlaceholder(rule.op, rule.numeric === true)"
                @input="setRuleValue(ci, ri, ($event.target as HTMLInputElement).value)"
                @keydown.enter.prevent="emit('apply')"
              />
              <button
                v-if="!ruleIsLike(rule.op) && columnIsNumeric(cond.column)"
                class="shrink-0 px-1 py-0.5 rounded border text-[10px] leading-4"
                :class="rule.numeric ? 'border-ch-accent text-ch-accent' : 'border-ch-border text-ch-muted hover:text-ch-text'"
                :title="t('editor.filterNumericToggleHint')"
                @click="toggleRuleNumeric(ci, ri, !rule.numeric)"
              >
                {{ rule.numeric ? t('editor.filterNumeric') : t('editor.filterText') }}
              </button>
              <button
                class="text-ch-muted hover:text-ch-text shrink-0"
                :title="t('editor.filterRemoveRule')"
                @click="removeRule(ci, ri)"
              >
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
              </button>
            </div>
            <button
              class="ml-10 text-[11px] text-ch-accent hover:underline"
              @click="addRule(ci)"
            >
              + {{ t('editor.filterAddRule') }}
            </button>
          </div>
          <button
            class="text-[11px] text-ch-accent hover:underline w-full text-left"
            @click="addCondition"
          >
            + {{ t('editor.filterAddGroup') }}
          </button>
          <p v-if="error" class="text-[11px] text-red-400">{{ error }}</p>
        </div>
        <div class="flex items-center gap-2 px-4 py-3 border-t border-ch-border shrink-0">
          <button
            class="px-2 py-1 rounded text-ch-muted hover:text-ch-text hover:bg-ch-hover text-xs"
            @click="clear"
          >
            {{ t('editor.filterReset') }}
          </button>
          <div class="ml-auto flex items-center gap-2">
            <button
              class="px-3 py-1 rounded border border-ch-border text-ch-muted hover:text-ch-text hover:bg-ch-hover text-xs"
              @click="emit('close')"
            >
              {{ t('common.cancel') }}
            </button>
            <button
              class="px-3 py-1 rounded bg-ch-accent text-white text-xs disabled:opacity-50"
              :disabled="applyDisabled"
              @click="emit('apply')"
            >
              {{ t('editor.filterApply') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>