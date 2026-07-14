<template>
  <div class="space-y-6">
    <header class="page-header mb-0 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <h1 class="page-title">
          {{ isAdmin ? t('tickets.adminTitle') : t('tickets.title') }}
        </h1>
        <p class="page-description">
          {{ isAdmin ? t('tickets.adminDescription') : t('tickets.description') }}
        </p>
      </div>
      <button
        v-if="!isAdmin"
        type="button"
        class="btn btn-primary"
        @click="showCreateDialog = true"
      >
        <Icon name="plus" size="md" />
        {{ t('tickets.new') }}
      </button>
    </header>

    <section class="card grid gap-3 p-4 sm:grid-cols-2 xl:grid-cols-4">
      <label class="relative sm:col-span-2 xl:col-span-1">
        <span class="sr-only">{{ t('tickets.search') }}</span>
        <input
          v-model.trim="filters.search"
          class="input pl-10"
          :placeholder="t('tickets.search')"
          @keyup.enter="loadTickets(true)"
        >
        <Icon name="search" size="sm" class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-gray-400" />
      </label>
      <select v-model="filters.status" class="input" @change="loadTickets(true)">
        <option value="">{{ t('tickets.allStatuses') }}</option>
        <option v-for="status in statuses" :key="status" :value="status">{{ statusLabel(status) }}</option>
      </select>
      <select v-model="filters.category" class="input" @change="loadTickets(true)">
        <option value="">{{ t('tickets.allCategories') }}</option>
        <option v-for="category in categories" :key="category" :value="category">{{ t(`tickets.category.${category}`) }}</option>
      </select>
      <select v-model="filters.priority" class="input" @change="loadTickets(true)">
        <option value="">{{ t('tickets.allPriorities') }}</option>
        <option v-for="priority in priorities" :key="priority" :value="priority">{{ t(`tickets.priority.${priority}`) }}</option>
      </select>
    </section>

    <div class="card ticket-shell grid min-h-[620px] overflow-hidden lg:grid-cols-[360px_minmax(0,1fr)]">
      <aside
        class="min-w-0 border-gray-200 dark:border-dark-700 lg:border-r"
        :class="selectedTicket ? 'hidden lg:block' : 'block'"
      >
        <div class="flex h-12 items-center justify-between border-b border-gray-200 px-4 dark:border-dark-700">
          <span class="text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('tickets.listTitle') }}</span>
          <span class="text-xs tabular-nums text-gray-400">{{ total }}</span>
        </div>

        <div v-if="loadingList" class="flex h-72 items-center justify-center">
          <LoadingSpinner />
        </div>
        <div v-else-if="tickets.length === 0" class="flex h-72 flex-col items-center justify-center px-6 text-center">
          <div class="mb-3 flex h-11 w-11 items-center justify-center border border-dashed border-gray-300 text-xl text-gray-400 dark:border-dark-600">?</div>
          <p class="text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('tickets.empty') }}</p>
          <button v-if="!isAdmin" type="button" class="mt-3 text-sm font-medium text-primary-600 hover:text-primary-700" @click="showCreateDialog = true">
            {{ t('tickets.createFirst') }}
          </button>
        </div>
        <div v-else class="max-h-[620px] overflow-y-auto">
          <button
            v-for="ticket in tickets"
            :key="ticket.id"
            type="button"
            class="ticket-row w-full border-b border-gray-100 px-4 py-4 text-left transition-colors dark:border-dark-800"
            :class="selectedTicket?.id === ticket.id ? 'bg-primary-50 dark:bg-primary-950/25' : 'hover:bg-gray-50 dark:hover:bg-dark-800/70'"
            @click="selectTicket(ticket.id)"
          >
            <div class="flex items-start justify-between gap-3">
              <span class="min-w-0 truncate text-sm font-semibold text-gray-900 dark:text-white">{{ ticket.subject }}</span>
              <span v-if="unreadForMode(ticket)" class="mt-1.5 h-2 w-2 flex-none rounded-full bg-primary-500" :title="t('tickets.unread')" />
            </div>
            <div v-if="isAdmin" class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400">
              {{ ticket.username || ticket.user_email || `#${ticket.user_id}` }}
            </div>
            <div class="mt-3 flex items-center justify-between gap-3">
              <div class="flex min-w-0 items-center gap-2">
                <span :class="statusClass(ticket.status)" class="badge">{{ statusLabel(ticket.status) }}</span>
                <span :class="priorityClass(ticket.priority)" class="text-xs font-medium">{{ t(`tickets.priority.${ticket.priority}`) }}</span>
              </div>
              <time class="flex-none text-[11px] tabular-nums text-gray-400">{{ formatCompactDate(ticket.last_message_at) }}</time>
            </div>
          </button>
        </div>

        <div v-if="totalPages > 1" class="flex h-12 items-center justify-between border-t border-gray-200 px-3 dark:border-dark-700">
          <button type="button" class="btn btn-ghost h-8 w-8 p-0" :disabled="page <= 1" :title="t('common.back')" @click="changePage(page - 1)"><Icon name="chevronLeft" size="sm" /></button>
          <span class="text-xs tabular-nums text-gray-500">{{ page }} / {{ totalPages }}</span>
          <button type="button" class="btn btn-ghost h-8 w-8 p-0" :disabled="page >= totalPages" :title="t('common.next')" @click="changePage(page + 1)"><Icon name="chevronRight" size="sm" /></button>
        </div>
      </aside>

      <main class="min-w-0" :class="selectedTicket ? 'block' : 'hidden lg:block'">
        <div v-if="loadingDetail" class="flex h-full min-h-[620px] items-center justify-center">
          <LoadingSpinner />
        </div>
        <div v-else-if="selectedTicket" class="flex h-full min-h-[620px] flex-col">
          <div class="border-b border-gray-200 px-4 py-4 dark:border-dark-700 sm:px-6">
            <div class="flex items-start gap-3">
              <button type="button" class="btn btn-ghost -ml-2 h-9 w-9 p-0 lg:hidden" :title="t('common.back')" @click="selectedTicket = null"><Icon name="arrowLeft" size="md" /></button>
              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center gap-2">
                  <h2 class="min-w-0 truncate text-lg font-semibold text-gray-950 dark:text-white">{{ selectedTicket.subject }}</h2>
                  <span :class="statusClass(selectedTicket.status)" class="badge">{{ statusLabel(selectedTicket.status) }}</span>
                </div>
                <div class="mt-1 flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
                  <span>#{{ selectedTicket.id }}</span>
                  <span>{{ t(`tickets.category.${selectedTicket.category}`) }}</span>
                  <span>{{ t(`tickets.priority.${selectedTicket.priority}`) }}</span>
                  <span v-if="isAdmin">{{ selectedTicket.user_email }}</span>
                </div>
              </div>
              <div v-if="isAdmin" class="flex flex-col gap-2 sm:flex-row">
                <select v-model="adminStatus" class="input min-w-36 py-2 text-xs" :disabled="updating" @change="updateAdminTicket({ status: adminStatus })">
                  <option v-for="status in availableAdminStatuses" :key="status" :value="status">{{ statusLabel(status) }}</option>
                </select>
                <select v-model="adminPriority" class="input min-w-28 py-2 text-xs" :disabled="updating" @change="updateAdminTicket({ priority: adminPriority })">
                  <option v-for="priority in priorities" :key="priority" :value="priority">{{ t(`tickets.priority.${priority}`) }}</option>
                </select>
              </div>
            </div>
          </div>

          <div ref="messageScroller" class="flex-1 space-y-5 overflow-y-auto bg-gray-50/50 px-4 py-6 dark:bg-dark-950/35 sm:px-6">
            <article
              v-for="message in selectedTicket.messages || []"
              :key="message.id"
              class="flex"
              :class="message.sender_role === (isAdmin ? 'admin' : 'user') ? 'justify-end' : 'justify-start'"
            >
              <div class="max-w-[86%] sm:max-w-[72%]">
                <div class="mb-1 flex items-center gap-2 text-[11px] text-gray-400" :class="message.sender_role === (isAdmin ? 'admin' : 'user') ? 'justify-end' : ''">
                  <span>{{ senderLabel(message.sender_role) }}</span>
                  <time>{{ formatDateTime(message.created_at) }}</time>
                </div>
                <div
                  class="overflow-hidden rounded-xl border"
                  :class="message.sender_role === (isAdmin ? 'admin' : 'user')
                    ? 'border-primary-600 bg-primary-600 text-white'
                    : 'border-gray-200 bg-white text-gray-800 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-100'"
                >
                  <div
                    v-if="message.attachments?.length"
                    class="grid gap-1.5 p-1.5"
                    :class="message.attachments.length === 1 ? 'grid-cols-1' : 'grid-cols-2'"
                  >
                    <template v-for="attachment in message.attachments" :key="attachment.id">
                      <div class="group relative min-w-0 overflow-hidden rounded-md bg-gray-100 dark:bg-dark-950">
                        <img
                          v-if="attachmentPreviewURLs[attachment.id]"
                          :src="attachmentPreviewURLs[attachment.id]"
                          :alt="attachment.file_name"
                          class="aspect-[4/3] h-full max-h-72 w-full object-cover transition-opacity hover:opacity-90"
                          loading="lazy"
                        >
                        <div v-else class="flex aspect-[4/3] items-center justify-center px-3 text-center text-xs text-gray-500 dark:text-gray-400">
                          {{ t('tickets.attachmentUnavailable') }}
                        </div>
                        <button
                          type="button"
                          class="absolute bottom-2 right-2 flex h-8 w-8 items-center justify-center rounded bg-black/65 text-white opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100"
                          :title="t('tickets.downloadAttachment')"
                          @click="downloadTicketAttachment(attachment)"
                        >
                          <Icon name="download" size="sm" />
                        </button>
                      </div>
                    </template>
                  </div>
                  <p v-if="message.content" class="whitespace-pre-wrap break-words px-4 py-3 text-sm leading-6">{{ message.content }}</p>
                </div>
              </div>
            </article>
          </div>

          <div class="border-t border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900 sm:px-6">
            <div v-if="selectedTicket.status !== 'closed'">
              <div v-if="replyFiles.length" class="mb-3 grid grid-cols-3 gap-2 sm:grid-cols-4 md:grid-cols-6">
                <div v-for="(item, index) in replyFiles" :key="item.previewUrl" class="group relative aspect-square overflow-hidden rounded-md border border-gray-200 bg-gray-100 dark:border-dark-700 dark:bg-dark-950">
                  <img :src="item.previewUrl" :alt="item.file.name" class="h-full w-full object-cover">
                  <button type="button" class="absolute right-1 top-1 flex h-7 w-7 items-center justify-center rounded bg-black/65 text-white opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100" :title="t('tickets.removeAttachment')" @click="removePendingFile(replyFiles, index)">
                    <Icon name="x" size="sm" />
                  </button>
                </div>
              </div>
              <div class="flex items-end gap-2 sm:gap-3">
                <input ref="replyFileInput" type="file" class="sr-only" accept="image/jpeg,image/png,image/gif,image/webp" multiple @change="onReplyFilesSelected">
                <button v-if="attachmentPolicy.enabled" type="button" class="btn btn-secondary h-10 w-10 flex-none p-0" :title="attachmentButtonTitle" :disabled="sending || replyFiles.length >= attachmentPolicy.max_attachments_per_message" @click="replyFileInput?.click()">
                  <Icon name="paperclip" size="md" />
                </button>
                <textarea
                  v-model="replyContent"
                  rows="3"
                  maxlength="10000"
                  class="input min-h-[84px] flex-1 resize-y"
                  :placeholder="t('tickets.replyPlaceholder')"
                  @keydown.ctrl.enter.prevent="sendReply"
                  @keydown.meta.enter.prevent="sendReply"
                />
                <button type="button" class="btn btn-primary h-10 px-4 sm:px-5" :disabled="sending || (!replyContent.trim() && replyFiles.length === 0)" @click="sendReply">
                  {{ sending ? t('common.submitting') : t('tickets.send') }}
                </button>
              </div>
            </div>
            <div v-else class="flex items-center justify-between gap-3 text-sm text-gray-500 dark:text-gray-400">
              <span>{{ t('tickets.closedNotice') }}</span>
              <button v-if="!isAdmin" type="button" class="btn btn-secondary" :disabled="updating" @click="reopenSelected">
                {{ t('tickets.reopen') }}
              </button>
            </div>
            <div v-if="!isAdmin && selectedTicket.status !== 'closed'" class="mt-3 flex justify-end">
              <button type="button" class="text-xs font-medium text-gray-500 hover:text-red-600" :disabled="updating" @click="closeSelected">
                {{ t('tickets.closeTicket') }}
              </button>
            </div>
          </div>
        </div>
        <div v-else class="flex h-full min-h-[620px] flex-col items-center justify-center px-8 text-center text-gray-400">
          <div class="mb-4 h-px w-16 bg-gray-300 dark:bg-dark-600" />
          <p class="text-sm">{{ t('tickets.selectPrompt') }}</p>
        </div>
      </main>
    </div>

    <BaseDialog :show="showCreateDialog" :title="t('tickets.new')" width="wide" @close="closeCreateDialog">
      <form id="support-ticket-form" class="space-y-5" @submit.prevent="createNewTicket">
        <label class="block">
          <span class="input-label">{{ t('tickets.subject') }}</span>
          <input v-model.trim="createForm.subject" maxlength="200" class="input" required>
        </label>
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="block">
            <span class="input-label">{{ t('tickets.categoryLabel') }}</span>
            <select v-model="createForm.category" class="input">
              <option v-for="category in categories" :key="category" :value="category">{{ t(`tickets.category.${category}`) }}</option>
            </select>
          </label>
          <label class="block">
            <span class="input-label">{{ t('tickets.priorityLabel') }}</span>
            <select v-model="createForm.priority" class="input">
              <option v-for="priority in priorities" :key="priority" :value="priority">{{ t(`tickets.priority.${priority}`) }}</option>
            </select>
          </label>
        </div>
        <label class="block">
          <span class="input-label">{{ t('tickets.descriptionLabel') }}</span>
          <textarea v-model.trim="createForm.content" maxlength="10000" rows="7" class="input resize-y" :required="createFiles.length === 0" />
        </label>
        <div v-if="attachmentPolicy.enabled" class="space-y-3">
          <input ref="createFileInput" type="file" class="sr-only" accept="image/jpeg,image/png,image/gif,image/webp" multiple @change="onCreateFilesSelected">
          <button type="button" class="btn btn-secondary" :title="attachmentButtonTitle" :disabled="createFiles.length >= attachmentPolicy.max_attachments_per_message" @click="createFileInput?.click()">
            <Icon name="paperclip" size="sm" />
            {{ t('tickets.addImages') }}
          </button>
          <div v-if="createFiles.length" class="grid grid-cols-3 gap-2 sm:grid-cols-5">
            <div v-for="(item, index) in createFiles" :key="item.previewUrl" class="group relative aspect-square overflow-hidden rounded-md border border-gray-200 bg-gray-100 dark:border-dark-700 dark:bg-dark-950">
              <img :src="item.previewUrl" :alt="item.file.name" class="h-full w-full object-cover">
              <button type="button" class="absolute right-1 top-1 flex h-7 w-7 items-center justify-center rounded bg-black/65 text-white opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100" :title="t('tickets.removeAttachment')" @click="removePendingFile(createFiles, index)">
                <Icon name="x" size="sm" />
              </button>
            </div>
          </div>
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closeCreateDialog">{{ t('common.cancel') }}</button>
          <button type="submit" form="support-ticket-form" class="btn btn-primary" :disabled="creating || !createForm.subject || (!createForm.content && createFiles.length === 0)">
            {{ creating ? t('common.submitting') : t('tickets.create') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { ticketsAPI } from '@/api/tickets'
import adminTicketsAPI from '@/api/admin/tickets'
import { useAppStore } from '@/stores/app'
import type {
  CreateSupportTicketRequest,
  SupportTicket,
  SupportTicketAttachment,
  SupportTicketAttachmentPolicy,
  SupportTicketCategory,
  SupportTicketFilters,
  SupportTicketPriority,
  SupportTicketSenderRole,
  SupportTicketStatus
} from '@/types/supportTicket'

const props = defineProps<{ mode: 'user' | 'admin' }>()
const { t, locale } = useI18n()
const appStore = useAppStore()
const isAdmin = computed(() => props.mode === 'admin')

const statuses: SupportTicketStatus[] = ['open', 'in_progress', 'waiting_user', 'resolved', 'closed']
const priorities: SupportTicketPriority[] = ['low', 'normal', 'high', 'urgent']
const categories: SupportTicketCategory[] = ['technical', 'billing', 'account', 'other']

const tickets = ref<SupportTicket[]>([])
const selectedTicket = ref<SupportTicket | null>(null)
const loadingList = ref(false)
const loadingDetail = ref(false)
const sending = ref(false)
const updating = ref(false)
const creating = ref(false)
const showCreateDialog = ref(false)
const replyContent = ref('')
type PendingAttachment = { file: File; previewUrl: string }
const replyFiles = reactive<PendingAttachment[]>([])
const createFiles = reactive<PendingAttachment[]>([])
const attachmentPreviewURLs = reactive<Record<number, string>>({})
const replyFileInput = ref<HTMLInputElement | null>(null)
const createFileInput = ref<HTMLInputElement | null>(null)
const attachmentPolicy = reactive<SupportTicketAttachmentPolicy>({ enabled: false, max_file_size_bytes: 10 * 1024 * 1024, max_attachments_per_message: 4 })
const attachmentButtonTitle = computed(() => t('tickets.attachmentHint', {
  size: Math.max(1, Math.round(attachmentPolicy.max_file_size_bytes / 1024 / 1024)),
  count: attachmentPolicy.max_attachments_per_message
}))
const messageScroller = ref<HTMLElement | null>(null)
const page = ref(1)
const pageSize = 20
const total = ref(0)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))
const filters = reactive<SupportTicketFilters>({ status: '', category: '', priority: '', search: '' })
const adminStatus = ref<SupportTicketStatus>('open')
const adminPriority = ref<SupportTicketPriority>('normal')
const availableAdminStatuses = computed<SupportTicketStatus[]>(() => {
  if (!selectedTicket.value) return statuses
  const transitions: Record<SupportTicketStatus, SupportTicketStatus[]> = {
    open: ['open', 'in_progress', 'waiting_user', 'resolved', 'closed'],
    in_progress: ['in_progress', 'waiting_user', 'resolved', 'closed'],
    waiting_user: ['waiting_user', 'in_progress', 'resolved', 'closed'],
    resolved: ['resolved', 'in_progress', 'closed'],
    closed: ['closed', 'in_progress']
  }
  return transitions[selectedTicket.value.status]
})
const createForm = reactive<CreateSupportTicketRequest>({ subject: '', category: 'technical', priority: 'normal', content: '' })

