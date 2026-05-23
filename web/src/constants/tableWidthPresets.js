export const CHANNEL_DETAIL_MODEL_COLUMN_WIDTHS = {
  selection: 64,
  name: 180,
  type: 72,
  alias: 200,
  priceUnit: 108,
  price: 140,
  status: 92,
  upstreamReturn: 140,
  actions: 176,
};

export const PROVIDER_LIST_COLUMN_WIDTHS = {
  id: 240,
  name: 280,
  createdAt: 148,
  updatedAt: 148,
  actions: 72,
};

export const PROVIDER_LIST_TABLE_MIN_WIDTH =
  PROVIDER_LIST_COLUMN_WIDTHS.id +
  PROVIDER_LIST_COLUMN_WIDTHS.name +
  PROVIDER_LIST_COLUMN_WIDTHS.createdAt +
  PROVIDER_LIST_COLUMN_WIDTHS.updatedAt +
  PROVIDER_LIST_COLUMN_WIDTHS.actions;

export const TOPUP_PLAN_LIST_COLUMN_WIDTHS = {
  name: 160,
  group: 140,
  payAmount: 120,
  creditedAmount: 120,
  sortOrder: 100,
  enabled: 100,
  publicVisible: 100,
  validityDays: 100,
  actions: 176,
};

export const TOPUP_PLAN_LIST_TABLE_MIN_WIDTH =
  TOPUP_PLAN_LIST_COLUMN_WIDTHS.name +
  TOPUP_PLAN_LIST_COLUMN_WIDTHS.group +
  TOPUP_PLAN_LIST_COLUMN_WIDTHS.payAmount +
  TOPUP_PLAN_LIST_COLUMN_WIDTHS.creditedAmount +
  TOPUP_PLAN_LIST_COLUMN_WIDTHS.sortOrder +
  TOPUP_PLAN_LIST_COLUMN_WIDTHS.enabled +
  TOPUP_PLAN_LIST_COLUMN_WIDTHS.publicVisible +
  TOPUP_PLAN_LIST_COLUMN_WIDTHS.validityDays +
  TOPUP_PLAN_LIST_COLUMN_WIDTHS.actions;

export const PACKAGE_LIST_COLUMN_WIDTHS = {
  name: 120,
  group: 110,
  salePrice: 80,
  dailyQuota: 120,
  emergencyQuota: 120,
  durationDays: 80,
  status: 92,
  createdAt: 148,
  updatedAt: 148,
  actions: 176,
};

export const PACKAGE_LIST_TABLE_MIN_WIDTH =
  PACKAGE_LIST_COLUMN_WIDTHS.name +
  PACKAGE_LIST_COLUMN_WIDTHS.group +
  PACKAGE_LIST_COLUMN_WIDTHS.salePrice +
  PACKAGE_LIST_COLUMN_WIDTHS.dailyQuota +
  PACKAGE_LIST_COLUMN_WIDTHS.emergencyQuota +
  PACKAGE_LIST_COLUMN_WIDTHS.durationDays +
  PACKAGE_LIST_COLUMN_WIDTHS.status +
  PACKAGE_LIST_COLUMN_WIDTHS.createdAt +
  PACKAGE_LIST_COLUMN_WIDTHS.updatedAt +
  PACKAGE_LIST_COLUMN_WIDTHS.actions;

export const CHANNEL_LIST_COLUMN_WIDTHS = {
  selection: 48,
  name: 180,
  type: 88,
  status: 92,
  createdAt: 148,
  updatedAt: 148,
  capabilities: 160,
  balance: 140,
  priority: 92,
  actions: 192,
};

export const CHANNEL_LIST_TABLE_MIN_WIDTH =
  CHANNEL_LIST_COLUMN_WIDTHS.selection +
  CHANNEL_LIST_COLUMN_WIDTHS.name +
  CHANNEL_LIST_COLUMN_WIDTHS.type +
  CHANNEL_LIST_COLUMN_WIDTHS.status +
  CHANNEL_LIST_COLUMN_WIDTHS.createdAt +
  CHANNEL_LIST_COLUMN_WIDTHS.updatedAt +
  CHANNEL_LIST_COLUMN_WIDTHS.capabilities +
  CHANNEL_LIST_COLUMN_WIDTHS.balance +
  CHANNEL_LIST_COLUMN_WIDTHS.priority +
  CHANNEL_LIST_COLUMN_WIDTHS.actions;

export const GROUP_LIST_COLUMN_WIDTHS = {
  name: 120,
  description: 220,
  channels: 260,
  billingRatio: 96,
  status: 92,
  createdAt: 148,
  updatedAt: 148,
  actions: 192,
};

