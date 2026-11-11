<template>
  <BaseDialog :show="record !== null" :title="t('affiliate.payout.receipt')" width="wide" @close="$emit('close')">
    <div v-if="record?.payout" class="space-y-4">
      <dl class="grid grid-cols-2 gap-3 rounded-lg bg-gray-50 p-4 text-sm dark:bg-dark-800">
        <div><dt class="text-gray-500">{{ t('affiliate.payout.cashbackAmount') }}</dt><dd>{{ formatCurrency(record.amount) }}</dd></div>
        <div><dt class="text-gray-500">{{ t('affiliate.payout.paidAmount') }}</dt><dd>{{ record.payout.usdt_amount.toFixed(6) }} USDT</dd></div>
        <div><dt class="text-gray-500">{{ t('affiliate.payout.network') }}</dt><dd>{{ networkLabel }}</dd></div>
        <div><dt class="text-gray-500">{{ t('affiliate.payout.exchangeRate') }}</dt><dd>1 USDT = {{ record.payout.exchange_rate }} CNY</dd></div>
        <div class="col-span-2"><dt class="text-gray-500">{{ t('affiliate.payout.address') }}</dt><dd class="break-all font-mono">{{ record.payout.wallet_address }}</dd></div>
        <div class="col-span-2"><dt class="text-gray-500">{{ t('affiliate.payout.transaction') }}</dt><dd class="break-all font-mono"><a :href="transactionURL" target="_blank" rel="noopener noreferrer" class="text-primary-600 hover:underline">{{ record.payout.tx_hash }}</a></dd></div>
      </dl>
      <p v-if="loading" class="text-sm text-gray-500">{{ t('common.loading') }}</p>
      <p v-else-if="error" class="text-sm text-red-500">{{ error }}</p>
      <TicketConversation v-else-if="ticket" :messages="ticket.messages || []" />
      <RouterLink :to="`${admin ? '/admin' : ''}/tickets/${record.payout.ticket_id}`" class="btn btn-secondary">
        {{ t('affiliate.payout.openTicket') }}
      </RouterLink>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import TicketConversation from '@/components/tickets/TicketConversation.vue'
import ticketsAPI from '@/api/tickets'
import adminTicketsAPI from '@/api/admin/tickets'
import type { AffiliateTransferRecord } from '@/types/affiliate'
import type { Ticket } from '@/types/ticket'
import { formatCurrency } from '@/utils/format'
import { affiliateTransactionURL } from '@/utils/affiliate'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{ record: AffiliateTransferRecord | null; admin?: boolean }>()
defineEmits<{ close: [] }>()
const { t } = useI18n()
const ticket = ref<Ticket | null>(null)
const loading = ref(false)
const error = ref('')
const networkLabel = computed(() => props.record?.payout?.network === 'polygon' ? 'Polygon' : 'BSC')
const transactionURL = computed(() => props.record?.payout ? affiliateTransactionURL(props.record.payout) : '')
let requestID = 0
watch(() => props.record, async (record) => {
  const id = ++requestID
  ticket.value = null
  error.value = ''
  loading.value = false
  if (!record?.payout?.ticket_id) return
  loading.value = true
  try {
    const result = await (props.admin ? adminTicketsAPI : ticketsAPI).get(record.payout.ticket_id)
    if (id === requestID) ticket.value = result
  } catch (e) {
    if (id === requestID) error.value = extractApiErrorMessage(e, t('common.error'))
  } finally {
    if (id === requestID) loading.value = false
  }
}, { immediate: true })
</script>
