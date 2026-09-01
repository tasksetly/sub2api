package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UpstreamProvider 代表一个上游 sub2api 实例（供应商）。
//
// 与 accounts 的关系：accounts 存的是「拿到手的 API Key」，
// 本表存的是「能登录上游后台的账号」，用于自动拉取分组倍率、
// 余额、并发限制，并直接在上游创建 API Key。
//
// 密码用 AES-256-GCM 可逆加密存储（见 repository.AESEncryptor），
// 因为 token 过期后需要用原密码重新登录，bcrypt 不可行。
type UpstreamProvider struct {
	ent.Schema
}

func (UpstreamProvider) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "upstream_providers"},
	}
}

func (UpstreamProvider) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (UpstreamProvider) Fields() []ent.Field {
	return []ent.Field{
		// name: 上游显示名称，同时写入所建账户的 accounts.supplier 用于成本归集
		field.String("name").
			MaxLen(100).
			NotEmpty(),
		// base_url: 上游站点根地址，如 https://foo.example.com
		field.String("base_url").
			MaxLen(255).
			NotEmpty().
			Comment("Upstream sub2api site root URL, no trailing slash."),
		field.String("notes").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),

		// ========== 登录凭据 ==========
		field.String("username").
			MaxLen(255).
			NotEmpty().
			Comment("Login email/username on the upstream site."),
		// password_encrypted: AES-256-GCM 密文，绝不出现在任何 DTO 响应中
		field.String("password_encrypted").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Comment("AES-256-GCM encrypted login password; reversible because token refresh needs it."),
		// totp_secret_encrypted: 上游开启 2FA 时用于自动生成验证码（可空）
		field.String("totp_secret_encrypted").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Comment("AES-256-GCM encrypted TOTP secret, set only when upstream enforces 2FA."),

		// ========== 会话缓存 ==========
		field.String("token_encrypted").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Comment("AES-256-GCM encrypted upstream JWT, cached to avoid logging in on every sync."),
		field.String("refresh_token_encrypted").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Comment("AES-256-GCM encrypted upstream refresh token used to renew the access JWT."),
		field.Time("token_expires_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).
			Comment("Cached token expiry; refresh happens before this."),

		// ========== 同步来的只读快照 ==========
		// 这些值只用于展示比价，不自动写回本地 accounts。
		field.Float("balance").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Comment("Upstream account balance from GET /user/profile."),
		field.Float("frozen_balance").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Int("upstream_concurrency").
			Optional().
			Nillable().
			Comment("Concurrency limit reported by the upstream user profile."),
		field.String("upstream_user_id").
			MaxLen(64).
			Optional().
			Nillable().
			Comment("User id on the upstream side, for troubleshooting."),

		// rate_correction: 价格倍率修正系数，用于抹平各上游充值比例的差异。
		//
		// 上游声明的倍率是按它自己的站内币计的，充值比例不同就没法直接比：
		// 充值比例 10 倍、倍率 1.0 的上游，真实成本等于 1:1 充值、倍率 0.1 的上游。
		// 填 1/充值比例（10 倍 → 0.1），比价倍率 = 声明倍率 × rate_correction。
		//
		// 默认 1.0 表示 1:1 充值，不做修正。
		field.Float("rate_correction").
			SchemaType(map[string]string{dialect.Postgres: "decimal(10,6)"}).
			Default(1.0).
			Comment("Multiplier that normalizes the upstream's recharge ratio; 1/ratio (10x recharge -> 0.1)."),

		// ========== 同步状态 ==========
		field.String("status").
			MaxLen(20).
			Default(domain.StatusActive),
		field.Time("last_sync_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		// last_sync_error: 同步失败原因（验证码/2FA/密码错误等），空表示上次同步成功
		field.String("last_sync_error").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Bool("sync_enabled").
			Default(true).
			Comment("Whether the background scheduler syncs this provider."),
	}
}

func (UpstreamProvider) Edges() []ent.Edge {
	return []ent.Edge{
		// groups: 从上游同步下来的分组快照
		edge.To("groups", UpstreamGroup.Type),
		// accounts: 由本上游创建出来的本地账户
		edge.To("accounts", Account.Type),
	}
}

func (UpstreamProvider) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status"),
		index.Fields("deleted_at"),
		index.Fields("sync_enabled"),
	}
}
