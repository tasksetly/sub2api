-- 邀请返现的 USDT 结算快照和凭证工单；旧提现流水保留 NULL。
ALTER TABLE user_affiliate_ledger ADD COLUMN IF NOT EXISTS payout JSONB;

COMMENT ON COLUMN user_affiliate_ledger.payout IS 'USDT 返现快照：网络、收款地址、交易哈希、汇率、USDT 金额、备注和凭证工单 ID';

-- 同一网络交易只能登记一次，防止更换幂等键后重复扣减。
CREATE UNIQUE INDEX IF NOT EXISTS idx_affiliate_payout_network_tx
    ON user_affiliate_ledger ((payout->>'network'), (LOWER(payout->>'tx_hash')))
    WHERE action = 'withdraw' AND payout IS NOT NULL;