function errorMessage(error: unknown, fallback: string): string {
  return (error as { message?: string })?.message || fallback
}

async function loadTickets(resetPage = false) {
  if (resetPage) page.value = 1
  loadingList.value = true
  try {
    const result = isAdmin.value
      ? await adminTicketsAPI.list(page.value, pageSize, filters)
      : await ticketsAPI.list(page.value, pageSize, filters)
    tickets.value = result.items || []
    total.value = result.total || 0
    if (selectedTicket.value) {
      const refreshed = tickets.value.find((item) => item.id === selectedTicket.value?.id)
      if (refreshed) Object.assign(selectedTicket.value, refreshed)
    }
  } catch (error) {
    appStore.showError(errorMessage(error, t('tickets.loadFailed')))
  } finally {
    loadingList.value = false
  }
}

async function selectTicket(id: number) {
  clearPendingFiles(replyFiles)
  loadingDetail.value = true
  try {
    clearAttachmentPreviews()
    const ticket = isAdmin.value ? await adminTicketsAPI.get(id) : await ticketsAPI.get(id)
    selectedTicket.value = ticket
    loadAttachmentPreviews(ticket)
    adminStatus.value = selectedTicket.value.status
    adminPriority.value = selectedTicket.value.priority
    const row = tickets.value.find((item) => item.id === id)
    if (row) {
      if (isAdmin.value) row.admin_unread = false
      else row.user_unread = false
    }
    await scrollToLatestMessage()
  } catch (error) {
    appStore.showError(errorMessage(error, t('tickets.loadFailed')))
  } finally {
    loadingDetail.value = false
  }
}

