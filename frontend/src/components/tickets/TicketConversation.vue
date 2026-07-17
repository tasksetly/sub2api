<template>
  <div class="divide-y divide-gray-100 dark:divide-dark-700">
    <article v-for="message in messages" :key="message.id" class="grid grid-cols-[2.5rem_minmax(0,1fr)] gap-3 py-5 first:pt-1 last:pb-1">
      <div
        class="flex h-10 w-10 items-center justify-center rounded-md"
        :class="message.author_role === 'admin'
          ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/35 dark:text-emerald-300'
          : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'"
      >
        <Icon :name="message.author_role === 'admin' ? 'chat' : 'user'" size="md" />
      </div>

      <div class="min-w-0">
        <header class="flex flex-wrap items-center gap-x-2 gap-y-1">
          <span class="text-sm font-semibold text-gray-900 dark:text-white">
            {{ message.author_role === 'admin' ? t('tickets.support') : (message.author_name || t('tickets.user')) }}
          </span>
          <span
            class="rounded px-1.5 py-0.5 text-[11px] font-medium"
            :class="message.author_role === 'admin'
              ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/25 dark:text-emerald-300'
              : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400'"
          >
            {{ message.author_role === 'admin' ? t('tickets.supportRole') : t('tickets.userRole') }}
          </span>
          <time class="ml-auto text-xs text-gray-400">{{ formatDate(message.created_at) }}</time>
        </header>

        <div
          class="mt-2 rounded-md border px-4 py-3.5 text-sm leading-6"
          :class="message.author_role === 'admin'
            ? 'border-emerald-200 bg-emerald-50/60 text-gray-800 dark:border-emerald-900/60 dark:bg-emerald-950/20 dark:text-gray-100'
            : 'border-gray-200 bg-gray-50 text-gray-800 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-100'"
        >
          <p v-if="message.content" class="whitespace-pre-wrap break-words">{{ message.content }}</p>
          <div v-if="message.attachments.length" class="mt-3 grid max-w-3xl grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
            <TicketAttachmentImage v-for="attachment in message.attachments" :key="attachment.url" :attachment="attachment" />
          </div>
        </div>
      </div>
    </article>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import TicketAttachmentImage from './TicketAttachmentImage.vue'
import type { TicketMessage } from '@/types/ticket'

defineProps<{ messages: TicketMessage[] }>()
const { t, locale } = useI18n()

function formatDate(value: string) {
  return new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}
</script>
