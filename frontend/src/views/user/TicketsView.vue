<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-5">
      <template v-if="ticketId">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <button class="btn btn-ghost" type="button" @click="router.push('/tickets')">
            <Icon name="arrowLeft" size="sm" />
            {{ t('common.back') }}
          </button>
          <button v-if="ticket && ticket.status !== 'closed'" class="btn btn-secondary" type="button" @click="showCloseConfirm = true">
            <Icon name="xCircle" size="sm" />
            {{ t('tickets.closeTicket') }}
          </button>
        </div>

        <div v-if="loadingDetail" class="flex justify-center py-16"><Icon name="refresh" class="animate-spin text-primary-600" /></div>
        <template v-else-if="ticket">
          <header class="border-b border-gray-200 pb-4 dark:border-dark-700">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div>
                <div class="mb-2 flex items-center gap-2">
                  <span class="text-sm text-gray-500">#{{ ticket.id }}</span>
                  <TicketBadge :value="ticket.status" />
                </div>
                <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ ticket.subject }}</h1>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t(`tickets.category.${ticket.category}`) }} · {{ formatDate(ticket.created_at) }}</p>
              </div>
            </div>
          </header>

          <section class="min-h-64 py-2">
            <TicketConversation :messages="ticket.messages || []" />
          </section>

          <section v-if="ticket.status !== 'closed'" class="border-t border-gray-200 pt-5 dark:border-dark-700">
            <TicketComposer
              :key="composerKey"
              :submitting="submitting"
              :placeholder="t('tickets.replyPlaceholder')"
              :submit-label="t('tickets.sendReply')"
              @submit="sendReply"
            />
          </section>
          <div v-else class="rounded-md border border-gray-200 bg-gray-50 px-4 py-3 text-sm text-gray-600 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300">
            {{ t('tickets.closedNotice') }}
          </div>
        </template>
      </template>

      <template v-else>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex items-center gap-2">
            <select v-model="statusFilter" class="input w-44" @change="page = 1; loadList()">
              <option value="">{{ t('common.all') }}</option>
              <option value="open">{{ t('tickets.status.open') }}</option>
              <option value="in_progress">{{ t('tickets.status.in_progress') }}</option>
              <option value="waiting_user">{{ t('tickets.status.waiting_user') }}</option>
              <option value="closed">{{ t('tickets.status.closed') }}</option>
            </select>
          </div>
          <button class="btn btn-primary" type="button" @click="showCreate = true">
            <Icon name="plus" size="sm" />
            {{ t('tickets.create') }}
          </button>
        </div>

        <div class="overflow-hidden rounded-md border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
          <div v-if="loadingList" class="flex justify-center py-16"><Icon name="refresh" class="animate-spin text-primary-600" /></div>
          <button
            v-for="item in tickets"
            v-else
            :key="item.id"
            type="button"
            class="grid w-full gap-3 border-b border-gray-100 px-5 py-4 text-left last:border-b-0 hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-700/50 sm:grid-cols-[1fr_auto]"
            @click="router.push(`/tickets/${item.id}`)"
          >
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <span class="text-xs text-gray-400">#{{ item.id }}</span>
                <h2 class="truncate font-medium text-gray-900 dark:text-white">{{ item.subject }}</h2>
              </div>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t(`tickets.category.${item.category}`) }}</p>
            </div>
            <div class="flex items-center gap-3 sm:justify-end">
              <TicketBadge :value="item.status" />
              <time class="whitespace-nowrap text-xs text-gray-500">{{ formatDate(item.last_message_at) }}</time>
            </div>
          </button>
          <div v-if="!loadingList && !tickets.length" class="py-16 text-center text-sm text-gray-500">{{ t('tickets.empty') }}</div>
        </div>

        <Pagination v-if="total > pageSize" :total="total" :page="page" :page-size="pageSize" :show-page-size-selector="false" @update:page="changePage" />
      </template>
    </div>

    <BaseDialog :show="showCreate" :title="t('tickets.create')" width="wide" @close="showCreate = false">
      <div class="space-y-4">
        <div>
          <label class="input-label">{{ t('tickets.subject') }}</label>
          <input v-model="createSubject" class="input" maxlength="200" />
        </div>
        <div>
          <label class="input-label">{{ t('tickets.categoryLabel') }}</label>
          <select v-model="createCategory" class="input">
            <option v-for="category in categories" :key="category" :value="category">{{ t(`tickets.category.${category}`) }}</option>
          </select>
        </div>
        <TicketComposer
          :key="createComposerKey"
          :submitting="submitting"
          :placeholder="t('tickets.messagePlaceholder')"
          :submit-label="t('tickets.submitTicket')"
          @submit="createNewTicket"
        />
      </div>
    </BaseDialog>

    <ConfirmDialog
      :show="showCloseConfirm"
      :title="t('tickets.closeTicket')"
      :message="t('tickets.closeConfirm')"
      danger
      @confirm="closeCurrentTicket"
      @cancel="showCloseConfirm = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import TicketBadge from '@/components/tickets/TicketBadge.vue'
