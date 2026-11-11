import adminAccessKeys from './adminAccessKeys'
import mcp from './mcp'
import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import channelMonitorV2 from './channelMonitorV2'
import batchImage from './batchImage'
import playground from './playground'
import admin from './admin'
import misc from './misc'
import tickets from './tickets'
import intelligenceTest from './intelligenceTest'

export default {
  adminAccessKeys,
  mcp,
  ...landing,
  ...common,
  ...dashboard,
  ...channelMonitorV2,
  ...batchImage,
  ...playground,
  admin,
  ...misc,
  ...tickets,
  ...intelligenceTest,
}
