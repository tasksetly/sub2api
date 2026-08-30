package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UpstreamGroup 是从上游同步下来的分组快照，用于横向比价。
//
// 这是只读镜像：字段值来自上游 GET /groups/available 与 /groups/rates，
// 每次同步整体覆盖。不参与本地调度，也不自动写回本地 groups 表。
type UpstreamGroup struct {
	ent.Schema
}

func (UpstreamGroup) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "upstream_groups"},
	}
}

func (UpstreamGroup) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (UpstreamGroup) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("upstream_provider_id"),
		// remote_group_id: 分组在上游的主键，创建 API Key 时要回传给上游
		field.Int64("remote_group_id"),
		field.String("name").
			MaxLen(100).
			NotEmpty(),
		field.String("platform").
			MaxLen(50).
			Default(""),
		field.String("subscription_type").
			MaxLen(20).
			Default(""),

		// ========== 比价核心字段 ==========
		// rate_multiplier: 上游分组基础倍率
		field.Float("rate_multiplier").
			SchemaType(map[string]string{dialect.Postgres: "decimal(10,4)"}).
			Default(1.0),
		// effective_rate_multiplier: 叠加了用户专属倍率/高峰倍率后的实际倍率，
		// 来自 GET /groups/rates；这才是真正该用来比价的值
		field.Float("effective_rate_multiplier").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(10,4)"}),
		field.Bool("peak_rate_enabled").
			Default(false),
		field.Float("peak_rate_multiplier").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(10,4)"}),
		field.String("peak_start").
			MaxLen(5).
			Default(""),
		field.String("peak_end").
			MaxLen(5).
			Default(""),

		// ========== 限额 ==========
		field.Float("daily_limit_usd").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Float("weekly_limit_usd").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Float("monthly_limit_usd").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),

		field.Time("synced_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (UpstreamGroup) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("provider", UpstreamProvider.Type).
			Ref("groups").
			Field("upstream_provider_id").
			Unique().
			Required(),
	}
}

func (UpstreamGroup) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("upstream_provider_id"),
		// 同一上游下 remote_group_id 唯一，同步时按此 upsert
		index.Fields("upstream_provider_id", "remote_group_id").Unique(),
	}
}