import TicketComposer from '@/components/tickets/TicketComposer.vue'
import TicketConversation from '@/components/tickets/TicketConversation.vue'
import { ticketsAPI } from '@/api'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { Ticket, TicketCategory, TicketStatus } from '@/types/ticket'

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const ticketId = computed(() => Number(route.params.id || 0))
const tickets = ref<Ticket[]>([])
const ticket = ref<Ticket | null>(null)
const loadingList = ref(false)
const loadingDetail = ref(false)
const submitting = ref(false)
const page = ref(1)
const pageSize = 20
const total = ref(0)
const statusFilter = ref<TicketStatus | ''>('')
const showCreate = ref(false)
const showCloseConfirm = ref(false)
const createSubject = ref('')
const createCategory = ref<TicketCategory>('technical')
const composerKey = ref(0)
const createComposerKey = ref(0)
const categories: TicketCategory[] = ['technical', 'billing', 'account', 'suggestion', 'other']

function formatDate(value: string) {
  return new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}

async function loadList() {
  loadingList.value = true
  try {
    const result = await ticketsAPI.list(page.value, pageSize, statusFilter.value)
    tickets.value = result.items
    total.value = result.total
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('tickets.loadFailed')))
  } finally {
    loadingList.value = false
  }
}

async function loadDetail() {
  if (!ticketId.value) return
  loadingDetail.value = true
  try {
    ticket.value = await ticketsAPI.get(ticketId.value)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('tickets.loadFailed')))
  } finally {
    loadingDetail.value = false
  }
}

async function createNewTicket(payload: { content: string; images: File[] }) {
  if (!createSubject.value.trim()) return
  submitting.value = true
  try {
    const created = await ticketsAPI.create({ subject: createSubject.value.trim(), category: createCategory.value, ...payload })
    showCreate.value = false
    createSubject.value = ''
    createComposerKey.value++
    await router.push(`/tickets/${created.id}`)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('tickets.submitFailed')))
  } finally {
    submitting.value = false
  }
}

async function sendReply(payload: { content: string; images: File[] }) {
  submitting.value = true
  try {
    ticket.value = await ticketsAPI.reply(ticketId.value, payload)
    composerKey.value++
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('tickets.replyFailed')))
  } finally {
    submitting.value = false
  }
}

async function closeCurrentTicket() {
  showCloseConfirm.value = false
  try {
    ticket.value = await ticketsAPI.close(ticketId.value)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('tickets.closeFailed')))
  }
}

function changePage(value: number) {
  page.value = value
  loadList()
}

watch(ticketId, (id) => id ? loadDetail() : loadList())
onMounted(() => ticketId.value ? loadDetail() : loadList())
</script>
