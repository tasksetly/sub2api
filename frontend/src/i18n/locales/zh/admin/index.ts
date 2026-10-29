import overview from './overview'
import channels from './channels'
import accounts from './accounts'
import resources from './resources'
import ops from './ops'
import settings from './settings'
import audit from './audit'
import promptAudit from './promptAudit'
import plugins from './plugins'

// 在 admin locale 最终合并处保留分组表单字段，避免旧缓存模块覆盖后缺少翻译键。
const groups = {
  ...overview.groups,
  form: {
    ...overview.groups.form,
    modelMapping: overview.groups.form.modelMapping || '模型映射',
    modelMappingHint: overview.groups.form.modelMappingHint || '将公开模型 ID 映射为上游模型 ID，格式为 JSON 对象。',
  },
}

export default {
  ...overview,
  ...channels,
  ...accounts,
  ...resources,
  ...ops,
  ...settings,
  ...audit,
  ...promptAudit,
  ...plugins,
  groups,
}
