-- Capture recharge completion atomically; delivery never runs in a payment request.
CREATE TABLE IF NOT EXISTS notification_webhook_deliveries (
    id BIGSERIAL PRIMARY KEY,
    event_id VARCHAR(120) NOT NULL UNIQUE,
    event_type VARCHAR(80) NOT NULL,
    order_id BIGINT NOT NULL,
    config_version VARCHAR(64) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'delivering', 'delivered', 'failed', 'skipped')),
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    claimed_at TIMESTAMPTZ,
    claimed_by VARCHAR(64),
    last_status_code INTEGER NOT NULL DEFAULT 0,
    last_error VARCHAR(200) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_notification_webhook_pending
    ON notification_webhook_deliveries (available_at, id)
    WHERE status IN ('pending', 'delivering');
CREATE INDEX IF NOT EXISTS idx_notification_webhook_finished
    ON notification_webhook_deliveries (finished_at)
    WHERE finished_at IS NOT NULL;

CREATE OR REPLACE FUNCTION enqueue_recharge_completed_webhook()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    config JSONB;
    event_key TEXT;
    paid_currency TEXT;
BEGIN
    -- A damaged optional configuration must not block fulfillment. Keep the
    -- outbox INSERT outside this block so real database failures remain atomic.
    BEGIN
        SELECT value::jsonb INTO config FROM settings WHERE key = 'notification_webhook';
    EXCEPTION WHEN invalid_text_representation THEN
        RETURN NEW;
    END;
    IF config IS NULL OR config->>'enabled' IS DISTINCT FROM 'true'
       OR COALESCE(config->>'version', '') = ''
       OR LENGTH(config->>'version') > 64
       OR COALESCE(config->>'destination', '') = '' THEN
        RETURN NEW;
    END IF;
    event_key := 'payment.recharge.completed:' || NEW.id;
    paid_currency := UPPER(BTRIM(COALESCE(NEW.provider_snapshot->>'currency', '')));
    IF paid_currency !~ '^[A-Z]{3}$' THEN
        paid_currency := 'CNY';
    END IF;
    INSERT INTO notification_webhook_deliveries (event_id, event_type, order_id, config_version, payload)
    VALUES (
        event_key, 'payment.recharge.completed', NEW.id, config->>'version',
        jsonb_build_object(
            'id', event_key,
            'event', 'payment.recharge.completed',
            'created_at', COALESCE(NEW.completed_at, NOW()),
            'data', jsonb_build_object(
                'order_id', NEW.id,
                'out_trade_no', NEW.out_trade_no,
                'user_id', NEW.user_id,
                'user_email', NEW.user_email,
                'user_name', NEW.user_name,
                'order_type', NEW.order_type,
                'status', NEW.status,
                'payment_type', NEW.payment_type,
                'pay_amount', NEW.pay_amount,
                'pay_currency', paid_currency,
                'credited_amount', NEW.amount,
                'bonus_amount', NEW.bonus_amount,
                'balance_currency', 'USD',
                'paid_at', NEW.paid_at,
                'completed_at', NEW.completed_at
            )
        )
    ) ON CONFLICT (event_id) DO NOTHING;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_recharge_completed_webhook ON payment_orders;
CREATE TRIGGER trg_recharge_completed_webhook
AFTER UPDATE OF status ON payment_orders
FOR EACH ROW
WHEN (NEW.order_type = 'balance' AND NEW.status = 'COMPLETED'
      AND OLD.status IS DISTINCT FROM NEW.status AND OLD.completed_at IS NULL
      AND NEW.completed_at IS NOT NULL)
EXECUTE FUNCTION enqueue_recharge_completed_webhook();