async function sendReply() {
  if (!selectedTicket.value || (!replyContent.value.trim() && replyFiles.length === 0) || sending.value) return
  sending.value = true
  try {
    const ticket = isAdmin.value
      ? await adminTicketsAPI.reply(selectedTicket.value.id, replyContent.value.trim(), replyFiles.map((item) => item.file))
      : await ticketsAPI.reply(selectedTicket.value.id, replyContent.value.trim(), replyFiles.map((item) => item.file))
    selectedTicket.value = ticket
    clearAttachmentPreviews()
    loadAttachmentPreviews(ticket)
    replyContent.value = ''
    clearPendingFiles(replyFiles)
    adminStatus.value = selectedTicket.value.status
    await loadTickets()
    await scrollToLatestMessage()
  } catch (error) {
    appStore.showError(errorMessage(error, t('tickets.replyFailed')))
  } finally {
    sending.value = false
  }
}

async function updateAdminTicket(update: { status?: SupportTicketStatus; priority?: SupportTicketPriority }) {
  if (!selectedTicket.value || updating.value) return
  updating.value = true
  try {
    selectedTicket.value = await adminTicketsAPI.update(selectedTicket.value.id, update)
    adminStatus.value = selectedTicket.value.status
    adminPriority.value = selectedTicket.value.priority
    await loadTickets()
  } catch (error) {
    adminStatus.value = selectedTicket.value.status
    adminPriority.value = selectedTicket.value.priority
    appStore.showError(errorMessage(error, t('tickets.updateFailed')))
  } finally {
    updating.value = false
  }
}