export const GROUP_LIST_TABLE_MIN_WIDTH =
  GROUP_LIST_COLUMN_WIDTHS.name +
  GROUP_LIST_COLUMN_WIDTHS.description +
  GROUP_LIST_COLUMN_WIDTHS.channels +
  GROUP_LIST_COLUMN_WIDTHS.billingRatio +
  GROUP_LIST_COLUMN_WIDTHS.status +
  GROUP_LIST_COLUMN_WIDTHS.createdAt +
  GROUP_LIST_COLUMN_WIDTHS.updatedAt +
  GROUP_LIST_COLUMN_WIDTHS.actions;

export const TOKEN_LIST_COLUMN_WIDTHS = {
  name: 160,
  status: 92,
  token: 220,
  usedAmount: 120,
  remainingAmount: 120,
  createdTime: 148,
  expiredTime: 148,
  actions: 228,
};

export const TOKEN_LIST_TABLE_MIN_WIDTH =
  TOKEN_LIST_COLUMN_WIDTHS.name +
  TOKEN_LIST_COLUMN_WIDTHS.status +
  TOKEN_LIST_COLUMN_WIDTHS.token +
  TOKEN_LIST_COLUMN_WIDTHS.usedAmount +
  TOKEN_LIST_COLUMN_WIDTHS.remainingAmount +
  TOKEN_LIST_COLUMN_WIDTHS.createdTime +
  TOKEN_LIST_COLUMN_WIDTHS.expiredTime +
  TOKEN_LIST_COLUMN_WIDTHS.actions;

export const USER_LIST_COLUMN_WIDTHS = {
  username: 160,
  wallet: 150,
  package: 140,
  balance: 120,
  requestCount: 100,
  createdAt: 148,
  updatedAt: 148,
  role: 92,
  status: 92,
  actions: 176,
};

export const USER_LIST_TABLE_MIN_WIDTH =
  USER_LIST_COLUMN_WIDTHS.username +
  USER_LIST_COLUMN_WIDTHS.wallet +
  USER_LIST_COLUMN_WIDTHS.package +
  USER_LIST_COLUMN_WIDTHS.balance +
  USER_LIST_COLUMN_WIDTHS.requestCount +
  USER_LIST_COLUMN_WIDTHS.createdAt +
  USER_LIST_COLUMN_WIDTHS.updatedAt +
  USER_LIST_COLUMN_WIDTHS.role +
  USER_LIST_COLUMN_WIDTHS.status +
  USER_LIST_COLUMN_WIDTHS.actions;

export const REDEMPTION_LIST_COLUMN_WIDTHS = {
  name: 160,
  group: 140,
  status: 92,
  faceValue: 140,
  createdTime: 148,
  codeExpiresAt: 148,
  redeemedTime: 148,
  actions: 192,
};

export const REDEMPTION_LIST_TABLE_MIN_WIDTH =
  REDEMPTION_LIST_COLUMN_WIDTHS.name +
  REDEMPTION_LIST_COLUMN_WIDTHS.group +
  REDEMPTION_LIST_COLUMN_WIDTHS.status +
  REDEMPTION_LIST_COLUMN_WIDTHS.faceValue +
  REDEMPTION_LIST_COLUMN_WIDTHS.createdTime +
  REDEMPTION_LIST_COLUMN_WIDTHS.codeExpiresAt +
  REDEMPTION_LIST_COLUMN_WIDTHS.redeemedTime +
  REDEMPTION_LIST_COLUMN_WIDTHS.actions;

export const LOG_LIST_COLUMN_WIDTHS = {
  time: 148,
  channel: 140,
  group: 140,
  type: 88,
  model: 180,
  username: 140,
  tokenName: 160,
  promptTokens: 96,
  completionTokens: 96,
  quota: 140,
};

export const LOG_LIST_TABLE_MIN_WIDTH =
  LOG_LIST_COLUMN_WIDTHS.time +
  LOG_LIST_COLUMN_WIDTHS.channel +
  LOG_LIST_COLUMN_WIDTHS.group +
  LOG_LIST_COLUMN_WIDTHS.type +
  LOG_LIST_COLUMN_WIDTHS.model +
  LOG_LIST_COLUMN_WIDTHS.username +
  LOG_LIST_COLUMN_WIDTHS.tokenName +
  LOG_LIST_COLUMN_WIDTHS.promptTokens +
  LOG_LIST_COLUMN_WIDTHS.completionTokens +
  LOG_LIST_COLUMN_WIDTHS.quota;

export const BUSINESS_FLOW_COLUMN_WIDTHS = {
  user: 140,
  userCompact: 120,
  status: 92,
  type: 88,
  source: 180,
  amount: 140,
  quota: 140,
  datetime: 148,
  group: 140,
  packageName: 180,
  model: 180,
  order: 220,
  message: 180,
  tokenCount: 96,
  actions: 120,
};

