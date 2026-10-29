import overview from './overview'
import channels from './channels'
import accounts from './accounts'
import resources from './resources'
import ops from './ops'
import settings from './settings'
import audit from './audit'
import promptAudit from './promptAudit'
import plugins from './plugins'

// Keep group form keys available at the final admin merge boundary. This
// prevents a partially cached locale module from dropping fields that are
// referenced by the group management view.
const groups = {
  ...overview.groups,
  form: {
    ...overview.groups.form,
    modelMapping: overview.groups.form.modelMapping || 'Model Mapping',
    modelMappingHint: overview.groups.form.modelMappingHint || 'Map public model IDs to upstream model IDs. JSON object format.',
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