async function closeSelected() {
  if (!selectedTicket.value || updating.value || !window.confirm(t('tickets.closeConfirm'))) return
  updating.value = true
  try {
    selectedTicket.value = await ticketsAPI.close(selectedTicket.value.id)
    await loadTickets()
  } catch (error) {
    appStore.showError(errorMessage(error, t('tickets.updateFailed')))
  } finally {
    updating.value = false
  }
}

async function reopenSelected() {
  if (!selectedTicket.value || updating.value) return
  updating.value = true
  try {
    selectedTicket.value = await ticketsAPI.reopen(selectedTicket.value.id)
    await loadTickets()
  } catch (error) {
    appStore.showError(errorMessage(error, t('tickets.updateFailed')))
  } finally {
    updating.value = false
  }
}

async function createNewTicket() {
  if (creating.value) return
  creating.value = true
  try {
    const created = await ticketsAPI.create({ ...createForm }, createFiles.map((item) => item.file))
    showCreateDialog.value = false
    clearPendingFiles(createFiles)
    Object.assign(createForm, { subject: '', category: 'technical', priority: 'normal', content: '' })
    await loadTickets(true)
    await selectTicket(created.id)
  } catch (error) {
    appStore.showError(errorMessage(error, t('tickets.createFailed')))
  } finally {
    creating.value = false
  }
}