export const BUSINESS_FLOW_TABLE_MIN_WIDTH =
  BUSINESS_FLOW_COLUMN_WIDTHS.user +
  BUSINESS_FLOW_COLUMN_WIDTHS.userCompact +
  BUSINESS_FLOW_COLUMN_WIDTHS.status +
  BUSINESS_FLOW_COLUMN_WIDTHS.type +
  BUSINESS_FLOW_COLUMN_WIDTHS.source +
  BUSINESS_FLOW_COLUMN_WIDTHS.amount +
  BUSINESS_FLOW_COLUMN_WIDTHS.quota +
  BUSINESS_FLOW_COLUMN_WIDTHS.datetime +
  BUSINESS_FLOW_COLUMN_WIDTHS.group +
  BUSINESS_FLOW_COLUMN_WIDTHS.packageName +
  BUSINESS_FLOW_COLUMN_WIDTHS.model +
  BUSINESS_FLOW_COLUMN_WIDTHS.order +
  BUSINESS_FLOW_COLUMN_WIDTHS.message +
  BUSINESS_FLOW_COLUMN_WIDTHS.tokenCount +
  BUSINESS_FLOW_COLUMN_WIDTHS.actions;

export const TASK_LIST_COLUMN_WIDTHS = {
  type: 92,
  user: 140,
  channel: 140,
  model: 180,
  status: 92,
  createdAt: 148,
  updatedAt: 148,
  actionsCompact: 88,
  actionsWide: 228,
};

export const TASK_LIST_TABLE_MIN_WIDTH =
  TASK_LIST_COLUMN_WIDTHS.type +
  TASK_LIST_COLUMN_WIDTHS.user +
  TASK_LIST_COLUMN_WIDTHS.channel +
  TASK_LIST_COLUMN_WIDTHS.model +
  TASK_LIST_COLUMN_WIDTHS.status +
  TASK_LIST_COLUMN_WIDTHS.createdAt +
  TASK_LIST_COLUMN_WIDTHS.updatedAt +
  TASK_LIST_COLUMN_WIDTHS.actionsWide;

export const BALANCE_LOT_COLUMN_WIDTHS = {
  source: 180,
  sourceId: 220,
  remaining: 140,
  total: 140,
  status: 92,
  grantedAt: 148,
  expiresAt: 148,
};

export const BALANCE_LOT_TABLE_MIN_WIDTH =
  BALANCE_LOT_COLUMN_WIDTHS.source +
  BALANCE_LOT_COLUMN_WIDTHS.remaining +
  BALANCE_LOT_COLUMN_WIDTHS.total +
  BALANCE_LOT_COLUMN_WIDTHS.status +
  BALANCE_LOT_COLUMN_WIDTHS.grantedAt +
  BALANCE_LOT_COLUMN_WIDTHS.expiresAt;

export const BALANCE_LOT_DETAIL_TABLE_MIN_WIDTH =
  BALANCE_LOT_TABLE_MIN_WIDTH + BALANCE_LOT_COLUMN_WIDTHS.sourceId;

export const TOPUP_RESULT_COLUMN_WIDTHS = {
  label: 160,
  value: 240,
};

export const TOPUP_RESULT_TABLE_MIN_WIDTH =
  TOPUP_RESULT_COLUMN_WIDTHS.label +
  TOPUP_RESULT_COLUMN_WIDTHS.value +
  TOPUP_RESULT_COLUMN_WIDTHS.label +
  TOPUP_RESULT_COLUMN_WIDTHS.value;

export const TOPUP_RECORD_COLUMN_WIDTHS = {
  time: 148,
  businessType: 92,
  status: 92,
  amount: 140,
  quotaOrPackage: 180,
  redemptionCode: 220,
  actions: 228,
};

export const TOPUP_RECORD_TABLE_MIN_WIDTH =
  TOPUP_RECORD_COLUMN_WIDTHS.time +
  TOPUP_RECORD_COLUMN_WIDTHS.businessType +
  TOPUP_RECORD_COLUMN_WIDTHS.status +
  TOPUP_RECORD_COLUMN_WIDTHS.amount +
  TOPUP_RECORD_COLUMN_WIDTHS.quotaOrPackage +
  TOPUP_RECORD_COLUMN_WIDTHS.actions;

export const TOPUP_REDEMPTION_RECORD_TABLE_MIN_WIDTH =
  TOPUP_RECORD_COLUMN_WIDTHS.time +
  TOPUP_RECORD_COLUMN_WIDTHS.amount +
  TOPUP_RECORD_COLUMN_WIDTHS.redemptionCode;
