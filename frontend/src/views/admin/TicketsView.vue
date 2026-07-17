<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl space-y-5">
      <template v-if="ticketId">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <button class="btn btn-ghost" type="button" @click="router.push('/admin/tickets')">
            <Icon name="arrowLeft" size="sm" />
            {{ t('common.back') }}
          </button>
          <button class="btn btn-primary" type="button" :disabled="updating || !ticket" @click="updateTicket">
            <Icon v-if="updating" name="refresh" size="sm" class="animate-spin" />
            <Icon v-else name="check" size="sm" />
            {{ t('common.save') }}
          </button>
        </div>

        <div v-if="loadingDetail" class="flex justify-center py-16"><Icon name="refresh" class="animate-spin text-primary-600" /></div>
        <template v-else-if="ticket">
          <header class="grid gap-4 border-b border-gray-200 pb-5 dark:border-dark-700 lg:grid-cols-[1fr_auto]">
            <div class="min-w-0">
              <div class="mb-2 flex flex-wrap items-center gap-2">
                <span class="text-sm text-gray-500">#{{ ticket.id }}</span>
                <TicketBadge :value="ticket.status" />
                <TicketBadge :value="ticket.priority" type="priority" />
              </div>
              <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ ticket.subject }}</h1>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ ticket.user_email }} · {{ t(`tickets.category.${ticket.category}`) }} · {{ formatDate(ticket.created_at) }}
              </p>
            </div>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="input-label">{{ t('common.status') }}</label>
                <select v-model="detailStatus" class="input w-44">
                  <option v-for="status in statuses" :key="status" :value="status">{{ t(`tickets.status.${status}`) }}</option>
                </select>
              </div>
              <div>
                <label class="input-label">{{ t('tickets.priorityLabel') }}</label>
                <select v-model="detailPriority" class="input w-36">
                  <option v-for="priority in priorities" :key="priority" :value="priority">{{ t(`tickets.priority.${priority}`) }}</option>
                </select>
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
              :placeholder="t('tickets.adminReplyPlaceholder')"
              :submit-label="t('tickets.sendReply')"
              @submit="sendReply"
            />
          </section>
        </template>
      </template>

      <template v-else>
        <div class="grid gap-3 md:grid-cols-[minmax(240px,1fr)_180px_160px_auto]">
          <div class="relative">
            <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-3 text-gray-400" />
            <input v-model="search" class="input pl-9" :placeholder="t('tickets.searchPlaceholder')" @keyup.enter="applyFilters" />
          </div>
          <select v-model="statusFilter" class="input" @change="applyFilters">
            <option value="">{{ t('tickets.allStatuses') }}</option>
            <option v-for="status in statuses" :key="status" :value="status">{{ t(`tickets.status.${status}`) }}</option>
          </select>
          <select v-model="priorityFilter" class="input" @change="applyFilters">
            <option value="">{{ t('tickets.allPriorities') }}</option>
            <option v-for="priority in priorities" :key="priority" :value="priority">{{ t(`tickets.priority.${priority}`) }}</option>
          </select>
          <button class="btn btn-secondary" type="button" @click="applyFilters"><Icon name="filter" size="sm" />{{ t('common.filter') }}</button>
        </div>

        <div class="overflow-hidden rounded-md border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
          <div v-if="loadingList" class="flex justify-center py-16"><Icon name="refresh" class="animate-spin text-primary-600" /></div>
          <template v-else>
            <button
              v-for="item in tickets"
              :key="item.id"
              type="button"
              class="grid w-full gap-3 border-b border-gray-100 px-5 py-4 text-left last:border-b-0 hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-700/50 lg:grid-cols-[minmax(0,1fr)_220px_220px]"
              @click="router.push(`/admin/tickets/${item.id}`)"
            >
              <div class="min-w-0">
                <div class="flex items-center gap-2">
                  <span class="text-xs text-gray-400">#{{ item.id }}</span>
                  <h2 class="truncate font-medium text-gray-900 dark:text-white">{{ item.subject }}</h2>
                </div>
                <p class="mt-1 truncate text-sm text-gray-500 dark:text-gray-400">{{ item.user_email }} · {{ t(`tickets.category.${item.category}`) }}</p>
              </div>
              <div class="flex items-center gap-2 lg:justify-start">
                <TicketBadge :value="item.status" />
                <TicketBadge :value="item.priority" type="priority" />
              </div>
              <time class="self-center text-xs text-gray-500 lg:text-right">{{ formatDate(item.last_message_at) }}</time>
            </button>
            <div v-if="!tickets.length" class="py-16 text-center text-sm text-gray-500">{{ t('tickets.adminEmpty') }}</div>
          </template>
        </div>
        <Pagination v-if="total > pageSize" :total="total" :page="page" :page-size="pageSize" :show-page-size-selector="false" @update:page="changePage" />
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import TicketBadge from '@/components/tickets/TicketBadge.vue'
import TicketComposer from '@/components/tickets/TicketComposer.vue'
import TicketConversation from '@/components/tickets/TicketConversation.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { Ticket, TicketPriority, TicketStatus } from '@/types/ticket'

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
const updating = ref(false)
const page = ref(1)
const pageSize = 20
const total = ref(0)
const search = ref('')
const statusFilter = ref('')
const priorityFilter = ref('')
const detailStatus = ref<TicketStatus>('open')
const detailPriority = ref<TicketPriority>('normal')
const composerKey = ref(0)
const statuses: TicketStatus[] = ['open', 'in_progress', 'waiting_user', 'closed']
const priorities: TicketPriority[] = ['low', 'normal', 'high', 'urgent']

function formatDate(value: string) {
  return new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}

async function loadList() {
  loadingList.value = true
  try {
    const result = await adminAPI.tickets.list(page.value, pageSize, {
      search: search.value.trim() || undefined,
      status: statusFilter.value || undefined,
      priority: priorityFilter.value || undefined
    })
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
    ticket.value = await adminAPI.tickets.get(ticketId.value)
    detailStatus.value = ticket.value.status
    detailPriority.value = ticket.value.priority
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('tickets.loadFailed')))
  } finally {
    loadingDetail.value = false
  }
}

async function sendReply(payload: { content: string; images: File[] }) {
  submitting.value = true
  try {
    ticket.value = await adminAPI.tickets.reply(ticketId.value, payload)
    detailStatus.value = ticket.value.status
    composerKey.value++
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('tickets.replyFailed')))
  } finally {
    submitting.value = false
  }
}

async function updateTicket() {
  updating.value = true
  try {
    ticket.value = await adminAPI.tickets.update(ticketId.value, { status: detailStatus.value, priority: detailPriority.value })
    appStore.showSuccess(t('common.saved'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('tickets.updateFailed')))
  } finally {
    updating.value = false
  }
}

function applyFilters() {
  page.value = 1
  loadList()
}

function changePage(value: number) {
  page.value = value
  loadList()
}

watch(ticketId, (id) => id ? loadDetail() : loadList())
onMounted(() => ticketId.value ? loadDetail() : loadList())
</script>