async function loadAttachmentPolicy() {
  try {
    const policy = isAdmin.value ? await adminTicketsAPI.attachmentPolicy() : await ticketsAPI.attachmentPolicy()
    Object.assign(attachmentPolicy, policy)
  } catch {
    attachmentPolicy.enabled = false
  }
}

function onReplyFilesSelected(event: Event) {
  addPendingFiles((event.target as HTMLInputElement).files, replyFiles)
  ;(event.target as HTMLInputElement).value = ''
}

function onCreateFilesSelected(event: Event) {
  addPendingFiles((event.target as HTMLInputElement).files, createFiles)
  ;(event.target as HTMLInputElement).value = ''
}

function addPendingFiles(files: FileList | null, target: PendingAttachment[]) {
  if (!files) return
  const allowedTypes = new Set(['image/jpeg', 'image/png', 'image/gif', 'image/webp'])
  for (const file of Array.from(files)) {
    if (target.length >= attachmentPolicy.max_attachments_per_message) {
      appStore.showError(t('tickets.attachmentTooMany', { count: attachmentPolicy.max_attachments_per_message }))
      break
    }
    if (!allowedTypes.has(file.type)) {
      appStore.showError(t('tickets.attachmentTypeInvalid'))
      continue
    }
    if (file.size <= 0 || file.size > attachmentPolicy.max_file_size_bytes) {
      appStore.showError(t('tickets.attachmentTooLarge', { size: Math.max(1, Math.round(attachmentPolicy.max_file_size_bytes / 1024 / 1024)) }))
      continue
    }
    const duplicate = target.some((item) => item.file.name === file.name && item.file.size === file.size && item.file.lastModified === file.lastModified)
    if (!duplicate) target.push({ file, previewUrl: URL.createObjectURL(file) })
  }
}

