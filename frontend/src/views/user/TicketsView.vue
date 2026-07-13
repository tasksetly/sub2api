<template>
  <AppLayout>
    <main class="page-container">
      <div class="mb-4 flex justify-between">
        <h1>我的工单</h1>
        <button class="btn btn-primary" @click="create">新建工单</button>
      </div>

      <div
        v-for="ticket in tickets"
        :key="ticket.id"
        class="cursor-pointer border-b p-3"
        @click="open(ticket.id)"
      >
        {{ ticket.subject }} <span>{{ ticket.status }}</span>
      </div>

      <section v-if="detail">
        <h2>{{ detail.subject }}</h2>
        <p v-for="item in detail.messages" :key="item.id">
          <b>{{ item.sender_role }}:</b> {{ item.content }}
        </p>
        <textarea v-if="detail.status !== 'closed'" v-model="message" class="input" />
        <button
          v-if="detail.status !== 'closed'"
          class="btn btn-primary"
          @click="reply"
        >
          发送
        </button>
      </section>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { ticketsAPI } from '@/api'
import type { Ticket, TicketDetail } from '@/types'

const tickets = ref<Ticket[]>([])
const detail = ref<TicketDetail>()
const message = ref('')

async function load() {
  tickets.value = (await ticketsAPI.list()).items
}

async function open(id: number) {
  detail.value = await ticketsAPI.getById(id)
}

async function reply() {
  const content = message.value.trim()
  if (!detail.value || !content) return

  detail.value = await ticketsAPI.addMessage(detail.value.id, content)
  message.value = ''
}

async function create() {
  const subject = window.prompt('主题')
  const content = window.prompt('内容')
  if (!subject || !content) return

  detail.value = await ticketsAPI.create({ subject, content, category: 'other' })
  await load()
}

onMounted(load)
</script>
