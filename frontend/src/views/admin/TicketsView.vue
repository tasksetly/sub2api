<template>
  <AppLayout>
    <main class="page-container">
      <h1>工单管理</h1>

      <div
        v-for="ticket in tickets"
        :key="ticket.id"
        class="cursor-pointer border-b p-3"
        @click="openTicket(ticket.id)"
      >
        {{ ticket.subject }} {{ ticket.status }} {{ ticket.priority }}
      </div>

      <section v-if="detail">
        <h2>{{ detail.subject }}</h2>
        <p v-for="ticketMessage in detail.messages" :key="ticketMessage.id">
          {{ ticketMessage.sender_role }}: {{ ticketMessage.content }}
        </p>

        <label>
          状态
          <select v-model="status" @change="updateStatus">
            <option value="pending" disabled>待处理</option>
            <option value="in_progress">处理中</option>
            <option value="resolved">已解决</option>
            <option value="closed">已关闭</option>
          </select>
        </label>

        <label>
          优先级
          <select v-model="priority" @change="updatePriority">
            <option value="low">低</option>
            <option value="normal">普通</option>
            <option value="high">高</option>
            <option value="urgent">紧急</option>
          </select>
        </label>

        <template v-if="detail.status !== 'closed'">
          <textarea v-model="message" class="input" />
          <button class="btn btn-primary" @click="reply">发送</button>
        </template>
      </section>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { adminAPI } from '@/api/admin'
import type {
  Ticket,
  TicketDetail,
  TicketPriority,
  TicketStatus,
} from '@/types'

const tickets = ref<Ticket[]>([])
const detail = ref<TicketDetail>()
const status = ref<TicketStatus>('in_progress')
const priority = ref<TicketPriority>('normal')
const message = ref('')

async function load() {
  tickets.value = (await adminAPI.tickets.list()).items
}

async function openTicket(id: number) {
  const ticket = await adminAPI.tickets.getById(id)
  detail.value = ticket
  status.value = ticket.status
  priority.value = ticket.priority
}

async function updateStatus() {
  if (!detail.value) return

  const ticketId = detail.value.id
  await adminAPI.tickets.update(ticketId, { status: status.value })
  await openTicket(ticketId)
  await load()
}

async function updatePriority() {
  if (!detail.value) return

  const ticketId = detail.value.id
  await adminAPI.tickets.update(ticketId, { priority: priority.value })
  await openTicket(ticketId)
  await load()
}

async function reply() {
  const content = message.value.trim()
  if (!detail.value || !content) return

  detail.value = await adminAPI.tickets.addMessage(detail.value.id, content)
  message.value = ''
  await load()
}

onMounted(load)
</script>