function removePendingFile(target: PendingAttachment[], index: number) {
  const [removed] = target.splice(index, 1)
  if (removed) URL.revokeObjectURL(removed.previewUrl)
}

function clearPendingFiles(target: PendingAttachment[]) {
  target.forEach((item) => URL.revokeObjectURL(item.previewUrl))
  target.splice(0)
}

async function loadAttachmentPreview(ticketID: number, attachment: SupportTicketAttachment) {
  if (attachmentPreviewURLs[attachment.id]) return
  try {
    const blob = isAdmin.value
      ? await adminTicketsAPI.downloadAttachment(ticketID, attachment.id)
      : await ticketsAPI.downloadAttachment(ticketID, attachment.id)
    attachmentPreviewURLs[attachment.id] = URL.createObjectURL(blob)
  } catch {
    // The message still exposes a download action; avoid failing the whole ticket view for one object.
  }
}

function loadAttachmentPreviews(ticket: SupportTicket) {
  for (const message of ticket.messages || []) {
    for (const attachment of message.attachments || []) {
      void loadAttachmentPreview(ticket.id, attachment)
    }
  }
}

async function downloadTicketAttachment(attachment: SupportTicketAttachment) {
  if (!selectedTicket.value) return
  try {
    const blob = isAdmin.value
      ? await adminTicketsAPI.downloadAttachment(selectedTicket.value.id, attachment.id)
      : await ticketsAPI.downloadAttachment(selectedTicket.value.id, attachment.id)
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = attachment.file_name
    link.click()
    URL.revokeObjectURL(url)
  } catch (error) {
    appStore.showError(errorMessage(error, t('tickets.attachmentDownloadFailed')))
  }
}

function clearAttachmentPreviews() {
  Object.values(attachmentPreviewURLs).forEach((url) => URL.revokeObjectURL(url))
  Object.keys(attachmentPreviewURLs).forEach((key) => delete attachmentPreviewURLs[Number(key)])
}

function closeCreateDialog() {
  showCreateDialog.value = false
  clearPendingFiles(createFiles)
}

async function changePage(nextPage: number) {
  page.value = nextPage
  selectedTicket.value = null
  await loadTickets()
}

async function scrollToLatestMessage() {
  await nextTick()
  if (messageScroller.value) messageScroller.value.scrollTop = messageScroller.value.scrollHeight
}

function unreadForMode(ticket: SupportTicket) {
  return isAdmin.value ? ticket.admin_unread : ticket.user_unread
}

function statusLabel(status: SupportTicketStatus) {
  if (status === 'waiting_user' && !isAdmin.value) return t('tickets.status.waiting_you')
  return t(`tickets.status.${status}`)
}

function senderLabel(role: SupportTicketSenderRole) {
  if (isAdmin.value) return role === 'admin' ? t('tickets.sender.adminYou') : t('tickets.sender.user')
  return role === 'user' ? t('tickets.sender.you') : t('tickets.sender.support')
}

function statusClass(status: SupportTicketStatus) {
  return {
    open: 'badge-warning',
    in_progress: 'badge-primary',
    waiting_user: 'badge-purple',
    resolved: 'badge-success',
    closed: 'badge-gray'
  }[status]
}

function priorityClass(priority: SupportTicketPriority) {
  return priority === 'urgent' ? 'text-red-600 dark:text-red-400' : priority === 'high' ? 'text-orange-600 dark:text-orange-400' : 'text-gray-500 dark:text-gray-400'
}

function formatDateTime(value: string) {
  return new Intl.DateTimeFormat(locale.value === 'zh' ? 'zh-CN' : 'en-US', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}

function formatCompactDate(value: string) {
  const date = new Date(value)
  const today = new Date()
  const sameDay = date.toDateString() === today.toDateString()
  return new Intl.DateTimeFormat(locale.value === 'zh' ? 'zh-CN' : 'en-US', sameDay ? { hour: '2-digit', minute: '2-digit' } : { month: 'short', day: 'numeric' }).format(date)
}

onMounted(() => Promise.all([loadTickets(), loadAttachmentPolicy()]))
onBeforeUnmount(() => {
  clearPendingFiles(replyFiles)
  clearPendingFiles(createFiles)
  clearAttachmentPreviews()
})
</script>

<style scoped>
.ticket-row { min-height: 108px; }
</style>
